// Package app is the composition root of Ledger: it reads the configuration,
// opens the connections, creates the repositories and assembles the service.
// Migrations are NOT run here; apply them with goose before the start.
package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/DITLEKS/GO_Project/ledger/internal/cache"
	"github.com/DITLEKS/GO_Project/ledger/internal/repository/pg"
	"github.com/DITLEKS/GO_Project/ledger/internal/service"
)

const startTimeout = 5 * time.Second

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// DatabaseDSN prefers DATABASE_URL, then assembles an escaped URL from DB_*.
func DatabaseDSN() string {
	if value := os.Getenv("DATABASE_URL"); value != "" {
		return value
	}
	u := url.URL{
		Scheme: "postgres",
		Host:   net.JoinHostPort(envOr("DB_HOST", "localhost"), envOr("DB_PORT", "5432")),
		User:   url.UserPassword(envOr("DB_USER", "postgres"), envOr("DB_PASS", "postgres")),
		Path:   "/" + envOr("DB_NAME", "cashapp"),
	}
	q := u.Query()
	q.Set("sslmode", "disable")
	u.RawQuery = q.Encode()
	return u.String()
}

// New returns a ready service, a function that releases its resources, and an error.
// ctx is passed to the startup Ping of the database.
func New(ctx context.Context) (service.Service, func() error, error) {
	svc, closeFn, err := Build(ctx)
	if err != nil {
		return nil, nil, err
	}
	return svc, closeFn, nil
}

// Build is New that returns the concrete type; Ledger's own tests need its extra methods.
func Build(ctx context.Context) (*service.Ledger, func() error, error) {
	db, err := sql.Open("pgx", DatabaseDSN())
	if err != nil {
		return nil, nil, fmt.Errorf("open PostgreSQL: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	startCtx, cancel := context.WithTimeout(ctx, startTimeout)
	defer cancel()
	if err := db.PingContext(startCtx); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("connect PostgreSQL (check DATABASE_URL/DB_*): %w", err)
	}
	// Read-only schema check, not a migration.
	if _, err := db.ExecContext(startCtx, "SELECT category, limit_amount, period FROM budgets LIMIT 0; SELECT id, amount, date FROM expenses LIMIT 0"); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("check PostgreSQL schema; run goose up: %w", err)
	}
	reportCache, err := cache.Open(startCtx)
	if err != nil {
		_ = db.Close()
		return nil, nil, err
	}
	log.Print("Ledger: connected to PostgreSQL (pool open=10 idle=5)")
	log.Print("Ledger: connected to Redis (report TTL=30s)")

	store := pg.NewStore(db)
	svc := service.New(pg.NewBudgetRepository(db), pg.NewExpenseRepository(db), store, store, reportCache)
	// closeFn runs only after active requests stop. It does not delete data.
	closeFn := func() error {
		return errors.Join(reportCache.Close(), db.Close())
	}
	return svc, closeFn, nil
}
