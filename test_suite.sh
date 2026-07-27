#!/usr/bin/env bash
#
# queuectl-test-suite.sh
#
# Thorough local test harness mirroring the assignment's live automated test.
# Run this from your repo root (where go.mod lives) BEFORE your live review.
#
# Requirements: bash, python3 (used for JSON parsing/validation — no jq needed)
#
# Usage:
#   chmod +x queuectl-test-suite.sh
#   ./queuectl-test-suite.sh
#
# Exit code 0 = everything passed. Exit code 1 = at least one failure
# (failed test names are printed at the end).

set -uo pipefail

# ---------- setup ----------

REPO_ROOT="$(pwd)"
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

PASS=0
FAIL=0
FAILED_TESTS=()

pass() { echo -e "  ${GREEN}✓ PASS${NC}: $1"; PASS=$((PASS+1)); }
fail() { echo -e "  ${RED}✗ FAIL${NC}: $1"; FAIL=$((FAIL+1)); FAILED_TESTS+=("$1"); }
info() { echo -e "${YELLOW}→${NC} $1"; }
section() { echo ""; echo -e "${BLUE}══ $1 ══${NC}"; }

echo "Building queuectl..."
if ! go build -o "$REPO_ROOT/queuectl" ./cmd/queuectl; then
  echo -e "${RED}BUILD FAILED — fix compile errors before testing.${NC}"
  exit 1
fi
BINARY="$REPO_ROOT/queuectl"
echo "Build OK: $BINARY"

TEST_DIR="$(mktemp -d)"
LOG_DIR="$TEST_DIR/logs"
mkdir -p "$LOG_DIR"
cd "$TEST_DIR"

kill_all_workers() {
  pkill -9 -f "$BINARY worker start" 2>/dev/null
  sleep 1
}

cleanup() {
  kill_all_workers
  cd "$REPO_ROOT"
  echo ""
  info "Test artifacts + logs kept at: $TEST_DIR"
}
trap cleanup EXIT

fresh_db() {
  kill_all_workers
  rm -f "$TEST_DIR/queuectl.db" "$TEST_DIR/queuectl.db-wal" "$TEST_DIR/queuectl.db-shm"
}

# ---------- helpers ----------

