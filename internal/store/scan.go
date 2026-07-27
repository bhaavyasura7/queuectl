package store

import (
	"database/sql"
	"queuectl/internal/types"
)

func ScanRow(row *sql.Row) (types.Job, error) {
	var j types.Job
	err := row.Scan(
		&j.ID, &j.Command, &j.State, &j.Priority, &j.RunAt, &j.Timeout,
		&j.Attempts, &j.MaxRetries, &j.BackoffBase,
		&j.CreatedAt, &j.UpdatedAt,
		&j.NextAttemptAt, &j.ClaimedAt, &j.HeartbeatAt,
		&j.WorkerID, &j.LastError, &j.Stdout, &j.Stderr,
	)
	return j, err
}
