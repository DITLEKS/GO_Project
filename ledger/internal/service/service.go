// Package service contains the business logic of Ledger.
// It talks to storage only through the domain contracts.
package service

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/DITLEKS/GO_Project/ledger/internal/domain"
)

const opTimeout = 5 * time.Second

// Service is the application interface used by Gateway.
type Service interface {
	// SetBudget validates and saves the budget; returns the normalized value.
	SetBudget(ctx context.Context, b domain.Budget) (domain.Budget, error)
	ListBudgets(ctx context.Context) ([]domain.Budget, error)
	// AddTransaction validates, applies the budget rule and saves; returns the record with its ID.
	AddTransaction(ctx context.Context, t domain.Transaction) (domain.Transaction, error)
	// ImportTransactions imports a batch concurrently with the given number of workers
	// (workers <= 0 means the default). On cancellation it returns the partial summary and the context error.
	ImportTransactions(ctx context.Context, txs []domain.Transaction, workers int) (domain.ImportResult, error)
	ListTransactions(ctx context.Context) ([]domain.Transaction, error)
	ReportSummary(ctx context.Context, from, to time.Time) (domain.ReportSummary, error)
}

// Ledger implements Service on top of the domain contracts.
type Ledger struct {
	budgets  domain.BudgetRepository
	expenses domain.ExpenseRepository
	locker   domain.CategoryLocker
	purger   domain.Purger
	cache    domain.ReportCache
}

var _ Service = (*Ledger)(nil)

// New wires the service; cache may be nil (reports are then always computed).
func New(budgets domain.BudgetRepository, expenses domain.ExpenseRepository, locker domain.CategoryLocker, purger domain.Purger, cache domain.ReportCache) *Ledger {
	return &Ledger{budgets: budgets, expenses: expenses, locker: locker, purger: purger, cache: cache}
}

func (s *Ledger) SetBudget(ctx context.Context, b domain.Budget) (domain.Budget, error) {
	b.Category = strings.TrimSpace(b.Category)
	if err := b.Validate(); err != nil {
		return domain.Budget{}, &domain.ValidationError{Err: err}
	}
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()
	err := s.locker.WithCategoryLock(ctx, b.Category, func(ctx context.Context) error {
		return s.budgets.Upsert(ctx, b)
	})
	if err != nil {
		return domain.Budget{}, err
	}
	return b, nil
}

func (s *Ledger) GetBudget(ctx context.Context, category string) (domain.Budget, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()
	return s.budgets.GetByCategory(ctx, strings.TrimSpace(category))
}

func (s *Ledger) ListBudgets(ctx context.Context) ([]domain.Budget, error) {
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()
	return s.budgets.List(ctx)
}

func (s *Ledger) SpentByCategory(ctx context.Context, category string) (float64, error) {
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()
	return s.expenses.SumByCategory(ctx, category)
}

// AddTransaction: validation, then the budget rule, then the insert.
// The rule and the insert run under one category lock, so parallel requests cannot exceed the limit.
func (s *Ledger) AddTransaction(ctx context.Context, t domain.Transaction) (domain.Transaction, error) {
	if err := t.Validate(); err != nil {
		return domain.Transaction{}, &domain.ValidationError{Err: err}
	}
	t.Category = strings.TrimSpace(t.Category)
	t.Amount = math.Round(t.Amount*100) / 100
	if t.Date == "" {
		t.Date = time.Now().UTC().Format(domain.DateLayout)
	}
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()
	var created domain.Transaction
	err := s.locker.WithCategoryLock(ctx, t.Category, func(ctx context.Context) error {
		if t.Amount < 0 {
			budget, ok, err := s.budgets.GetByCategory(ctx, t.Category)
			if err != nil {
				return fmt.Errorf("read budget: %w", err)
			}
			if ok {
				spent, err := s.expenses.SumByCategory(ctx, t.Category)
				if err != nil {
					return fmt.Errorf("check budget: %w", err)
				}
				// Spent + new expense must not exceed the limit; compared in whole cents.
				if cents(spent)+cents(-t.Amount) > cents(budget.Limit) {
					return fmt.Errorf("%w: category %q", domain.ErrBudgetExceeded, t.Category)
				}
			}
		}
		saved, err := s.expenses.Add(ctx, t)
		if err != nil {
			return err
		}
		created = saved
		return nil
	})
	if err != nil {
		return domain.Transaction{}, err
	}
	s.invalidateReports(ctx)
	return created, nil
}

func (s *Ledger) ListTransactions(ctx context.Context) ([]domain.Transaction, error) {
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()
	return s.expenses.List(ctx)
}

func cents(v float64) int64 { return int64(math.Round(v * 100)) }

func roundMoney(v float64) float64 { return math.Round(v*100) / 100 }