# Prints the state of a job by id, or "not_found". Works purely off documented
# commands (list --state <state> --json), no assumption of a `get` command.
find_job_state() {
  local id="$1"
  for state in pending processing completed failed dead; do
    local out
    out=$("$BINARY" list --state "$state" --json 2>/dev/null)
    local found
    found=$(echo "$out" | python3 -c "
import json, sys
try:
    jobs = json.load(sys.stdin)
except Exception:
    jobs = []
for j in jobs:
    if j.get('id') == '$id':
        print('yes')
        break
" 2>/dev/null)
    if [ "$found" == "yes" ]; then
      echo "$state"
      return 0
    fi
  done
  echo "not_found"
}

wait_for_state() {
  local id="$1" expected="$2" timeout="$3"
  local elapsed=0
  while [ "$elapsed" -lt "$timeout" ]; do
    if [ "$(find_job_state "$id")" == "$expected" ]; then
      return 0
    fi
    sleep 1
    elapsed=$((elapsed+1))
  done
  return 1
}

wait_for_not_state() {
  # waits until job is NO LONGER in $unwanted state (used for "not stuck in processing")
  local id="$1" unwanted="$2" timeout="$3"
  local elapsed=0
  while [ "$elapsed" -lt "$timeout" ]; do
    local s
    s=$(find_job_state "$id")
    if [ "$s" != "$unwanted" ] && [ "$s" != "not_found" ]; then
      echo "$s"
      return 0
    fi
    sleep 1
    elapsed=$((elapsed+1))
  done
  return 1
}

json_field() {
  # json_field '<json array string>' '<id>' '<field>'
  echo "$1" | python3 -c "
import json, sys
jobs = json.load(sys.stdin)
for j in jobs:
    if j.get('id') == '$2':
        print(j.get('$3'))
        break
" 2>/dev/null
}

count_in_state() {
  local state="$1"
  "$BINARY" list --state "$state" --json 2>/dev/null | python3 -c "
import json, sys
try:
    print(len(json.load(sys.stdin)))
except Exception:
    print(0)
"
}

# ---------- interface contract checks ----------

test_json_purity() {
  section "Interface contract: --json output purity"
  fresh_db
  "$BINARY" enqueue '{"id":"purity1","command":"echo hi"}' > /dev/null 2>&1
  local out
  out=$("$BINARY" list --state pending --json 2>/dev/null)
  if echo "$out" | python3 -c "
import json, sys
d = json.load(sys.stdin)
assert isinstance(d, list)
" 2>/dev/null; then
    pass "list --json emits ONLY a clean JSON array on stdout"
  else
    fail "list --json output is not clean JSON (stray text mixed into stdout, or invalid JSON)"
    echo "    Got: $out"
  fi
}

test_worker_start_optional_count() {
  section "Interface contract: --count is optional, defaults sanely"
  fresh_db
  "$BINARY" enqueue '{"id":"defaultcount1","command":"echo hi"}' > /dev/null 2>&1
  "$BINARY" worker start > "$LOG_DIR/default_count.log" 2>&1 &
  local wpid=$!
  if wait_for_state "defaultcount1" "completed" 10; then
    pass "worker start with no --count flag works (defaults to 1)"
  else
    fail "worker start with no --count flag did not process the job"
  fi
  kill -TERM "$wpid" 2>/dev/null
  wait "$wpid" 2>/dev/null
}

# ---------- scenario 1: basic job completes ----------

test_scenario_1() {
  section "Scenario 1: A basic job completes"
  fresh_db
  local id="basic-$$"
  "$BINARY" enqueue "{\"id\":\"$id\",\"command\":\"echo hello\"}" > /dev/null 2>&1

  "$BINARY" worker start --count 1 > "$LOG_DIR/s1.log" 2>&1 &
  local wpid=$!

  if wait_for_state "$id" "completed" 10; then
    pass "Job reaches 'completed' state"
  else
    fail "Job never reached 'completed' (stuck at: $(find_job_state "$id"))"
  fi

  kill -TERM "$wpid" 2>/dev/null
  wait "$wpid" 2>/dev/null
}

# ---------- scenario 2: retry + backoff -> DLQ ----------

test_scenario_2() {
  section "Scenario 2: A failing job retries with backoff and lands in the DLQ"
  fresh_db
  "$BINARY" config set max-retries 3 > /dev/null 2>&1
  "$BINARY" config set backoff-base 2 > /dev/null 2>&1

  local id="fail-$$"
  local start_ts
  start_ts=$(date +%s)
  "$BINARY" enqueue "{\"id\":\"$id\",\"command\":\"exit 1\",\"max_retries\":3}" > /dev/null 2>&1

  "$BINARY" worker start --count 1 > "$LOG_DIR/s2.log" 2>&1 &
  local wpid=$!

  # backoff schedule with base=2, max_retries=3: waits ~2s, ~4s, ~8s between
  # attempts -> total worst case ~14s of waiting + execution overhead
  if wait_for_state "$id" "dead" 30; then
    local elapsed=$(( $(date +%s) - start_ts ))
    pass "Job exhausts retries and lands in 'dead' (DLQ) after ~${elapsed}s"
  else
    fail "Job did not reach 'dead' state within 30s (stuck at: $(find_job_state "$id"))"
  fi

  kill -TERM "$wpid" 2>/dev/null
  wait "$wpid" 2>/dev/null

  local dead_json
  dead_json=$("$BINARY" list --state dead --json 2>/dev/null)
  if [ "$(json_field "$dead_json" "$id" "id")" == "$id" ]; then
    pass "Job appears correctly in 'dead' state list"
  else
    fail "Job missing from dead-state list after exhausting retries"
  fi

  # dlq retry: verify it resets attempts and re-enqueues
  "$BINARY" dlq retry "$id" > /dev/null 2>&1
  local pending_json
  pending_json=$("$BINARY" list --state pending --json 2>/dev/null)
  local attempts_after
  attempts_after=$(json_field "$pending_json" "$id" "attempts")
  if [ "$attempts_after" == "0" ]; then
    pass "dlq retry resets attempts to 0 and returns job to 'pending'"
  else
    fail "dlq retry did not reset attempts correctly (got: '$attempts_after', expected '0')"
  fi
}

# ---------- scenario 3: many jobs, many workers, exactly-once ----------

test_scenario_3() {
  section "Scenario 3: Many jobs across multiple workers — exactly-once execution"
  fresh_db
  local n=30
  local marker_file="$TEST_DIR/exec_log_$$.txt"
  rm -f "$marker_file"
  touch "$marker_file"

  info "Enqueuing $n jobs..."
  for i in $(seq 1 "$n"); do
    "$BINARY" enqueue "{\"id\":\"job-$i\",\"command\":\"echo job-$i >> $marker_file\"}" > /dev/null 2>&1
  done

  info "Starting 2 SEPARATE worker processes (simulating separate terminals), 2 workers each"
  "$BINARY" worker start --count 2 > "$LOG_DIR/s3a.log" 2>&1 &
  local wpid1=$!
  "$BINARY" worker start --count 2 > "$LOG_DIR/s3b.log" 2>&1 &
  local wpid2=$!

  local elapsed=0
  local completed_count=0
  while [ "$elapsed" -lt 30 ]; do
    completed_count=$(count_in_state completed)
    if [ "$completed_count" -eq "$n" ]; then
      break
    fi
    sleep 1
    elapsed=$((elapsed+1))
  done

  if [ "$completed_count" -eq "$n" ]; then
    pass "All $n jobs completed within timeout"
  else
    fail "Only $completed_count/$n jobs completed within 30s"
  fi

  local total_lines unique_lines
  total_lines=$(wc -l < "$marker_file" 2>/dev/null | tr -d ' ')
  unique_lines=$(sort -u "$marker_file" 2>/dev/null | wc -l | tr -d ' ')

  if [ "$total_lines" -eq "$n" ] && [ "$unique_lines" -eq "$n" ]; then
    pass "Exactly-once verified: $total_lines total executions, $unique_lines unique — no duplicates, none missing"
  else
    fail "EXACTLY-ONCE VIOLATED: $total_lines total executions, $unique_lines unique (expected $n/$n — duplicates or lost jobs)"
  fi

  kill -TERM "$wpid1" "$wpid2" 2>/dev/null
  wait "$wpid1" "$wpid2" 2>/dev/null
}

# ---------- scenario 4: SIGKILL mid-job -> recovers within 60s ----------

test_scenario_4() {
  section "Scenario 4: Worker SIGKILLed mid-job — recovers, nothing stuck in processing"
  fresh_db
  local id="crash-$$"
  "$BINARY" enqueue "{\"id\":\"$id\",\"command\":\"sleep 15\"}" > /dev/null 2>&1

  "$BINARY" worker start --count 1 > "$LOG_DIR/s4a.log" 2>&1 &
  local wpid=$!

  if ! wait_for_state "$id" "processing" 10; then
    fail "Job never reached 'processing' before kill — cannot test crash recovery"
    kill -9 "$wpid" 2>/dev/null
    return
  fi

  info "Job is 'processing' under PID $wpid — sending SIGKILL now"
  local kill_time
  kill_time=$(date +%s)
  kill -9 "$wpid" 2>/dev/null
  wait "$wpid" 2>/dev/null

  info "Starting a NEW worker so its reaper can detect and recover the orphaned job"
  "$BINARY" worker start --count 1 > "$LOG_DIR/s4b.log" 2>&1 &
  local wpid2=$!

  # This must resolve to something other than 'processing' within 60s
  local recovered_state
  if recovered_state=$(wait_for_not_state "$id" "processing" 60); then
    local recovery_time=$(( $(date +%s) - kill_time ))
    if [ "$recovery_time" -le 60 ]; then
      pass "Job recovered from stuck 'processing' in ~${recovery_time}s (limit 60s) — new state: $recovered_state"
    else
      fail "Job recovered but took ${recovery_time}s — EXCEEDS the 60s requirement"
    fi
  else
    fail "Job STILL stuck in 'processing' after 60s — crash recovery FAILED"
  fi

  if wait_for_state "$id" "completed" 30; then
    pass "Recovered job eventually completes successfully"
  else
    fail "Recovered job did not reach 'completed' (stuck at: $(find_job_state "$id"))"
  fi

  kill -TERM "$wpid2" 2>/dev/null
  wait "$wpid2" 2>/dev/null
}

# ---------- scenario 5: full restart survival ----------

test_scenario_5() {
  section "Scenario 5: Jobs survive a full restart"
  fresh_db

  # Part A: job enqueued, no worker ever runs, "restart" happens, then a worker starts
  local id1="persist-a-$$"
  "$BINARY" enqueue "{\"id\":\"$id1\",\"command\":\"echo persisted\"}" > /dev/null 2>&1
  if [ "$(find_job_state "$id1")" == "pending" ]; then
    pass "Job persists as 'pending' with no worker running (proves it's on disk, not in memory)"
  else
    fail "Job not found in 'pending' state immediately after enqueue"
  fi

  "$BINARY" worker start --count 1 > "$LOG_DIR/s5a.log" 2>&1 &
  local wpid1=$!
  if wait_for_state "$id1" "completed" 10; then
    pass "Job enqueued before any worker existed still executes correctly"
  else
    fail "Job was lost or never executed"
  fi
  kill -TERM "$wpid1" 2>/dev/null
  wait "$wpid1" 2>/dev/null

  # Part B: harder case — SIGKILL the whole system mid-job, then fully restart
  local id2="persist-b-$$"
  "$BINARY" enqueue "{\"id\":\"$id2\",\"command\":\"sleep 3 && echo done\"}" > /dev/null 2>&1
  "$BINARY" worker start --count 1 > "$LOG_DIR/s5b.log" 2>&1 &
  local wpid2=$!
  sleep 1
  kill -9 "$wpid2" 2>/dev/null
  wait "$wpid2" 2>/dev/null

  info "Simulating full restart with a fresh worker process"
  "$BINARY" worker start --count 1 > "$LOG_DIR/s5c.log" 2>&1 &
  local wpid3=$!
  if wait_for_state "$id2" "completed" 40; then
    pass "Job survives a full crash + restart cycle and completes"
  else
    fail "Job did NOT survive the crash + restart cycle"
  fi
  kill -TERM "$wpid3" 2>/dev/null
  wait "$wpid3" 2>/dev/null
}

# ---------- worker stop (cross-process signaling) ----------

test_worker_stop() {
  section "worker stop from a separate process (simulating a different terminal)"
  fresh_db
  "$BINARY" enqueue '{"id":"stoptest1","command":"sleep 20"}' > /dev/null 2>&1

  "$BINARY" worker start --count 1 > "$LOG_DIR/stop_test.log" 2>&1 &
  local wpid=$!
  sleep 2

  if ! kill -0 "$wpid" 2>/dev/null; then
    fail "Worker process died unexpectedly before the stop test could run"
    return
  fi

  info "Invoking 'worker stop' as an independent process"
  "$BINARY" worker stop > /dev/null 2>&1

  local exited=false
  local elapsed=0
  while [ "$elapsed" -lt 15 ]; do
    if ! kill -0 "$wpid" 2>/dev/null; then
      exited=true
      break
    fi
    sleep 1
    elapsed=$((elapsed+1))
  done

  if [ "$exited" == "true" ]; then
    pass "worker stop successfully terminated the running worker via cross-process signal"
  else
    fail "worker stop did NOT terminate the running worker within 15s"
    kill -9 "$wpid" 2>/dev/null
  fi
}

test_graceful_shutdown() {
  section "Graceful shutdown: SIGTERM mid-job finishes the in-flight job before exiting"
  fresh_db
  local id="graceful-$$"
  "$BINARY" enqueue "{\"id\":\"$id\",\"command\":\"sleep 4 && echo done\"}" > /dev/null 2>&1

  "$BINARY" worker start --count 1 > "$LOG_DIR/graceful.log" 2>&1 &
  local wpid=$!

  if ! wait_for_state "$id" "processing" 10; then
    fail "Job never reached 'processing' — cannot test graceful shutdown"
    kill -9 "$wpid" 2>/dev/null
    return
  fi

  info "Sending SIGTERM to worker mid-job"
  kill -TERM "$wpid"

  if wait_for_state "$id" "completed" 15; then
    pass "In-flight job completed despite SIGTERM — graceful shutdown honored"
  else
    fail "In-flight job did NOT complete after SIGTERM — graceful shutdown is broken (job likely abandoned)"
  fi

  wait "$wpid" 2>/dev/null
}

# ---------- config snapshot semantics ----------

test_config_snapshot() {
  section "Config semantics: changes don't retroactively affect already-enqueued jobs"
  fresh_db
  "$BINARY" config set max-retries 5 > /dev/null 2>&1
  local id_a="cfg-a-$$"
  "$BINARY" enqueue "{\"id\":\"$id_a\",\"command\":\"exit 1\"}" > /dev/null 2>&1

  "$BINARY" config set max-retries 1 > /dev/null 2>&1
  local id_b="cfg-b-$$"
  "$BINARY" enqueue "{\"id\":\"$id_b\",\"command\":\"exit 1\"}" > /dev/null 2>&1

  local pending_json
  pending_json=$("$BINARY" list --state pending --json 2>/dev/null)
  local retries_a retries_b
  retries_a=$(json_field "$pending_json" "$id_a" "max_retries")
  retries_b=$(json_field "$pending_json" "$id_b" "max_retries")

  if [ "$retries_a" == "5" ] && [ "$retries_b" == "1" ]; then
    pass "Each job kept its own snapshotted max_retries (A=5, B=1) despite config changing between enqueues"
  else
    fail "Config snapshot semantics broken (job A max_retries='$retries_a' expected '5', job B='$retries_b' expected '1')"
  fi
}

# ---------- run everything ----------

SUITE_START=$(date +%s)

test_json_purity
test_worker_start_optional_count
test_scenario_1
test_scenario_2
test_scenario_3
test_scenario_4
test_scenario_5
test_worker_stop
test_graceful_shutdown
test_config_snapshot

SUITE_END=$(date +%s)
TOTAL_TIME=$((SUITE_END - SUITE_START))

echo ""
echo "=============================================="
echo -e "  RESULTS: ${GREEN}$PASS passed${NC}, ${RED}$FAIL failed${NC}  (${TOTAL_TIME}s total)"
echo "=============================================="

if [ "$FAIL" -gt 0 ]; then
  echo -e "${RED}Failed tests:${NC}"
  for t in "${FAILED_TESTS[@]}"; do
    echo "  - $t"
  done
  echo ""
  echo "Logs for each scenario are in: $LOG_DIR"
  exit 1
fi

echo -e "${GREEN}All tests passed. You're in good shape for the live review.${NC}"
exit 0