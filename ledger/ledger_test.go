package ledger

import (
	"errors"
	"math"
	"strings"
	"testing"
)

func TestTransactionValidateTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		tx          Transaction
		wantErr     bool
		errContains string
	}{
		{"valid_income", Transaction{Amount: 100, Category: "Зарплата", Date: "2026-10-01"}, false, ""},
		{"valid_expense", Transaction{Amount: -50, Category: "еда"}, false, ""},
		{"valid_fractional", Transaction{Amount: 0.01, Category: "еда"}, false, ""},
		{"empty_date_allowed", Transaction{Amount: 10, Category: "еда", Date: ""}, false, ""},
		{"zero_amount", Transaction{Amount: 0, Category: "еда"}, true, "сумма"},
		{"nan_amount", Transaction{Amount: math.NaN(), Category: "еда"}, true, "сумма"},
		{"positive_inf_amount", Transaction{Amount: math.Inf(1), Category: "еда"}, true, "сумма"},
		{"negative_inf_amount", Transaction{Amount: math.Inf(-1), Category: "еда"}, true, "сумма"},
		{"empty_category", Transaction{Amount: 10, Category: ""}, true, "категория"},
		{"blank_category", Transaction{Amount: 10, Category: "   "}, true, "категория"},
		{"bad_date_format", Transaction{Amount: 10, Category: "еда", Date: "10.09.2025"}, true, "дата"},
		{"bad_date_value", Transaction{Amount: 10, Category: "еда", Date: "2025-13-40"}, true, "дата"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.tx.Validate()
			if !tc.wantErr {
				if err != nil {
					t.Fatalf("неожиданная ошибка: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("ожидалась ошибка, получен nil")
			}
			if !strings.Contains(err.Error(), tc.errContains) {
				t.Fatalf("ошибка %q не содержит %q", err, tc.errContains)
			}
		})
	}
}

func TestBudgetValidateTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		b           Budget
		wantErr     bool
		errContains string
	}{
		{"valid", Budget{Category: "еда", Limit: 5000}, false, ""},
		{"valid_with_period", Budget{Category: "еда", Limit: 5000, Period: "месяц"}, false, ""},
		{"valid_tiny_limit", Budget{Category: "еда", Limit: 0.01}, false, ""},
		{"empty_category", Budget{Category: "", Limit: 100}, true, "категория"},
		{"blank_category", Budget{Category: "  ", Limit: 100}, true, "категория"},
		{"zero_limit", Budget{Category: "еда", Limit: 0}, true, "лимит"},
		{"negative_limit", Budget{Category: "еда", Limit: -10}, true, "лимит"},
		{"nan_limit", Budget{Category: "еда", Limit: math.NaN()}, true, "лимит"},
		{"inf_limit", Budget{Category: "еда", Limit: math.Inf(1)}, true, "лимит"},
		{"empty_category_and_bad_limit", Budget{Category: "", Limit: -1}, true, "категория"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.b.Validate()
			if !tc.wantErr {
				if err != nil {
					t.Fatalf("неожиданная ошибка: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("ожидалась ошибка, получен nil")
			}
			if !strings.Contains(err.Error(), tc.errContains) {
				t.Fatalf("ошибка %q не содержит %q", err, tc.errContains)
			}
		})
	}
}

// Тесты ниже работают с общим состоянием пакета, поэтому не параллельны и вызывают Reset.

