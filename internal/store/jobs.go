package store

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"
)

var ErrNoJob = errors.New("no claimable job")

func (s *Store) InsertJob(id, command, now string, priority int, runAt *string, timeout *int, maxRetries int, backoffBase float64) error {
	_, err := s.db.Exec(`
		INSERT INTO jobs (id, command, state, priority, run_at, timeout, attempts, max_retries, backoff_base, created_at, updated_at)
		VALUES (?, ?, 'pending', ?, ?, ?, 0, ?, ?, ?, ?)
	`, id, command, priority, runAt, timeout, maxRetries, backoffBase, now, now)
	return err
}

func (s *Store) ClaimNext(workerID string) (string, string, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	tx, err := s.db.Begin()
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("BEGIN IMMEDIATE"); err != nil {
		return "", "", err
	}

	var id, command string
	err = tx.QueryRow(`
		SELECT id, command FROM jobs
		WHERE state = 'pending' AND (run_at IS NULL OR run_at <= ?)
		   OR (state = 'failed' AND next_attempt_at <= ?)
		ORDER BY priority DESC, created_at ASC
		LIMIT 1
	`, now, now).Scan(&id, &command)
	if err == sql.ErrNoRows {
		return "", "", ErrNoJob
	}
	if err != nil {
		return "", "", err
	}

	res, err := tx.Exec(`
		UPDATE jobs
		SET state = 'processing', worker_id = ?, claimed_at = ?, heartbeat_at = ?, updated_at = ?
		WHERE id = ? AND state IN ('pending', 'failed')
	`, workerID, now, now, now, id)
	if err != nil {
		return "", "", err
	}

	n, _ := res.RowsAffected()
	if n != 1 {
		return "", "", ErrNoJob
	}

	if err := tx.Commit(); err != nil {
		return "", "", err
	}

	return id, command, nil
}

func (s *Store) CompleteJob(id string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`
		UPDATE jobs SET state = 'completed', updated_at = ?, last_error = NULL WHERE id = ?
	`, now, id)
	return err
}

func (s *Store) FailJob(id string, attempts int, maxRetries int, backoffBase float64, errMsg, stdout, stderr string) error {
	now := time.Now().UTC().Format(time.RFC3339)

	var state string
	var nextAttemptAt *string

	if attempts+1 < maxRetries {
		state = "failed"
		delay := time.Duration(math.Pow(backoffBase, float64(attempts+1)) * float64(time.Second))
		nat := time.Now().UTC().Add(delay).Format(time.RFC3339)
		nextAttemptAt = &nat
	} else {
		state = "dead"
	}

	_, err := s.db.Exec(`
		UPDATE jobs
		SET state = ?,
		    attempts = attempts + 1,
		    last_error = ?,
		    stdout = ?,
		    stderr = ?,
		    next_attempt_at = ?,
		    updated_at = ?
		WHERE id = ?
	`, state, errMsg, stdout, stderr, nextAttemptAt, now, id)
	return err
}

func (s *Store) GetJob(id string) (*sql.Row, error) {
	row := s.db.QueryRow("SELECT * FROM jobs WHERE id = ?", id)
	return row, nil
}

func (s *Store) ListJobsByState(state string) (*sql.Rows, error) {
	return s.db.Query("SELECT * FROM jobs WHERE state = ? ORDER BY created_at ASC", state)
}

func (s *Store) CountJobsByState() (*sql.Rows, error) {
	return s.db.Query("SELECT state, COUNT(*) as count FROM jobs GROUP BY state")
}

func (s *Store) TouchHeartbeat(id string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec("UPDATE jobs SET heartbeat_at = ? WHERE id = ?", now, id)
	return err
}

func (s *Store) ReaperScan(leaseTimeout time.Duration) error {
	staleBefore := time.Now().UTC().Add(-leaseTimeout).Format(time.RFC3339)

	rows, err := s.db.Query(`
		SELECT id, attempts, max_retries, backoff_base FROM jobs
		WHERE state = 'processing' AND heartbeat_at < ?
	`, staleBefore)
	if err != nil {
		return err
	}
	defer rows.Close()

	var updates []struct {
		id          string
		state       string
		nextAttempt *string
	}

	now := time.Now().UTC().Format(time.RFC3339)

	for rows.Next() {
		var id string
		var attempts, maxRetries int
		var backoffBase float64
		if err := rows.Scan(&id, &attempts, &maxRetries, &backoffBase); err != nil {
			return err
		}

		newAttempts := attempts + 1
		var newState string
		var nextAttempt *string

		if newAttempts < maxRetries {
			newState = "failed"
			delay := time.Duration(math.Pow(backoffBase, float64(newAttempts)) * float64(time.Second))
			nat := time.Now().UTC().Add(delay).Format(time.RFC3339)
			nextAttempt = &nat
		} else {
			newState = "dead"
		}

		updates = append(updates, struct {
			id          string
			state       string
			nextAttempt *string
		}{id, newState, nextAttempt})
	}

	for _, u := range updates {
		_, err := s.db.Exec(`
			UPDATE jobs
			SET state = ?, attempts = attempts + 1, next_attempt_at = ?, updated_at = ?
			WHERE id = ?
		`, u.state, u.nextAttempt, now, u.id)
		if err != nil {
			return fmt.Errorf("reaper update %s: %w", u.id, err)
		}
	}

	return nil
}

func (s *Store) RetryDlqJob(id string) (bool, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(`
		UPDATE jobs
		SET state = 'pending', attempts = 0, next_attempt_at = NULL, updated_at = ?
		WHERE id = ? AND state = 'dead'
	`, now, id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}
