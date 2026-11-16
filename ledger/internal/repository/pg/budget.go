package pg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/DITLEKS/GO_Project/ledger/internal/domain"
)

// BudgetRepository implements domain.BudgetRepository.
type BudgetRepository struct {
	db *sql.DB
}

var _ domain.BudgetRepository = (*BudgetRepository)(nil)

func NewBudgetRepository(db *sql.DB) *BudgetRepository { return &BudgetRepository{db: db} }

func (r *BudgetRepository) Upsert(ctx context.Context, b domain.Budget) error {
	_, err := conn(ctx, r.db).ExecContext(ctx, `INSERT INTO budgets(category,limit_amount,period) VALUES($1,$2,$3)
        ON CONFLICT(category) DO UPDATE SET limit_amount=EXCLUDED.limit_amount,period=EXCLUDED.period`,
		b.Category, fmt.Sprintf("%.2f", b.Limit), b.Period)
	if err != nil {
		return fmt.Errorf("upsert budget: %w", err)
	}
	return nil
}

func (r *BudgetRepository) GetByCategory(ctx context.Context, category string) (domain.Budget, bool, error) {
	var b domain.Budget
	err := conn(ctx, r.db).QueryRowContext(ctx,
		"SELECT category,limit_amount,period FROM budgets WHERE category=$1", category).
		Scan(&b.Category, &b.Limit, &b.Period)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Budget{}, false, nil
	}
	if err != nil {
		return domain.Budget{}, false, fmt.Errorf("get budget: %w", err)
	}
	return b, true, nil
}

func (r *BudgetRepository) List(ctx context.Context) ([]domain.Budget, error) {
	rows, err := conn(ctx, r.db).QueryContext(ctx, "SELECT category,limit_amount,period FROM budgets ORDER BY category")
	if err != nil {
		return nil, fmt.Errorf("list budgets: %w", err)
	}
	defer rows.Close()
	result := make([]domain.Budget, 0)
	for rows.Next() {
		var b domain.Budget
		if err := rows.Scan(&b.Category, &b.Limit, &b.Period); err != nil {
			return nil, fmt.Errorf("scan budget: %w", err)
		}
		result = append(result, b)
	}
	return result, rows.Err()
}
