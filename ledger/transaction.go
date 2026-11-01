package ledger

import (
 "context"
 "database/sql"
 "errors"
 "fmt"
 "math"
 "strings"
 "time"
)

// Transaction: negative amounts are expenses, positive amounts are income.
type Transaction struct {
 ID int
 Amount float64
 Category string
 Description string
 Date string
}
// Validate only returns errors, never logs.
func (t Transaction) Validate() error {
 if t.Amount == 0 || math.IsNaN(t.Amount) || math.IsInf(t.Amount, 0) { return errors.New("сумма транзакции не может быть равна нулю или быть некорректным числом") }
 if math.Abs(t.Amount) > 999999999999.99 || math.Round(t.Amount*100) == 0 { return errors.New("сумма должна помещаться в NUMERIC(14,2) и быть не меньше копейки после округления") }
 if strings.TrimSpace(t.Category) == "" { return errors.New("категория транзакции не может быть пустой") }
 if t.Date != "" { if _, err := time.Parse("2006-01-02", t.Date); err != nil { return fmt.Errorf("дата %q должна быть в формате ГГГГ-ММ-ДД", t.Date) } }
 return nil
}
// AddTransaction preserves the command API; HTTP uses CreateTransaction for its ID.
func AddTransaction(tx Transaction) error { _, err := CreateTransaction(tx); return err }
func lockCategory(ctx context.Context, tx *sql.Tx, category string) error {
 // Shared by budget upserts and expense inserts, including absent budget rows.
 _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", category)
 return err
}
func CreateTransaction(tx Transaction) (Transaction, error) {
 if err := tx.Validate(); err != nil { return Transaction{}, err }
 tx.Category = strings.TrimSpace(tx.Category)
 tx.Amount = math.Round(tx.Amount*100)/100
 if tx.Date == "" { tx.Date = time.Now().UTC().Format("2006-01-02") }
 db, err := dbConn(); if err != nil { return Transaction{}, err }
 ctx, cancel := operationContext(); defer cancel()
 sqlTx, err := db.BeginTx(ctx, nil); if err != nil { return Transaction{}, fmt.Errorf("begin transaction: %w", err) }; defer sqlTx.Rollback()
 if err := lockCategory(ctx, sqlTx, tx.Category); err != nil { return Transaction{}, err }
 if tx.Amount < 0 {
  var limit string
  err := sqlTx.QueryRowContext(ctx, "SELECT limit_amount FROM budgets WHERE category=$1", tx.Category).Scan(&limit)
  if err != nil && !errors.Is(err, sql.ErrNoRows) { return Transaction{}, fmt.Errorf("read budget: %w", err) }
  if err == nil {
   var exceeded bool
   // Income never cancels spent money; compare exact NUMERIC amounts in SQL.
   err = sqlTx.QueryRowContext(ctx, `SELECT COALESCE(SUM(-amount),0) + $2::numeric > $3::numeric
                FROM expenses WHERE category=$1 AND amount < 0`, tx.Category, fmt.Sprintf("%.2f", -tx.Amount), limit).Scan(&exceeded)
   if err != nil { return Transaction{}, fmt.Errorf("check budget: %w", err) }
   if exceeded { return Transaction{}, fmt.Errorf("%w: category %q", ErrBudgetExceeded, tx.Category) }
  }
 }
 err = sqlTx.QueryRowContext(ctx, `INSERT INTO expenses(amount,category,description,date)
        VALUES($1,$2,$3,$4) RETURNING id`, fmt.Sprintf("%.2f",tx.Amount), tx.Category, tx.Description, tx.Date).Scan(&tx.ID)
 if err != nil { return Transaction{}, fmt.Errorf("insert expense: %w", err) }
 if err := sqlTx.Commit(); err != nil { return Transaction{}, fmt.Errorf("commit expense: %w", err) }
 invalidateReports(ctx)
 return tx, nil
}
func ListTransactions() ([]Transaction, error) {
 db, err := dbConn(); if err != nil { return nil, err }
 ctx, cancel := operationContext(); defer cancel()
 rows, err := db.QueryContext(ctx, `SELECT id,amount,category,COALESCE(description,''),date
        FROM expenses ORDER BY date DESC,id DESC`)
 if err != nil { return nil, fmt.Errorf("list expenses: %w", err) }; defer rows.Close()
 result := make([]Transaction,0)
 for rows.Next() {
  var tx Transaction; var date time.Time
  if err := rows.Scan(&tx.ID,&tx.Amount,&tx.Category,&tx.Description,&date); err != nil { return nil, err }
  tx.Date = date.Format("2006-01-02"); result = append(result,tx)
 }
 return result, rows.Err()
}
