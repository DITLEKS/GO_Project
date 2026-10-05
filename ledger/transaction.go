package main

import (
	"errors"
	"strings"
)

// Transaction описывает финансовую транзакцию.
type Transaction struct {
	ID          int
	Amount      float64
	Category    string
	Description string
	Date        string
}

// transactions — хранилище транзакций в памяти (изначально пустой срез).
var transactions = []Transaction{}

// AddTransaction проверяет данные и добавляет транзакцию в хранилище.
// ID назначается автоматически: len(transactions)+1.
func AddTransaction(tx Transaction) error {
	if tx.Amount == 0 {
		return errors.New("сумма транзакции не может быть равна нулю")
	}
	if strings.TrimSpace(tx.Category) == "" {
		return errors.New("категория транзакции не может быть пустой")
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
