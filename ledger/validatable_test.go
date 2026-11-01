package ledger

import "testing"

func TestTransactionValidate(t *testing.T) {
	cases := []struct {
		name    string
		tx      Transaction
		wantErr bool
	}{
		{"доход", Transaction{Amount: 100, Category: "Зарплата", Date: "2026-10-01"}, false},
		{"расход", Transaction{Amount: -50, Category: "Еда"}, false},
		{"нулевая сумма", Transaction{Amount: 0, Category: "Еда"}, true},
		{"пустая категория", Transaction{Amount: 10, Category: "  "}, true},
		{"неверная дата", Transaction{Amount: 10, Category: "Еда", Date: "01.10.2026"}, true},
	}
	for _, c := range cases {
		if err := c.tx.Validate(); (err != nil) != c.wantErr {
			t.Errorf("%s: ошибка=%v, ожидалась ошибка=%v", c.name, err, c.wantErr)
		}
	}
}

func TestBudgetValidate(t *testing.T) {
	if err := (Budget{Category: "Еда", Limit: 100}).Validate(); err != nil {
		t.Errorf("неожиданная ошибка: %v", err)
	}
	if err := (Budget{Category: "", Limit: 100}).Validate(); err == nil {
		t.Error("ожидалась ошибка для пустой категории")
	}
	if err := (Budget{Category: "Еда", Limit: 0}).Validate(); err == nil {
		t.Error("ожидалась ошибка для нулевого лимита")
	}
	if err := (Budget{Category: "Еда", Limit: -5}).Validate(); err == nil {
		t.Error("ожидалась ошибка для отрицательного лимита")
	}
}

func TestCheckValidPolymorphism(t *testing.T) {
	var items = []Validatable{
		Transaction{Amount: 1, Category: "A"},
		Budget{Category: "A", Limit: 1},
	}
	for _, v := range items {
		if err := CheckValid(v); err != nil {
			t.Errorf("%T: %v", v, err)
		}
	}
	if CheckValid(Transaction{}) == nil || CheckValid(Budget{}) == nil {
		t.Error("ожидались ошибки для пустых структур")
	}
}

func TestInvalidNotStored(t *testing.T) {
	resetState()
	if err := AddTransaction(Transaction{Amount: 5, Category: ""}); err == nil {
		t.Fatal("ожидалась ошибка валидации")
	}
	if err := SetBudget(Budget{Category: "X", Limit: -1}); err == nil {
		t.Fatal("ожидалась ошибка валидации")
	}
	if len(ListTransactions()) != 0 || len(ListBudgets()) != 0 {
		t.Fatal("невалидные данные не должны сохраняться")
	}
}
