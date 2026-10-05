package main

import "testing"

func TestAddAndListTransactions(t *testing.T) {
	transactions = []Transaction{}

	if err := AddTransaction(Transaction{Amount: 100, Category: "Тест"}); err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if err := AddTransaction(Transaction{Amount: -50, Category: "Тест"}); err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if err := AddTransaction(Transaction{Amount: 0, Category: "Тест"}); err == nil {
		t.Fatal("ожидалась ошибка для нулевой суммы")
	}

	list := ListTransactions()
	if len(list) != 2 {
		t.Fatalf("ожидалось 2 транзакции, получено %d", len(list))
	}
	if list[0].ID != 1 || list[1].ID != 2 {
		t.Fatalf("неверные ID: %d, %d", list[0].ID, list[1].ID)
	}
}
