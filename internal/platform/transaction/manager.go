package transaction

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type Executor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type contextKey struct{}
type state struct {
	db *sql.DB
	tx *sql.Tx
}

type Manager struct{ db *sql.DB }

func New(db *sql.DB) *Manager { return &Manager{db: db} }

func (m *Manager) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	if _, ok := ctx.Value(contextKey{}).(*state); ok {
		return errors.New("nested transactions are not supported")
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()
	if err := fn(context.WithValue(ctx, contextKey{}, &state{db: m.db, tx: tx})); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func Connection(ctx context.Context, db *sql.DB) Executor {
	if current, ok := ctx.Value(contextKey{}).(*state); ok && current.db == db {
		return current.tx
	}
	return db
}
