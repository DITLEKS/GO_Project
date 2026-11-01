// Package db открывает подключение к PostgreSQL и настраивает пул соединений.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // драйвер "pgx" для database/sql
)

// Параметры пула соединений зафиксированы в коде.
const (
	maxOpenConns    = 10
	maxIdleConns    = 5
	connMaxLifetime = 30 * time.Minute
	pingTimeout     = 5 * time.Second
)

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// DSNFromEnv берёт DSN из DATABASE_URL, а если она не задана — собирает из DB_HOST, DB_PORT,
// DB_USER, DB_PASS, DB_NAME (по умолчанию localhost:5432, postgres/postgres, база cashapp, sslmode=disable).
func DSNFromEnv() string {
	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		return dsn
	}
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(getenv("DB_USER", "postgres"), getenv("DB_PASS", "postgres")),
		Host:     net.JoinHostPort(getenv("DB_HOST", "localhost"), getenv("DB_PORT", "5432")),
		Path:     "/" + getenv("DB_NAME", "cashapp"),
		RawQuery: "sslmode=disable",
	}
	return u.String()
}

// Redact скрывает пароль в DSN для безопасного логирования.
func Redact(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil {
		return "<некорректный DSN>"
	}
	return u.Redacted()
}

// Open выполняет sql.Open("pgx", dsn), настраивает пул и проверяет соединение через Ping.
func Open(ctx context.Context, dsn string) (*sql.DB, error) {
	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("некорректная строка подключения к PostgreSQL (%s): %w", Redact(dsn), err)
	}
	conn.SetMaxOpenConns(maxOpenConns)
	conn.SetMaxIdleConns(maxIdleConns)
	conn.SetConnMaxLifetime(connMaxLifetime)

	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	if err := conn.PingContext(pingCtx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("PostgreSQL недоступен (%s): %w — проверьте, что база запущена, и переменные DATABASE_URL или DB_HOST/DB_PORT/DB_USER/DB_PASS/DB_NAME", Redact(dsn), err)
	}
	return conn, nil
}
