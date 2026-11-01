package ledger

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"
)

// pgStore — хранилище на PostgreSQL. Денежные колонки — NUMERIC(14,2); в Go читаем и пишем через float8.
type pgStore struct {
	db *sql.DB
}

// setBudget делает UPSERT по уникальной категории.
func (s *pgStore) setBudget(ctx context.Context, b Budget) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO budgets(category, limit_amount, period) VALUES($1, $2::float8, $3)
ON CONFLICT (category) DO UPDATE SET limit_amount = EXCLUDED.limit_amount, period = EXCLUDED.period`,
		b.Category, b.Limit, b.Period)
	return err
}

func (s *pgStore) getBudget(ctx context.Context, category string) (Budget, bool, error) {
	var b Budget
	err := s.db.QueryRowContext(ctx,
		`SELECT category, limit_amount::float8, period FROM budgets WHERE category = $1`, category,
	).Scan(&b.Category, &b.Limit, &b.Period)
	if errors.Is(err, sql.ErrNoRows) {
		return Budget{}, false, nil
	}
	if err != nil {
		return Budget{}, false, err
	}
	return b, true, nil
}

func (s *pgStore) listBudgets(ctx context.Context) ([]Budget, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT category, limit_amount::float8, period FROM budgets ORDER BY category`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]Budget, 0)
	for rows.Next() {
		var b Budget
		if err := rows.Scan(&b.Category, &b.Limit, &b.Period); err != nil {
			return nil, err
		}
		result = append(result, b)
	}
	return result, rows.Err()
}

// spentByCategory — сумма расходов (отрицательных сумм) как положительное число.
func (s *pgStore) spentByCategory(ctx context.Context, category string) (float64, error) {
	var spent float64
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(-amount), 0)::float8 FROM expenses WHERE category = $1 AND amount < 0`, category,
	).Scan(&spent)
	return spent, err
}

// addTransaction проверяет лимит бюджета и вставляет запись в одной транзакции БД.
// Строка бюджета блокируется (FOR UPDATE), чтобы параллельные запросы по категории не пробили лимит.
// Правило то же, что и в памяти: расход — отрицательная сумма, доходы и категории без бюджета не ограничиваются.
func (s *pgStore) addTransaction(ctx context.Context, tx Transaction) (Transaction, error) {
	date := tx.Date
	if date == "" {
		date = time.Now().Format(dateLayout)
	}

	sqlTx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Transaction{}, fmt.Errorf("начало транзакции БД: %w", err)
	}
	defer func() { _ = sqlTx.Rollback() }()

	if tx.Amount < 0 {
		var limit float64
		err := sqlTx.QueryRowContext(ctx,
			`SELECT limit_amount::float8 FROM budgets WHERE category = $1 FOR UPDATE`, tx.Category,
		).Scan(&limit)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			// бюджета нет — категория не ограничена
		case err != nil:
			return Transaction{}, fmt.Errorf("чтение бюджета: %w", err)
		default:
			var spent float64
			if err := sqlTx.QueryRowContext(ctx,
				`SELECT COALESCE(SUM(-amount), 0)::float8 FROM expenses WHERE category = $1 AND amount < 0`, tx.Category,
			).Scan(&spent); err != nil {
				return Transaction{}, fmt.Errorf("подсчёт расходов: %w", err)
			}
			amount := math.Abs(tx.Amount)
			if spent+amount > limit {
				return Transaction{}, budgetExceeded(tx.Category, limit, spent, amount)
			}
		}
	}

	var id int
	err = sqlTx.QueryRowContext(ctx,
		`INSERT INTO expenses(amount, category, description, date) VALUES($1::float8, $2, $3, $4::date) RETURNING id`,
		tx.Amount, tx.Category, tx.Description, date,
	).Scan(&id)
	if err != nil {
		return Transaction{}, fmt.Errorf("INSERT в expenses: %w", err)
	}
	if err := sqlTx.Commit(); err != nil {
		return Transaction{}, fmt.Errorf("commit: %w", err)
	}

	tx.ID = id
	tx.Date = date
	return tx, nil
}

func (s *pgStore) listTransactions(ctx context.Context) ([]Transaction, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, amount::float8, category, COALESCE(description, ''), to_char(date, 'YYYY-MM-DD')
FROM expenses ORDER BY date DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]Transaction, 0)
	for rows.Next() {
		var tx Transaction
		if err := rows.Scan(&tx.ID, &tx.Amount, &tx.Category, &tx.Description, &tx.Date); err != nil {
			return nil, err
		}
		result = append(result, tx)
	}
	return result, rows.Err()
}

// summary — SUM(amount) GROUP BY category за интервал дат (включительно).
func (s *pgStore) summary(ctx context.Context, from, to string) ([]CategorySummary, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT category, SUM(amount)::float8 FROM expenses
WHERE date BETWEEN $1::date AND $2::date
GROUP BY category ORDER BY category`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]CategorySummary, 0)
	for rows.Next() {
		var it CategorySummary
		if err := rows.Scan(&it.Category, &it.Total); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}
