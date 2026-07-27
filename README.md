# QueueCTL

SQLite-backed job queue CLI. Third-gen design. Embedded SQLite — no separate broker,
message queue, or daemon process. Workers, producers, and admin commands all use the
same `queuectl` binary and share a single SQLite file for state.

## Quick Start

```bash
go build -o queuectl ./cmd/queuectl
```

Requirements: Go 1.22+.

## Usage

### Enqueue

```bash
queuectl enqueue '{"command":"echo hello"}'
queuectl enqueue '{"command":"daily-report","run_at":"2026-07-27T08:00:00Z"}'
queuectl enqueue '{"command":"urgent","priority":"high"}'
queuectl enqueue '{"command":"slow","timeout":30}'
queuectl enqueue '{"command":"fragile","max_retries":5,"backoff_base":3}'
```

### Worker

```bash
# Start a worker (foreground, blocks, use separate terminal)
queuectl worker start --count 2

# Stop all workers from another terminal
queuectl worker stop

# Ctrl+C or SIGTERM → finish current job → clean exit
# SIGKILL → reaper recovers in another worker
```

### Status and Listing

```bash
queuectl status
queuectl list
queuectl list --state pending
queuectl list --state completed --json
```

### Dead Letter Queue

```bash
queuectl dlq list
queuectl dlq retry <job-id>
```

### Job Output and Statistics

```bash
queuectl output <job-id>
queuectl stats
```

### Configuration

```bash
queuectl config set max-retries 5
queuectl config set backoff-base 3
```

### Web Dashboard

```bash
queuectl web
# Open http://localhost:9090
```

Default DB path: `./queuectl.db` in the current working directory. Override with `--db`.

## Crash Recovery

Every live `worker start` process runs an independent reaper goroutine (every 5s)
that scans for jobs stuck in `processing` with stale heartbeats (>15s). Crashed jobs
are transitioned to `failed` (or `dead` if retries exhausted) and picked up by the
next claim loop. Worst-case recovery time: ~22 seconds. See DECISIONS.md for the full
walkthrough.

## Testing

All 5 assignment scenarios verified locally:

| # | Scenario | Command |
|---|---|---|
| 1 | Basic job completes | `enqueue '{"command":"echo hello"}' → worker start` |
| 2 | Failing → retry → DLQ | `enqueue '{"command":"exit 1","max_retries":3}' → worker start` |
| 3 | Exactly-once, N workers | 30 jobs + `--count 3` → 0 duplicates |
| 4 | SIGKILL → recover | `kill -9 <worker-pid>` mid-job → new worker recovers |
| 5 | Full restart survives | Restart worker → picks up existing jobs |

```bash
# SIGKILL test
queuectl enqueue '{"command":"sleep 30"}' --db test.db
queuectl worker start --count 1 --db test.db &  # terminal 1
kill -9 $(pgrep -f "worker start")
queuectl worker start --count 1 --db test.db    # terminal 2, reaper recovers
```

## Architecture

```
queuectl.db ── SQLite (WAL mode, single file, OS file lock)
    │
    ├── enqueue        ── INSERT job
    ├── worker start   ── claim loop + heartbeat + reaper goroutines
    ├── worker stop    ── syscall.Kill(pid, SIGTERM) → drain → exit
    ├── status         ── COUNT BY state + live workers
    ├── list           ── SELECT BY state (--json for machine output)
    ├── dlq            ── list dead + retry (dead→pending, attempts=0)
    ├── output         ── show stdout/stderr for a job
    ├── stats          ── throughput/success rate
    ├── config         ── read/write max-retries, backoff-base
    └── web            ── HTTP dashboard on :9090
```

See DECISIONS.md for design rationale, atomic claim mechanism, and rejected alternatives.
