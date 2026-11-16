package pg

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/DITLEKS/GO_Project/ledger/internal/domain"
)

// ExpenseRepository implements domain.ExpenseRepository.
// Every query uses the Context variant, so cancelling ctx interrupts the SQL operation.
type ExpenseRepository struct {
	db *sql.DB
}

var _ domain.ExpenseRepository = (*ExpenseRepository)(nil)

func NewExpenseRepository(db *sql.DB) *ExpenseRepository { return &ExpenseRepository{db: db} }

func (r *ExpenseRepository) Add(ctx context.Context, t domain.Transaction) (domain.Transaction, error) {
	err := conn(ctx, r.db).QueryRowContext(ctx, `INSERT INTO expenses(amount,category,description,date)
        VALUES($1,$2,$3,$4) RETURNING id`,
		fmt.Sprintf("%.2f", t.Amount), t.Category, t.Description, t.Date).Scan(&t.ID)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("insert expense: %w", err)
	}
	return t, nil
}

func (r *ExpenseRepository) List(ctx context.Context) ([]domain.Transaction, error) {
	rows, err := conn(ctx, r.db).QueryContext(ctx, `SELECT id,amount,category,COALESCE(description,''),date
        FROM expenses ORDER BY date DESC,id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list expenses: %w", err)
	}
	defer rows.Close()
	result := make([]domain.Transaction, 0)
	for rows.Next() {
		var t domain.Transaction
		var date time.Time
		if err := rows.Scan(&t.ID, &t.Amount, &t.Category, &t.Description, &t.Date); err != nil {
			return nil, fmt.Errorf("scan expense: %w", err)
		}
		t.Date = date.Format(domain.DateLayout)
		result = append(result, t)
	}
	return result, rows.Err()
}

// SumByCategory counts only expenses: income never cancels spent money.
func (r *ExpenseRepository) SumByCategory(ctx context.Context, category string) (float64, error) {
	var spent float64
	err := conn(ctx, r.db).QueryRowContext(ctx,
		"SELECT COALESCE(SUM(-amount),0) FROM expenses WHERE category=$1 AND amount<0", category).Scan(&spent)
	if err != nil {
		return 0, fmt.Errorf("sum expenses: %w", err)
	}
	return spent, nil
}

func (r *ExpenseRepository) CategoriesInPeriod(ctx context.Context, from, to string) ([]string, error) {
	rows, err := conn(ctx, r.db).QueryContext(ctx, `SELECT DISTINCT category FROM expenses
        WHERE date >= $1::date AND date <= $2::date ORDER BY category`, from, to)
	if err != nil {
		return nil, fmt.Errorf("list report categories: %w", err)
	}
	defer rows.Close()
	result := make([]string, 0)
	for rows.Next() {
		var category string
		if err := rows.Scan(&category); err != nil {
			return nil, fmt.Errorf("scan report category: %w", err)
		}
		result = append(result, category)
	}
	return result, rows.Err()
}

func (r *ExpenseRepository) TotalsForCategory(ctx context.Context, category, from, to string) (domain.CategoryTotals, error) {
	row := domain.CategoryTotals{Category: category}
	err := conn(ctx, r.db).QueryRowContext(ctx, `SELECT COALESCE(SUM(amount),0),
        COALESCE(SUM(CASE WHEN amount>0 THEN amount ELSE 0 END),0),
        COALESCE(SUM(CASE WHEN amount<0 THEN -amount ELSE 0 END),0)
        FROM expenses WHERE category=$1 AND date >= $2::date AND date <= $3::date`,
		category, from, to).Scan(&row.Total, &row.Income, &row.Expenses)
	if err != nil {
		return domain.CategoryTotals{}, fmt.Errorf("report aggregation for %q: %w", category, err)
	}
	return row, nil
}
