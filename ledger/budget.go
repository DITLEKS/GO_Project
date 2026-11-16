package ledger

import (
	"context"
	"io"

	"github.com/DITLEKS/GO_Project/ledger/internal/domain"
	"github.com/DITLEKS/GO_Project/ledger/internal/service"
)

// Budget is a cumulative spending limit for a category.
type Budget = domain.Budget

// ErrBudgetExceeded is returned by Service.AddTransaction when the limit would be exceeded.
var ErrBudgetExceeded = domain.ErrBudgetExceeded

// LoadBudgets reads a JSON array of budgets from r and saves each one through svc.
func LoadBudgets(ctx context.Context, svc Service, r io.Reader) error {
	return service.LoadBudgets(ctx, svc, r)
}
