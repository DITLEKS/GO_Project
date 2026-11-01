package ledger

import (
	"context"
	"database/sql"

	"github.com/DITLEKS/GO_Project/ledger/internal/cache"
)

// storage — источник данных: PostgreSQL (боевой режим) или память (тесты и демо).
// В теме 8 это будет вынесено в слой репозиториев / DI.
type storage interface {
	setBudget(ctx context.Context, b Budget) error
	getBudget(ctx context.Context, category string) (Budget, bool, error)
	listBudgets(ctx context.Context) ([]Budget, error)
	spentByCategory(ctx context.Context, category string) (float64, error)
	addTransaction(ctx context.Context, tx Transaction) (Transaction, error)
	listTransactions(ctx context.Context) ([]Transaction, error)
	summary(ctx context.Context, from, to string) ([]CategorySummary, error)
}

var (
	store       storage = newMemoryStore(true)
	dbConn      *sql.DB
	cacheClient *cache.Client
)
