package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/DITLEKS/GO_Project/ledger/internal/domain"
)

// The fakes below are safe for concurrent use: the import calls them from several workers.

type syncExpenses struct {
	mu    sync.Mutex
	items []domain.Transaction
	// delay makes Add slow; it still stops early when ctx is cancelled.
	delay time.Duration
}

func (f *syncExpenses) Add(ctx context.Context, t domain.Transaction) (domain.Transaction, error) {
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return domain.Transaction{}, ctx.Err()
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	t.ID = len(f.items) + 1
	f.items = append(f.items, t)
	return t, nil
}

func (f *syncExpenses) List(context.Context) ([]domain.Transaction, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]domain.Transaction(nil), f.items...), nil
}

func (f *syncExpenses) SumByCategory(_ context.Context, category string) (float64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var spent float64
	for _, t := range f.items {
		if t.Category == category && t.Amount < 0 {
			spent -= t.Amount
		}
	}
	return spent, nil
}

func (f *syncExpenses) CategoriesInPeriod(context.Context, string, string) ([]string, error) {
	return nil, nil
}

func (f *syncExpenses) TotalsForCategory(_ context.Context, category, _, _ string) (domain.CategoryTotals, error) {
	return domain.CategoryTotals{Category: category}, nil
}

func (f *syncExpenses) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.items)
}

// mutexLocker makes the budget check and the insert atomic, like the SQL transaction does.
type mutexLocker struct{ mu sync.Mutex }

func (l *mutexLocker) WithCategoryLock(ctx context.Context, _ string, fn func(ctx context.Context) error) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return fn(ctx)
}

func newImportLedger() (*Ledger, *syncExpenses) {
	expenses := &syncExpenses{}
	return New(&fakeBudgets{items: map[string]domain.Budget{}}, expenses, &mutexLocker{}, nil, nil), expenses
}

func TestImportTransactions(t *testing.T) {
	t.Parallel()
	svc, expenses := newImportLedger()
	ctx := context.Background()
	if _, err := svc.SetBudget(ctx, domain.Budget{Category: "еда", Limit: 100}); err != nil {
		t.Fatal(err)
	}
	// 40 expenses of 10 with a limit of 100: exactly 10 fit, 30 are rejected.
	txs := make([]domain.Transaction, 0, 44)
	for i := 0; i < 40; i++ {
		txs = append(txs, domain.Transaction{Amount: -10, Category: "еда", Date: "2026-11-01"})
	}
	txs = append(txs,
		domain.Transaction{Amount: 500, Category: "зарплата", Date: "2026-11-01"}, // 40: accepted
		domain.Transaction{Amount: 0, Category: "еда"},                           // 41: invalid amount
		domain.Transaction{Amount: -5},                                           // 42: empty category
		domain.Transaction{Amount: -99999, Category: "такси", Date: "2026-11-02"}, // 43: no budget, accepted
	)
	result, err := svc.ImportTransactions(ctx, txs, 4)
	if err != nil {
		t.Fatal(err)
	}
	if result.Accepted != 12 || result.Rejected != 32 || len(result.Errors) != 32 {
		t.Fatalf("accepted %d, rejected %d, errors %d", result.Accepted, result.Rejected, len(result.Errors))
	}
	if expenses.count() != 12 {
		t.Fatalf("saved %d transactions, expected 12", expenses.count())
	}
	if spent, _ := expenses.SumByCategory(ctx, "еда"); spent != 100 {
		t.Fatalf("budget exceeded or not used: spent %v", spent)
	}
	budgetErrors := 0
	messages := map[int]string{}
	for i, e := range result.Errors {
		if i > 0 && result.Errors[i-1].Index >= e.Index {
			t.Fatalf("errors are not sorted by index: %+v", result.Errors)
		}
		if e.Index < 0 || e.Index >= len(txs) || e.Error == "" {
			t.Fatalf("incorrect error entry: %+v", e)
		}
		messages[e.Index] = e.Error
		if e.Error == "budget exceeded" {
			budgetErrors++
		}
	}
	if budgetErrors != 30 {
		t.Fatalf("expected 30 budget errors, got %d", budgetErrors)
	}
	if messages[41] == "" || messages[42] == "" || messages[41] == "budget exceeded" || messages[42] == "budget exceeded" {
		t.Fatalf("validation errors are missing: %q, %q", messages[41], messages[42])
	}
	if _, rejected := messages[40]; rejected {
		t.Fatal("income was rejected")
	}
	if _, rejected := messages[43]; rejected {
		t.Fatal("category without budget was rejected")
	}
}

func TestImportWorkersAndEmptyInput(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, workers := range []int{-1, 0, 1, 3, MaxImportWorkers, 1000} {
		svc, expenses := newImportLedger()
		txs := make([]domain.Transaction, 20)
		for i := range txs {
			txs[i] = domain.Transaction{Amount: 1, Category: "x", Date: "2026-11-01"}
		}
		result, err := svc.ImportTransactions(ctx, txs, workers)
		if err != nil || result.Accepted != 20 || result.Rejected != 0 || len(result.Errors) != 0 || expenses.count() != 20 {
			t.Fatalf("workers=%d: %+v %v", workers, result, err)
		}
	}
	svc, _ := newImportLedger()
	result, err := svc.ImportTransactions(ctx, nil, 4)
	if err != nil || result.Accepted != 0 || result.Rejected != 0 || result.Errors == nil || len(result.Errors) != 0 {
		t.Fatalf("empty input: %+v %v", result, err)
	}
}

func TestImportCancelled(t *testing.T) {
	t.Parallel()
	svc, expenses := newImportLedger()
	expenses.delay = 20 * time.Millisecond
	txs := make([]domain.Transaction, 200)
	for i := range txs {
		txs[i] = domain.Transaction{Amount: 1, Category: "x", Date: "2026-11-01"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 70*time.Millisecond)
	defer cancel()
	start := time.Now()
	result, err := svc.ImportTransactions(ctx, txs, 4)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
	// The function returns only after the workers and the collector have exited.
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("import was not interrupted: took %s", elapsed)
	}
	// Partial result: fewer elements than the input, and it matches what was saved.
	if result.Accepted+result.Rejected >= len(txs) || result.Accepted != expenses.count() {
		t.Fatalf("incorrect partial result: %+v, saved %d", result, expenses.count())
	}
}
