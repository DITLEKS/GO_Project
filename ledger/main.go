package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

// loadBudgetsFromFile открывает файл, оборачивает его в bufio.Reader и загружает бюджеты.
func loadBudgetsFromFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("не удалось открыть файл %s: %w", path, err)
	}
	defer f.Close()

	if err := LoadBudgets(bufio.NewReader(f)); err != nil {
		return fmt.Errorf("не удалось загрузить бюджеты из %s: %w", path, err)
	}
	return nil
}

// tryAdd добавляет транзакцию и печатает результат.
func tryAdd(tx Transaction) {
	err := AddTransaction(tx)
	switch {
	case errors.Is(err, ErrBudgetExceeded):
		fmt.Printf("ОТКАЗ: транзакция %q отклонена — %v\n", tx.Description, err)
	case err != nil:
		fmt.Printf("Ошибка добавления транзакции (%s): %v\n", tx.Description, err)
	default:
		fmt.Printf("OK: транзакция %q добавлена (%.2f, %s)\n", tx.Description, tx.Amount, tx.Category)
	}
}

func main() {
	fmt.Println("Ledger service started")

	// 1. Начальные бюджеты через SetBudget.
	initialBudgets := []Budget{
		{Category: "Продукты", Limit: 5000, Period: "месяц"},
		{Category: "Транспорт", Limit: 2000, Period: "месяц"},
	}
	for _, b := range initialBudgets {
		if err := SetBudget(b); err != nil {
			fmt.Printf("Ошибка установки бюджета (%s): %v\n", b.Category, err)
		}
	}

	// 2. Загрузка бюджетов из файла budgets.json (os.Open + bufio.NewReader).
	if err := loadBudgetsFromFile("budgets.json"); err != nil {
		fmt.Printf("Предупреждение: %v\n", err)
	} else {
		fmt.Println("Бюджеты из budgets.json загружены")
	}

	fmt.Println("Текущие бюджеты:")
	for _, b := range ListBudgets() {
		fmt.Printf("Категория=%s | Лимит=%.2f | Период=%s\n", b.Category, b.Limit, b.Period)
	}

	// 3. Демонстрация обработки ошибок ввода-вывода и парсинга.
	fmt.Println("Проверка обработки ошибок I/O:")
	if err := loadBudgetsFromFile("missing.json"); err != nil {
		fmt.Printf("  ошибка: %v\n", err)
	}
	if err := LoadBudgets(strings.NewReader(`{"category": "не массив"}`)); err != nil {
		fmt.Printf("  ошибка: %v\n", err)
	}
	if err := LoadBudgets(strings.NewReader(`[{"category": "Связь", "limit": -10}]`)); err != nil {
		fmt.Printf("  ошибка: %v\n", err)
	}

	// 4. Тестовые сценарии добавления транзакций.
	fmt.Println("Добавление транзакций:")
	testData := []Transaction{
		{Amount: -1500.50, Category: "Продукты", Description: "Покупка продуктов в магазине", Date: "2026-10-01"},
		{Amount: 85000, Category: "Зарплата", Description: "Зарплата за сентябрь", Date: "2026-10-02"},
		{Amount: -450, Category: "Транспорт", Description: "Проездной на метро", Date: "2026-10-03"},
		{Amount: 0, Category: "Прочее", Description: "Некорректная транзакция", Date: "2026-10-04"},
		{Amount: -3000, Category: "Продукты", Description: "Закупка на неделю", Date: "2026-10-05"},
		{Amount: -1000, Category: "Продукты", Description: "Ужин в ресторане", Date: "2026-10-06"},
		{Amount: -1800, Category: "Транспорт", Description: "Такси в аэропорт", Date: "2026-10-07"},
		{Amount: -2500, Category: "Развлечения", Description: "Билеты в кино", Date: "2026-10-08"},
		{Amount: -700, Category: "Развлечения", Description: "Концерт", Date: "2026-10-09"},
		{Amount: -10000, Category: "Техника", Description: "Наушники (без бюджета)", Date: "2026-10-10"},
	}
	for _, tx := range testData {
		tryAdd(tx)
	}

	// 5. Изменение лимита через SetBudget и повторная попытка.
	fmt.Println("Увеличиваем лимит бюджета «Транспорт» до 3000:")
	if err := SetBudget(Budget{Category: "Транспорт", Limit: 3000, Period: "месяц"}); err != nil {
		fmt.Printf("Ошибка установки бюджета: %v\n", err)
	}
	tryAdd(Transaction{Amount: -1800, Category: "Транспорт", Description: "Такси в аэропорт (повтор)", Date: "2026-10-07"})

	// 6. Итоги.
	fmt.Println("Список транзакций:")
	for _, tx := range ListTransactions() {
		fmt.Printf("ID=%d | Сумма=%.2f | Категория=%s | Описание=%s | Дата=%s\n",
			tx.ID, tx.Amount, tx.Category, tx.Description, tx.Date)
	}

	fmt.Println("Расходы по категориям с бюджетом:")
	for _, b := range ListBudgets() {
		fmt.Printf("Категория=%s | Потрачено=%.2f | Лимит=%.2f\n", b.Category, SpentByCategory(b.Category), b.Limit)
	}
}
