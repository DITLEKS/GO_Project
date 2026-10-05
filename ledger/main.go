package main

import "fmt"

func main() {
	fmt.Println("Ledger service started")

	testData := []Transaction{
		{Amount: -1500.50, Category: "Продукты", Description: "Покупка продуктов в магазине", Date: "2026-10-01"},
		{Amount: 85000, Category: "Зарплата", Description: "Зарплата за сентябрь", Date: "2026-10-02"},
		{Amount: -450, Category: "Транспорт", Description: "Проездной на метро", Date: "2026-10-03"},
		{Amount: 0, Category: "Прочее", Description: "Некорректная транзакция", Date: "2026-10-04"},
	}

	for _, tx := range testData {
		if err := AddTransaction(tx); err != nil {
			fmt.Printf("Ошибка добавления транзакции (%s): %v\n", tx.Description, err)
		}
	}

	fmt.Println("Список транзакций:")
	for _, tx := range ListTransactions() {
		fmt.Printf("ID=%d | Сумма=%.2f | Категория=%s | Описание=%s | Дата=%s\n",
			tx.ID, tx.Amount, tx.Category, tx.Description, tx.Date)
	}
}
