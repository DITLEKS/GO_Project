package main

import (
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
		if _, err := time.Parse("2006-01-02", t.Date); err != nil {
			return fmt.Errorf("дата %q должна быть в формате ГГГГ-ММ-ДД", t.Date)
		}
	}
	return nil
}

// transactions — хранилище транзакций в памяти (изначально пустой срез).
var transactions = []Transaction{}

// AddTransaction сначала вызывает tx.Validate(): при ошибке транзакция не добавляется,
// ошибка возвращается вызывающему коду. Затем проверяется бюджет категории:
// при превышении лимита возвращается ErrBudgetExceeded и транзакция не сохраняется.
// ID назначается автоматически: len(transactions)+1.
func AddTransaction(tx Transaction) error {
	if err := tx.Validate(); err != nil {
		return err
	}
	if err := checkBudget(tx); err != nil {
		return err
	}

	tx.ID = len(transactions) + 1
	transactions = append(transactions, tx)
	return nil
}

// ListTransactions возвращает копию списка всех транзакций.
func ListTransactions() []Transaction {
	result := make([]Transaction, len(transactions))
	copy(result, transactions)
	return result
}
