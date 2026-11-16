package domain

import "context"

// The contracts below describe what the business logic needs,
// not how the data is stored.

// BudgetRepository stores budgets.
type BudgetRepository interface {
	// Upsert adds a budget or updates the existing one for the same category.
	Upsert(ctx context.Context, b Budget) error
	// GetByCategory returns false when the category has no budget.
	GetByCategory(ctx context.Context, category string) (Budget, bool, error)
	List(ctx context.Context) ([]Budget, error)
}

// ExpenseRepository stores transactions (expenses and income).
type ExpenseRepository interface {
	// Add saves the transaction and returns it with the assigned ID.
	Add(ctx context.Context, t Transaction) (Transaction, error)
	List(ctx context.Context) ([]Transaction, error)
	// SumByCategory returns the positive amount spent in the category; income is ignored.
	SumByCategory(ctx context.Context, category string) (float64, error)
	// CategoriesInPeriod lists, in alphabetical order, the categories that have
	// transactions in the inclusive interval [from, to] (YYYY-MM-DD).
	CategoriesInPeriod(ctx context.Context, from, to string) ([]string, error)
	// TotalsForCategory aggregates one category over the inclusive interval [from, to].
	TotalsForCategory(ctx context.Context, category, from, to string) (CategoryTotals, error)
}

// CategoryLocker runs fn atomically while holding an exclusive lock on the category.
// Repository calls made with the ctx passed to fn belong to the same unit of work.
type CategoryLocker interface {
	WithCategoryLock(ctx context.Context, category string, fn func(ctx context.Context) error) error
}

// Purger deletes all ledger data. Used by tests only.
type Purger interface {
	DeleteAll(ctx context.Context) error
}

// ReportCache is the optional, fail-open cache of report summaries.
type ReportCache interface {
	Version(ctx context.Context) (string, error)
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, data []byte) error
	Invalidate(ctx context.Context) error
}
