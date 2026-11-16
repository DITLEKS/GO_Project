// Package domain holds the entities and contracts of Ledger.
// It depends only on the standard library: no SQL, no HTTP.
package domain

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

// DateLayout is the only accepted date format.
const DateLayout = "2006-01-02"

// MaxAmount is the upper bound for money values.
const MaxAmount = 999999999999.99

// Transaction: negative amounts are expenses, positive amounts are income.
type Transaction struct {
	ID          int
	Amount      float64
	Category    string
	Description string
	Date        string
}

// Validate only returns errors, never logs.
func (t Transaction) Validate() error {
	if t.Amount == 0 || math.IsNaN(t.Amount) || math.IsInf(t.Amount, 0) {
		return errors.New("сумма транзакции не может быть равна нулю или быть некорректным числом")
	}
	if math.Abs(t.Amount) > MaxAmount || math.Round(t.Amount*100) == 0 {
		return errors.New("сумма должна помещаться в NUMERIC(14,2) и быть не меньше копейки после округления")
	}
	if strings.TrimSpace(t.Category) == "" {
		return errors.New("категория транзакции не может быть пустой")
	}
	if t.Date != "" {
		if _, err := time.Parse(DateLayout, t.Date); err != nil {
			return fmt.Errorf("дата %q должна быть в формате ГГГГ-ММ-ДД", t.Date)
		}
	}
	return nil
}
