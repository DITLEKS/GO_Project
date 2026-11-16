package ledger

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/DITLEKS/GO_Project/ledger/internal/app"
	"github.com/DITLEKS/GO_Project/ledger/internal/cache"
	"github.com/DITLEKS/GO_Project/ledger/internal/service"
)

var (
	testSvc   *service.Ledger
	testClose func() error
	testCache *cache.Client
)

// Destructive integration tests opt in through a separate test DSN.
func TestMain(m *testing.M) {
	if dsn := os.Getenv("TEST_DATABASE_URL"); dsn != "" {
		os.Setenv("DATABASE_URL", dsn)
		ctx := context.Background()
		c, err := cache.Open(ctx)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		testCache = c
		if err := Initialize(ctx); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		code := m.Run()
		_ = Close()
		_ = c.Close()
		os.Exit(code)
	}
	os.Exit(m.Run())
}

// The adapters below keep the bodies of the earlier tests unchanged:
// they map the old package-level functions to the service built by the factory.

func Initialize(ctx context.Context) error {
	svc, closeFn, err := app.Build(ctx)
	if err != nil {
		return err
	}
	testSvc, testClose = svc, closeFn
	return nil
}

func Close() error {
	if testClose == nil {
		return nil
	}
	err := testClose()
	testSvc, testClose = nil, nil
	return err
}

func bg() context.Context { return context.Background() }

func cacheConn() *cache.Client { return testCache }

func Reset() error { return testSvc.Reset(bg()) }

func SetBudget(b Budget) error {
	_, err := testSvc.SetBudget(bg(), b)
	return err
}

func GetBudget(category string) (Budget, bool, error) { return testSvc.GetBudget(bg(), category) }

func ListBudgets() ([]Budget, error) { return testSvc.ListBudgets(bg()) }

func SpentByCategory(category string) (float64, error) {
	return testSvc.SpentByCategory(bg(), category)
}

func CreateTransaction(tx Transaction) (Transaction, error) {
	return testSvc.AddTransaction(bg(), tx)
}

func AddTransaction(tx Transaction) error {
	_, err := CreateTransaction(tx)
	return err
}

func ListTransactions() ([]Transaction, error) { return testSvc.ListTransactions(bg()) }

func GetReportSummary(ctx context.Context, from, to time.Time) (ReportSummary, error) {
	return testSvc.ReportSummary(ctx, from, to)
}

func loadBudgets(r io.Reader) error { return LoadBudgets(bg(), testSvc, r) }

func resetChecked(t *testing.T) {
	t.Helper()
	if os.Getenv("TEST_DATABASE_URL") == "" {
		t.Skip("PostgreSQL integration: set TEST_DATABASE_URL and apply migrations; Redis required")
	}
	if err := Reset(); err != nil {
		t.Fatal(err)
	}
}

func listTransactions(t *testing.T) []Transaction {
	t.Helper()
	v, err := ListTransactions()
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func listBudgets(t *testing.T) []Budget {
	t.Helper()
	v, err := ListBudgets()
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func getBudget(t *testing.T, category string) (Budget, bool) {
	t.Helper()
	b, ok, err := GetBudget(category)
	if err != nil {
		t.Fatal(err)
	}
	return b, ok
}

func spentByCategory(t *testing.T, category string) float64 {
	t.Helper()
	v, err := SpentByCategory(category)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
