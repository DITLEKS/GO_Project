package service

import (
	"context"
	"errors"
	"sort"
	"sync/atomic"
	"testing"
	"time"

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

type fakeExpenses struct {
	items []domain.Transaction
	// delay makes TotalsForCategory slow; it still stops early when ctx is cancelled.
	delay time.Duration
	// calls counts TotalsForCategory calls made by the report goroutines.
	calls atomic.Int32
}

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

func (f *fakeExpenses) CategoriesInPeriod(_ context.Context, from, to string) ([]string, error) {
	seen := map[string]bool{}
	result := make([]string, 0)
	for _, t := range f.items {
		if t.Date >= from && t.Date <= to && !seen[t.Category] {
			seen[t.Category] = true
			result = append(result, t.Category)
		}
	}
	sort.Strings(result)
	return result, nil
}

func (f *fakeExpenses) TotalsForCategory(ctx context.Context, category, from, to string) (domain.CategoryTotals, error) {
	f.calls.Add(1)
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return domain.CategoryTotals{}, ctx.Err()
		}
	}
	row := domain.CategoryTotals{Category: category}
	for _, t := range f.items {
		if t.Category != category || t.Date < from || t.Date > to {
			continue
		}
		row.Total += t.Amount
		if t.Amount > 0 {
			row.Income += t.Amount
		} else {
			row.Expenses -= t.Amount
		}
	}
	return row, nil
}

type fakeLocker struct{}

func (fakeLocker) WithCategoryLock(ctx context.Context, _ string, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func newTestLedger() (*Ledger, *fakeExpenses) {
	expenses := &fakeExpenses{}
	return New(&fakeBudgets{items: map[string]domain.Budget{}}, expenses, fakeLocker{}, nil, nil), expenses
}

func day(t *testing.T, value string) time.Time {
	t.Helper()
	d, err := time.Parse(domain.DateLayout, value)
	if err != nil {
		t.Fatal(err)
	}
	return d
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

func TestReportSummaryConcurrent(t *testing.T) {
	t.Parallel()
	svc, expenses := newTestLedger()
	ctx := context.Background()
	for _, tx := range []domain.Transaction{
		{Amount: 1000, Category: "зарплата", Date: "2026-10-01"},
		{Amount: -25.50, Category: "еда", Date: "2026-10-31"},
		{Amount: -100, Category: "еда", Date: "2026-10-10"},
		{Amount: -40, Category: "транспорт", Date: "2026-10-15"},
		{Amount: -999, Category: "вне периода", Date: "2026-11-01"},
	} {
		if _, err := svc.AddTransaction(ctx, tx); err != nil {
			t.Fatal(err)
		}
	}
	report, err := svc.ReportSummary(ctx, day(t, "2026-10-01"), day(t, "2026-10-31"))
	if err != nil {
		t.Fatal(err)
	}
	if report.Income != 1000 || report.Expenses != 165.50 || report.Balance != 834.50 {
		t.Fatalf("incorrect totals: %+v", report)
	}
	want := []domain.CategorySummary{{Category: "еда", Total: -125.50}, {Category: "зарплата", Total: 1000}, {Category: "транспорт", Total: -40}}
	if len(report.Categories) != len(want) {
		t.Fatalf("incorrect categories: %+v", report.Categories)
	}
	for i := range want {
		if report.Categories[i] != want[i] {
			t.Fatalf("category %d: got %+v, want %+v", i, report.Categories[i], want[i])
		}
	}
	// One calculation per category of the period.
	if got := expenses.calls.Load(); got != 3 {
		t.Fatalf("expected 3 category calculations, got %d", got)
	}

	empty, err := svc.ReportSummary(ctx, day(t, "2020-01-01"), day(t, "2020-01-01"))
	if err != nil || empty.Categories == nil || len(empty.Categories) != 0 {
		t.Fatal(empty, err)
	}
}

func TestReportSummaryCancelled(t *testing.T) {
	t.Parallel()
	svc, expenses := newTestLedger()
	for _, category := range []string{"a", "b", "c"} {
		if _, err := svc.AddTransaction(context.Background(), domain.Transaction{Amount: -1, Category: category, Date: "2026-10-01"}); err != nil {
			t.Fatal(err)
		}
	}
	// Each category would take 5 seconds; the context allows 50 ms.
	expenses.delay = 5 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := svc.ReportSummary(ctx, day(t, "2026-10-01"), day(t, "2026-10-31"))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
	// ReportSummary returns only after all workers and the heartbeat have exited.
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("calculation was not interrupted: took %s", elapsed)
	}
}
