// Package tasks is the service layer for task tracking. It owns every database
// access, the single name/seq -> uuid resolution point, and the single
// implementation of task duration math, so that cmd/ contains only wiring and
// rendering.
package tasks

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"barbtils/internal/database"

	"github.com/lib/pq"
)

// Store is the only thing in the codebase that opens a database handle for tasks.
type Store struct {
	db *sql.DB
	q  *database.Queries
}

func Open(dbURL string) (*Store, error) {
	if dbURL == "" {
		return nil, errors.New("DB_URL is not configured — set it in ~/.config/barbtils/config.toml or the environment")
	}
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("reaching database: %w", err)
	}
	return &Store{db: db, q: database.New(db)}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// Queries exposes the generated layer for read paths that need no transaction.
func (s *Store) Queries() *database.Queries { return s.q }

// withTx runs fn inside a transaction, so that a session change and the task
// status change it implies either both land or neither does.
func (s *Store) withTx(ctx context.Context, fn func(*database.Queries) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op once Commit succeeds
	if err := fn(s.q.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit()
}

// secs converts the ::bigint second counts the queries return into a Duration.
func secs(n int64) time.Duration { return time.Duration(n) * time.Second }

// constraintViolation reports whether err is a unique/check violation on the
// named constraint, so SQLSTATE codes never reach the user.
func constraintViolation(err error, name string) bool {
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) {
		return false
	}
	return pqErr.Constraint == name
}
