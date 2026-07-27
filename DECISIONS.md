# DECISIONS.md

## Q1: Which exact line(s) guarantee exactly-once claiming?

**`internal/store/jobs.go:21-72` — `ClaimNext()`**

The atomic claim is three things working together:

1. **`db.Exec("BEGIN IMMEDIATE")` (line ~30)**: Acquires an OS-level exclusive file lock on
   `queuectl.db` before any SELECT runs. This is a kernel-level `flock()`/`LockFileEx()`
   call — not an in-process mutex. A second process's own `ClaimNext()` physically
   blocks here until this transaction commits or rolls back.

2. **`UPDATE ... WHERE id = ? AND state IN ('pending','failed')` (line ~58)**: Re-checks the
   state inside the transaction. Even if `BEGIN IMMEDIATE` somehow failed (it won't),
   this WHERE clause won't match a row that was already claimed between the SELECT and
   UPDATE.

3. **`RowsAffected() != 1` check (line ~64)**: If the UPDATE matched 0 rows, someone else
   claimed it — treat as `ErrNoJob`. This is the concrete proof line.

The `sync.Mutex` on `Store` (in `db.go`) serializes in-process goroutine access so that
two concurrent claim loops in the same `--count N` process don't interleave `BEGIN
IMMEDIATE` → SELECT → UPDATE → COMMIT on the single database connection. This is the
in-process guard. The OS file lock from `BEGIN IMMEDIATE` is the cross-process guard.
Both are necessary; together they give exactly-once semantics across any number of
goroutines and OS processes sharing the same SQLite file.

---

## Q2: Worst-case SIGKILL recovery walkthrough

```
t=0.00s   Worker heartbeats → heartbeat_at = 14:30:00
t=0.01s   Worker receives SIGKILL (kill -9). Heartbeat goroutine dies.
t=5.00s   Reaper tick 1 → staleBefore = 14:29:50. heartbeat_at=14:30:00 > 14:29:50 → skip
t=10.00s  Reaper tick 2 → staleBefore = 14:29:55. 14:30:00 > 14:29:55 → skip
t=15.00s  Reaper tick 3 → staleBefore = 14:30:00. 14:30:00 NOT < 14:30:00 → skip
t=20.00s  Reaper tick 4 → staleBefore = 14:30:05. 14:30:00 < 14:30:05 → STALE → recover
t=22.00s  Claim loop picks up recovered job with backoff=2^1=2s
```

**Worst case: ~22 seconds** (15s lease + 5s tick alignment penalty + 2s backoff).
38 seconds of margin under the 60s requirement. The `<` (strict less-than) in
`WHERE heartbeat_at < staleBefore` is why tick 3 at exactly t=15s misses — the
timestamp equals the cutoff, so the job isn't quite stale yet.

**Lease timeout**: 15s = 3 heartbeat intervals × 5s. One slow tick can't trigger
a false positive. Two slow ticks still can't reach 15s. Three missed ticks = safely
dead.

---

## Q3: Does `dlq retry` reset attempts?

**Yes, `attempts` is reset to 0.** `dlq retry` is a deliberate manual action — a human
decided the underlying issue is fixed and the job deserves a fresh retry budget. Not
resetting would mean a job with `max_retries=3` dies again immediately after 0 additional
failures, defeating the point.

`last_error` is preserved (not cleared) for observability — it's evidence of what killed
the job last time, unrelated to the retry budget.

Implementation: `internal/store/jobs.go:RetryDlqJob()` — `UPDATE jobs SET state='pending',
attempts=0, next_attempt_at=NULL WHERE id=? AND state='dead'`.

---

## Q4: Rejected alternative designs for `worker stop`

**Rejected: Unix domain socket per worker.**
Needs a socket path/naming scheme and a listener goroutine — all to deliver a signal the
OS already delivers for free via `syscall.Kill()`. More surface area to defend live for
no behavioral gain.

**Rejected: DB-polled "shutdown_requested" flag.**
Adds up to one full poll interval of latency before a worker notices the flag. The
assignment already mandates `SIGTERM`/`SIGINT` handling for Ctrl+C, so reusing that
identical signal path for `worker stop` avoids maintaining two shutdown mechanisms that
must behave identically.

**Chosen: PID registry in SQLite `workers` table + `syscall.Kill(pid, SIGTERM)`.**
Single mechanism for Ctrl+C and `worker stop`. No extra infrastructure. If a worker was
`SIGKILL`ed (stale row), `worker stop` detects `ESRCH` from `syscall.Kill` and cleans up
the orphaned row directly.

Implementation: `internal/cli/worker_cmd.go:stop` + `internal/worker/signals.go`.

---

## Q5: How would priority scheduling change the design?

The claim query's `ORDER BY` changes from `created_at ASC` to `priority DESC, created_at
ASC`. The atomic `BEGIN IMMEDIATE` + `UPDATE...WHERE` claim mechanism, backoff math, DLQ,
and worker-stop signaling are all **unchanged** — none of them depend on FIFO ordering.

What changes:
- Schema: new `priority TEXT` column with CHECK constraint
- Index: updated (or the CASE expression in ORDER BY handles it at query time)
- Enqueue command: accepts optional `"priority"` field in JSON

Ordering is decoupled from atomicity. The claim guarantee doesn't care what order jobs
are claimed in — it only cares that each job is claimed exactly once.

---

## Architecture decisions not covered above

**why `modernc.org/sqlite` (pure Go, no CGO)?**
`go build` produces a single static binary with zero native-compilation dependencies.
The reviewer can build it on any machine without a C toolchain. During a live
screen-share, `go build` just works — unlike CGO-based drivers which need gcc/clang.

**why synchronous?**
Go's goroutines + `database/sql` connection pool handle concurrency. SQLite's file
lock handles cross-process serialization. No async/await needed — Go's runtime
schedules goroutines onto OS threads automatically.

**why WAL + busy_timeout?**
WAL mode lets readers (`status`, `list`, `dlq list`) run without blocking on writers
(claim loops). `busy_timeout=5000` makes a second writer wait gracefully instead of
throwing `SQLITE_BUSY` immediately — essential when two `worker start` processes both
try `BEGIN IMMEDIATE` at the same moment.

**why `SetMaxOpenConns(1)`?**
SQLite supports only one writer at a time. Multiple open connections add no parallelism
for writes. A single connection with a Go mutex ensures predictable serialization of
in-process goroutines, making the concurrency model easier to reason about and cite in
review.
