package ledger

import "github.com/DITLEKS/GO_Project/ledger/internal/app"

// DatabaseDSN returns the PostgreSQL DSN that NewService will use:
// DATABASE_URL, or a URL assembled from DB_*. The connection itself is opened in internal/app.
func DatabaseDSN() string {
	return app.DatabaseDSN()
}
