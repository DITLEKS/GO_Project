// Package pg implements the domain repository contracts on PostgreSQL.
// It must not import HTTP or Gateway code.
package pg

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/DITLEKS/GO_Project/ledger/internal/domain"
)

// querier is satisfied by both *sql.DB and *sql.Tx.
type querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type txKey struct{}

// conn returns the transaction stored in ctx by WithCategoryLock, or the pool.
func conn(ctx context.Context, db *sql.DB) querier {
	if tx, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
		return tx
	}
	return db
}

// Store owns SQL transactions and maintenance operations.
type Store struct {
	db *sql.DB
}

var (
	_ domain.CategoryLocker = (*Store)(nil)
	_ domain.Purger         = (*Store)(nil)
)

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// WithCategoryLock runs fn in one SQL transaction under an advisory lock of the category.
// The lock is shared by budget upserts and expense inserts, including absent budget rows.
func (s *Store) WithCategoryLock(ctx context.Context, category string, fn func(ctx context.Context) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", category); err != nil {
		return fmt.Errorf("lock category: %w", err)
	}
	if err := fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// DeleteAll deletes ALL ledger data and resets IDs.
func (s *Store) DeleteAll(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, "TRUNCATE expenses, budgets RESTART IDENTITY"); err != nil {
		return fmt.Errorf("truncate ledger tables: %w", err)
	}
	return nil
}
