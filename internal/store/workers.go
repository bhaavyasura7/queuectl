package store

import (
	"database/sql"
	"os"
	"time"
)

func (s *Store) RegisterWorker() error {
	now := time.Now().UTC().Format(time.RFC3339)
	hostname, _ := os.Hostname()
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO workers (pid, hostname, started_at, status)
		VALUES (?, ?, ?, 'running')
	`, os.Getpid(), hostname, now)
	return err
}

func (s *Store) DeregisterWorker() error {
	_, err := s.db.Exec("DELETE FROM workers WHERE pid = ?", os.Getpid())
	return err
}

func (s *Store) GetRunningWorkers() (*sql.Rows, error) {
	return s.db.Query("SELECT pid, hostname, started_at, status FROM workers WHERE status = 'running'")
}

func (s *Store) LiveWorkerCount() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM workers WHERE status = 'running'").Scan(&count)
	return count, err
}

func (s *Store) DeregisterWorkerByPID(pid int) error {
	_, err := s.db.Exec("DELETE FROM workers WHERE pid = ?", pid)
	return err
}
