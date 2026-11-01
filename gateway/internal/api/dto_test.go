package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DITLEKS/GO_Project/ledger"
)

// Все тесты в этом файле — чистые функции без общего состояния, поэтому параллельны.

func TestTransactionRequestToDomain(t *testing.T) {
	t.Parallel()

	got := CreateTransactionRequest{Amount: -10.5, Category: "  еда  ", Description: "обед", Date: "2025-09-10"}.ToDomain()
	want := ledger.Transaction{Amount: -10.5, Category: "еда", Description: "обед", Date: "2025-09-10"}
	if got != want {
		t.Fatalf("получено %+v, ожидалось %+v", got, want)
	}
}

func TestBudgetRequestToDomain(t *testing.T) {
	t.Parallel()

	got := CreateBudgetRequest{Category: " еда ", Limit: 100, Period: "месяц"}.ToDomain()
	want := ledger.Budget{Category: "еда", Limit: 100, Period: "месяц"}
	if got != want {
		t.Fatalf("получено %+v, ожидалось %+v", got, want)
	}
}

func TestResponsesFromDomain(t *testing.T) {
	t.Parallel()

	tx := NewTransactionResponse(ledger.Transaction{ID: 7, Amount: 5, Category: "а", Description: "б", Date: "2025-01-02"})
	if tx != (TransactionResponse{ID: 7, Amount: 5, Category: "а", Description: "б", Date: "2025-01-02"}) {
		t.Fatalf("неверный TransactionResponse: %+v", tx)
	}

	b := NewBudgetResponse(ledger.Budget{Category: "а", Limit: 9, Period: "месяц"})
	if b != (BudgetResponse{Category: "а", Limit: 9, Period: "месяц"}) {
		t.Fatalf("неверный BudgetResponse: %+v", b)
	}
}

func TestEmptyListsMarshalToArray(t *testing.T) {
	t.Parallel()

	cases := map[string]any{
		"transactions_nil":   NewTransactionResponses(nil),
		"transactions_empty": NewTransactionResponses([]ledger.Transaction{}),
		"budgets_nil":        NewBudgetResponses(nil),
		"budgets_empty":      NewBudgetResponses([]ledger.Budget{}),
	}
	for name, v := range cases {
		name, v := name, v
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			data, err := json.Marshal(v)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != "[]" {
				t.Fatalf("ожидался [], получено %s", data)
			}
		})
	}
}

func TestNonEmptyListsKeepOrder(t *testing.T) {
	t.Parallel()

	txs := NewTransactionResponses([]ledger.Transaction{{ID: 1}, {ID: 2}, {ID: 3}})
	if len(txs) != 3 || txs[0].ID != 1 || txs[2].ID != 3 {
		t.Fatalf("неверный список: %+v", txs)
	}
	bs := NewBudgetResponses([]ledger.Budget{{Category: "a"}, {Category: "b"}})
	if len(bs) != 2 || bs[1].Category != "b" {
		t.Fatalf("неверный список: %+v", bs)
	}
}

func TestChainOrder(t *testing.T) {
	t.Parallel()

	var order []string
	mark := func(name string) Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name)
				next.ServeHTTP(w, r)
			})
		}
	}
	h := Chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		order = append(order, "handler")
	}), mark("first"), mark("second"))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	if got := strings.Join(order, ","); got != "first,second,handler" {
		t.Fatalf("неверный порядок: %s", got)
	}
}
