package ledger

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

// Transaction описывает финансовую транзакцию.
// Отрицательная сумма — расход, положительная — доход.
type Transaction struct {
	ID          int
	Amount      float64
	Category    string
	Description string
	Date        string
}

// Validate реализует Validatable. Сумма не может быть нулевой (отрицательная
// допустима — это расход), категория не пустая, непустая дата — в формате ГГГГ-ММ-ДД.
// Метод ничего не логирует, только возвращает ошибку.
func (t Transaction) Validate() error {
	if t.Amount == 0 || math.IsNaN(t.Amount) || math.IsInf(t.Amount, 0) {
		return errors.New("сумма транзакции не может быть равна нулю или быть некорректным числом")
	}
	if strings.TrimSpace(t.Category) == "" {
		return errors.New("категория транзакции не может быть пустой")
	}
	if t.Date != "" {
		if _, err := time.Parse(dateLayout, t.Date); err != nil {
			return fmt.Errorf("дата %q должна быть в формате ГГГГ-ММ-ДД", t.Date)
		}
	}
	return nil
}

// CreateTransaction валидирует транзакцию, проверяет бюджет категории и сохраняет её.
// При превышении лимита возвращает ErrBudgetExceeded, запись не добавляется.
// Возвращает созданную транзакцию с назначенным ID (в PostgreSQL — RETURNING id).
func CreateTransaction(tx Transaction) (Transaction, error) {
	if err := tx.Validate(); err != nil {
		return Transaction{}, err
	}
	created, err := store.addTransaction(context.Background(), tx)
	if err != nil {
		if errors.Is(err, ErrBudgetExceeded) {
			return Transaction{}, err
		}
		return Transaction{}, fmt.Errorf("не удалось сохранить транзакцию: %w", err)
	}
	return created, nil
}

// AddTransaction — CreateTransaction без возврата созданной записи.
func AddTransaction(tx Transaction) error {
	_, err := CreateTransaction(tx)
	return err
}

// ListTransactions возвращает все транзакции. В PostgreSQL — ORDER BY date DESC, id DESC;
// в памяти — в порядке добавления.
func ListTransactions() ([]Transaction, error) {
	list, err := store.listTransactions(context.Background())
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать транзакции: %w", err)
	}
	return list, nil
}