func TestBudgetExceededFlow(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	if err := SetBudget(Budget{Category: "еда", Limit: 5000}); err != nil {
		t.Fatalf("SetBudget: %v", err)
	}

	t.Run("within_limit", func(t *testing.T) {
		if err := AddTransaction(Transaction{Amount: -3000, Category: "еда", Date: "2025-09-10"}); err != nil {
			t.Fatalf("ожидался успех, получено: %v", err)
		}
		if n := len(ListTransactions()); n != 1 {
			t.Fatalf("ожидалась 1 транзакция, получено %d", n)
		}
	})

	t.Run("exceeded", func(t *testing.T) {
		before := len(ListTransactions())
		err := AddTransaction(Transaction{Amount: -2500, Category: "еда", Date: "2025-09-11"})
		if !errors.Is(err, ErrBudgetExceeded) {
			t.Fatalf("ожидалась ErrBudgetExceeded, получено: %v", err)
		}
		if after := len(ListTransactions()); after != before {
			t.Fatalf("отклонённая транзакция сохранилась: было %d, стало %d", before, after)
		}
		if got := SpentByCategory("еда"); got != 3000 {
			t.Fatalf("ожидалось потрачено 3000, получено %.2f", got)
		}
	})

	t.Run("exact_limit", func(t *testing.T) {
		if err := AddTransaction(Transaction{Amount: -2000, Category: "еда"}); err != nil {
			t.Fatalf("ровно до лимита должно проходить: %v", err)
		}
		if n := len(ListTransactions()); n != 2 {
			t.Fatalf("ожидалось 2 транзакции, получено %d", n)
		}
	})

	t.Run("one_kopeck_over", func(t *testing.T) {
		err := AddTransaction(Transaction{Amount: -0.01, Category: "еда"})
		if !errors.Is(err, ErrBudgetExceeded) {
			t.Fatalf("ожидалась ErrBudgetExceeded, получено: %v", err)
		}
		if n := len(ListTransactions()); n != 2 {
			t.Fatalf("ожидалось 2 транзакции, получено %d", n)
		}
	})

	t.Run("income_not_limited", func(t *testing.T) {
		if err := AddTransaction(Transaction{Amount: 10000, Category: "еда"}); err != nil {
			t.Fatalf("доход не должен ограничиваться бюджетом: %v", err)
		}
	})

	t.Run("category_without_budget", func(t *testing.T) {
		if err := AddTransaction(Transaction{Amount: -99999, Category: "такси"}); err != nil {
			t.Fatalf("категория без бюджета не должна ограничиваться: %v", err)
		}
	})

	t.Run("invalid_is_not_budget_error", func(t *testing.T) {
		before := len(ListTransactions())
		err := AddTransaction(Transaction{Amount: 0, Category: "еда"})
		if err == nil || errors.Is(err, ErrBudgetExceeded) {
			t.Fatalf("ожидалась ошибка валидации, получено: %v", err)
		}
		if after := len(ListTransactions()); after != before {
			t.Fatalf("невалидная транзакция сохранилась")
		}
	})
}

func TestSetBudgetUpdatesAndTrims(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	t.Run("update_replaces_limit", func(t *testing.T) {
		if err := SetBudget(Budget{Category: "еда", Limit: 5000}); err != nil {
			t.Fatal(err)
		}
		if err := SetBudget(Budget{Category: "еда", Limit: 7000, Period: "месяц"}); err != nil {
			t.Fatal(err)
		}
		b, ok := GetBudget("еда")
		if !ok || b.Limit != 7000 || b.Period != "месяц" {
			t.Fatalf("неверный бюджет: %+v (ok=%v)", b, ok)
		}
		if n := len(ListBudgets()); n != 1 {
			t.Fatalf("ожидался 1 бюджет, получено %d", n)
		}
	})

	t.Run("category_is_trimmed", func(t *testing.T) {
		if err := SetBudget(Budget{Category: "  транспорт  ", Limit: 100}); err != nil {
			t.Fatal(err)
		}
		if _, ok := GetBudget("транспорт"); !ok {
			t.Fatal("бюджет не найден по категории без пробелов")
		}
	})

	t.Run("invalid_not_stored", func(t *testing.T) {
		before := len(ListBudgets())
		if err := SetBudget(Budget{Category: "кино", Limit: 0}); err == nil {
			t.Fatal("ожидалась ошибка")
		}
		if after := len(ListBudgets()); after != before {
			t.Fatalf("невалидный бюджет сохранился")
		}
	})
}

func TestReset(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	if err := SetBudget(Budget{Category: "еда", Limit: 100}); err != nil {
		t.Fatal(err)
	}
	if err := AddTransaction(Transaction{Amount: 5, Category: "еда"}); err != nil {
		t.Fatal(err)
	}

	Reset()

	if n := len(ListTransactions()); n != 0 {
		t.Fatalf("после Reset ожидались 0 транзакций, получено %d", n)
	}
	if n := len(ListBudgets()); n != 0 {
		t.Fatalf("после Reset ожидались 0 бюджетов, получено %d", n)
	}

	if err := AddTransaction(Transaction{Amount: 1, Category: "x"}); err != nil {
		t.Fatal(err)
	}
	if id := ListTransactions()[0].ID; id != 1 {
		t.Fatalf("после Reset ID должен начинаться с 1, получено %d", id)
	}
}
