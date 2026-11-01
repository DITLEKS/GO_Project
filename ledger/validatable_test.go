package ledger

import "testing"

// Table-driven тесты Validate() для Transaction и Budget находятся в ledger_test.go.

func TestCheckValidPolymorphism(t *testing.T) {
	t.Parallel()

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
	Reset()
	t.Cleanup(Reset)

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
