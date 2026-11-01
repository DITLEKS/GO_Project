package ledger

import (
 "context"
 "database/sql"
 "errors"
 "fmt"
 "log"
 "net"
 "net/url"
 "os"
 "sync"
 "time"

 "github.com/DITLEKS/GO_Project/ledger/internal/cache"
 _ "github.com/jackc/pgx/v5/stdlib"
)

var (
 storageMu sync.RWMutex
 database *sql.DB
 reportCache *cache.Client
 ErrNotInitialized = errors.New("ledger: PostgreSQL is not initialized; call ledger.Initialize first")
)
func envOr(key, fallback string) string {
 if v := os.Getenv(key); v != "" { return v }; return fallback
}
// DatabaseDSN prefers DATABASE_URL, then assembles an escaped URL from DB_*.
func DatabaseDSN() string {
 if value := os.Getenv("DATABASE_URL"); value != "" { return value }
 u := url.URL{Scheme: "postgres", Host: net.JoinHostPort(envOr("DB_HOST", "localhost"), envOr("DB_PORT", "5432")), User: url.UserPassword(envOr("DB_USER", "postgres"), envOr("DB_PASS", "postgres")), Path: "/" + envOr("DB_NAME", "cashapp")}
 q := u.Query(); q.Set("sslmode", "disable"); u.RawQuery = q.Encode()
 return u.String()
}
// Initialize runs once before requests, after goose up; no volatile fallback.
func Initialize(ctx context.Context) error {
 storageMu.Lock(); defer storageMu.Unlock()
 if database != nil { return errors.New("ledger already initialized") }
 db, err := sql.Open("pgx", DatabaseDSN()); if err != nil { return fmt.Errorf("open PostgreSQL: %w", err) }
 db.SetMaxOpenConns(10); db.SetMaxIdleConns(5); db.SetConnMaxLifetime(30*time.Minute)
 pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second); defer cancel()
 if err := db.PingContext(pingCtx); err != nil { _ = db.Close(); return fmt.Errorf("connect PostgreSQL (check DATABASE_URL/DB_*): %w", err) }
 if _, err := db.ExecContext(pingCtx, "SELECT category, limit_amount, period FROM budgets LIMIT 0; SELECT id, amount, date FROM expenses LIMIT 0"); err != nil { _ = db.Close(); return fmt.Errorf("check PostgreSQL schema; run goose up: %w", err) }
 c, err := cache.Open(pingCtx); if err != nil { _ = db.Close(); return err }
 database, reportCache = db, c
 log.Print("Ledger: connected to PostgreSQL (pool open=10 idle=5)")
 log.Print("Ledger: connected to Redis (report TTL=30s)")
 return nil
}
// Close runs only after active requests stop. It does not delete data.
func Close() error {
 storageMu.Lock(); defer storageMu.Unlock()
 var err error
 if reportCache != nil { err = reportCache.Close(); reportCache = nil }
 if database != nil { err = errors.Join(err, database.Close()); database = nil }
 return err
}
func dbConn() (*sql.DB, error) {
 storageMu.RLock(); defer storageMu.RUnlock()
 if database == nil { return nil, ErrNotInitialized }; return database, nil
}
func cacheConn() *cache.Client { storageMu.RLock(); defer storageMu.RUnlock(); return reportCache }
func operationContext() (context.Context, context.CancelFunc) { return context.WithTimeout(context.Background(), 5*time.Second) }
func invalidateReports(ctx context.Context) {
 if c := cacheConn(); c != nil { if err := c.Invalidate(ctx); err != nil { log.Printf("report cache invalidation failed (TTL limits staleness): %v", err) } }
}
