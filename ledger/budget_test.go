package ledger

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func resetState() {
	transactions = []Transaction{}
	budgets = map[string]Budget{}
}

type errReader struct{}

func (errReader) Read(p []byte) (int, error) { return 0, errors.New("disk failure") }

func TestSetBudget(t *testing.T) {
	resetState()

	if err := SetBudget(Budget{Category: "Еда", Limit: 5000}); err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if err := SetBudget(Budget{Category: "Еда", Limit: 7000}); err != nil {
		t.Fatalf("неожиданная ошибка при обновлении: %v", err)
	}
	b, ok := GetBudget("Еда")
	if !ok || b.Limit != 7000 {
		t.Fatalf("ожидался лимит 7000, получено %+v (ok=%v)", b, ok)
	}
	if err := SetBudget(Budget{Category: "", Limit: 100}); err == nil {
		t.Fatal("ожидалась ошибка для пустой категории")
	}
	if err := SetBudget(Budget{Category: "Еда", Limit: 0}); err == nil {
		t.Fatal("ожидалась ошибка для нулевого лимита")
	}
}

func TestAddTransactionBudget(t *testing.T) {
	resetState()
	if err := SetBudget(Budget{Category: "Еда", Limit: 5000}); err != nil {
		t.Fatal(err)
	}

	if err := AddTransaction(Transaction{Amount: -3000, Category: "Еда"}); err != nil {
		t.Fatalf("транзакция в пределах бюджета должна быть принята: %v", err)
	}
	if err := AddTransaction(Transaction{Amount: -2000, Category: "Еда"}); err != nil {
		t.Fatalf("транзакция ровно до лимита должна быть принята: %v", err)
	}

	err := AddTransaction(Transaction{Amount: -1, Category: "Еда"})
	if !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("ожидалась ошибка ErrBudgetExceeded, получено: %v", err)
	}
	if len(ListTransactions()) != 2 {
		t.Fatalf("отклонённая транзакция не должна сохраняться, в списке %d", len(ListTransactions()))
	}

	if err := AddTransaction(Transaction{Amount: 1000, Category: "Еда"}); err != nil {
		t.Fatalf("доход не должен ограничиваться бюджетом: %v", err)
	}
	if err := AddTransaction(Transaction{Amount: -99999, Category: "БезБюджета"}); err != nil {
		t.Fatalf("категория без бюджета не должна ограничиваться: %v", err)
	}
	if got := SpentByCategory("Еда"); got != 5000 {
		t.Fatalf("ожидалось потрачено 5000, получено %.2f", got)
	}
}

func TestLoadBudgets(t *testing.T) {
	resetState()

	data := `[{"category": "Еда", "limit": 5000, "period": "месяц"}, {"category": "Транспорт", "limit": 2000}]`
	if err := LoadBudgets(strings.NewReader(data)); err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if len(ListBudgets()) != 2 {
		t.Fatalf("ожидалось 2 бюджета, получено %d", len(ListBudgets()))
	}
	if b, _ := GetBudget("Еда"); b.Limit != 5000 || b.Period != "месяц" {
		t.Fatalf("бюджет загружен неверно: %+v", b)
	}

	if err := LoadBudgets(strings.NewReader(`{"category": "x"}`)); err == nil {
		t.Fatal("ожидалась ошибка парсинга для не-массива")
	}
	if err := LoadBudgets(strings.NewReader(`[{"category": "x", "limit": -5}]`)); err == nil {
		t.Fatal("ожидалась ошибка валидации для отрицательного лимита")
	}
	if err := LoadBudgets(strings.NewReader("   ")); err == nil {
		t.Fatal("ожидалась ошибка для пустого источника")
	}
	if err := LoadBudgets(errReader{}); err == nil {
		t.Fatal("ожидалась ошибка чтения")
	}
	var nilReader io.Reader
	if err := LoadBudgets(nilReader); err == nil {
		t.Fatal("ожидалась ошибка для nil reader")
	}
}
