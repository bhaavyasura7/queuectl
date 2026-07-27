package store

import (
	"database/sql"
	"sync"

	_ "modernc.org/sqlite"
)

type Store struct {
	db    *sql.DB
	mu    sync.Mutex
}

func Open(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	//t prevents database/sql from opening multiple idle connections to the same SQLite file, since only one can write at a time anyway.
	db.SetMaxOpenConns(1)

	if err := runMigration(db); err != nil {
		db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) DB() *sql.DB {
	return s.db
}
