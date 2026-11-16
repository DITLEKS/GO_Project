package service

import (
	"context"
	"errors"
	"testing"

	"github.com/DITLEKS/GO_Project/ledger/internal/domain"
)

// In-memory fakes: the service is tested without PostgreSQL and Redis.

type fakeBudgets struct{ items map[string]domain.Budget }

func (f *fakeBudgets) Upsert(_ context.Context, b domain.Budget) error {
	f.items[b.Category] = b
	return nil
}

func (f *fakeBudgets) GetByCategory(_ context.Context, category string) (domain.Budget, bool, error) {
	b, ok := f.items[category]
	return b, ok, nil
}

func (f *fakeBudgets) List(context.Context) ([]domain.Budget, error) {
	result := make([]domain.Budget, 0, len(f.items))
	for _, b := range f.items {
		result = append(result, b)
	}
	return result, nil
}

type fakeExpenses struct{ items []domain.Transaction }

func (f *fakeExpenses) Add(_ context.Context, t domain.Transaction) (domain.Transaction, error) {
	t.ID = len(f.items) + 1
	f.items = append(f.items, t)
	return t, nil
}

func (f *fakeExpenses) List(context.Context) ([]domain.Transaction, error) { return f.items, nil }

func (f *fakeExpenses) SumByCategory(_ context.Context, category string) (float64, error) {
	var spent float64
	for _, t := range f.items {
		if t.Category == category && t.Amount < 0 {
			spent -= t.Amount
		}
	}
	return spent, nil
}

func (f *fakeExpenses) TotalsByCategory(context.Context, string, string) ([]domain.CategoryTotals, error) {
	return nil, nil
}

type fakeLocker struct{}

func (fakeLocker) WithCategoryLock(ctx context.Context, _ string, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func newTestLedger() (*Ledger, *fakeExpenses) {
	expenses := &fakeExpenses{}
	return New(&fakeBudgets{items: map[string]domain.Budget{}}, expenses, fakeLocker{}, nil, nil), expenses
}

func TestAddTransactionBudgetRule(t *testing.T) {
	t.Parallel()
	svc, expenses := newTestLedger()
	ctx := context.Background()
	if b, err := svc.SetBudget(ctx, domain.Budget{Category: " еда ", Limit: 5000}); err != nil || b.Category != "еда" {
		t.Fatal(b, err)
	}
	for _, tc := range []struct {
		name     string
		amount   float64
		category string
		exceeded bool
	}{
		{"within", -3000, "еда", false}, {"exceeded", -2500, "еда", true},
		{"exact", -2000, "еда", false}, {"cent_over", -0.01, "еда", true},
		{"income", 10000, "еда", false}, {"no_budget", -99999, "такси", false},
	} {
		before := len(expenses.items)
		created, err := svc.AddTransaction(ctx, domain.Transaction{Amount: tc.amount, Category: tc.category})
		if tc.exceeded {
			if !errors.Is(err, domain.ErrBudgetExceeded) || len(expenses.items) != before {
				t.Fatalf("%s: rejected transaction saved: %v", tc.name, err)
			}
			continue
		}
		if err != nil || created.ID != before+1 || created.Date == "" {
			t.Fatalf("%s: %+v %v", tc.name, created, err)
		}
	}
}

func TestValidationErrors(t *testing.T) {
	t.Parallel()
	svc, expenses := newTestLedger()
	ctx := context.Background()
	var invalid *domain.ValidationError
	if _, err := svc.AddTransaction(ctx, domain.Transaction{Category: "еда"}); !errors.As(err, &invalid) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	if _, err := svc.SetBudget(ctx, domain.Budget{Category: "еда", Limit: -1}); !errors.As(err, &invalid) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	if len(expenses.items) != 0 {
		t.Fatal("invalid data saved")
	}
}
