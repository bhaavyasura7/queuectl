# QueueCTL

SQLite-backed job queue CLI. Embedded database, single binary, zero dependencies.

## Setup

```bash
go build -o queuectl ./cmd/queuectl
```

Requirements: Go 1.22+.

## Usage

```
queuectl enqueue '<json>'          Enqueue a new job
queuectl worker start --count N    Start a worker (foreground)
queuectl worker stop               Stop all running workers
queuectl status                    Show queue status
queuectl list --state <s> [--json] List jobs
queuectl dlq list                  List dead-lettered jobs
queuectl dlq retry <id>            Retry a dead job
queuectl config set <key> <val>    Set configuration
```

DB file: `./queuectl.db` in the current working directory.
