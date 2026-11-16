package pg

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/DITLEKS/GO_Project/ledger/internal/domain"
)

// ExpenseRepository implements domain.ExpenseRepository.
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
		if err := rows.Scan(&t.ID, &t.Amount, &t.Category, &t.Description, &date); err != nil {
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

// TotalsByCategory uses one SQL snapshot for category values and totals.
func (r *ExpenseRepository) TotalsByCategory(ctx context.Context, from, to string) ([]domain.CategoryTotals, error) {
	rows, err := conn(ctx, r.db).QueryContext(ctx, `SELECT category,SUM(amount),
        SUM(CASE WHEN amount>0 THEN amount ELSE 0 END),
        SUM(CASE WHEN amount<0 THEN -amount ELSE 0 END)
        FROM expenses WHERE date >= $1::date AND date <= $2::date GROUP BY category ORDER BY category`, from, to)
	if err != nil {
		return nil, fmt.Errorf("report aggregation: %w", err)
	}
	defer rows.Close()
	result := make([]domain.CategoryTotals, 0)
	for rows.Next() {
		var row domain.CategoryTotals
		if err := rows.Scan(&row.Category, &row.Total, &row.Income, &row.Expenses); err != nil {
			return nil, fmt.Errorf("scan report row: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}
