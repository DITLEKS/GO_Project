package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/DITLEKS/GO_Project/ledger"
)

// Интеграционные тесты работают с общим состоянием ledger, поэтому не параллельны
// и в начале каждого вызывают ledger.Reset() (через newRouter).

func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

func newRouter(t *testing.T) http.Handler {
	t.Helper()
	ledger.Reset()
	t.Cleanup(ledger.Reset)
	return NewRouter()
}

func doRequest(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func requireJSON(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("статус %d, ожидался %d, тело: %s", rec.Code, wantStatus, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type %q, ожидался application/json; charset=utf-8", ct)
	}
}

func decodeBody[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("не удалось разобрать JSON %q: %v", rec.Body.String(), err)
	}
	return v
}

// requireError проверяет статус, Content-Type и формат {"error":"..."}.
// Если wantMsg пустой, проверяется лишь непустота сообщения.
func requireError(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantMsg string) {
	t.Helper()
	requireJSON(t, rec, wantStatus)
	body := decodeBody[map[string]string](t, rec)
	msg, ok := body["error"]
	if !ok || len(body) != 1 {
		t.Fatalf("ожидался ровно один ключ \"error\", получено %v", body)
	}
	if msg == "" {
		t.Fatal("сообщение об ошибке пустое")
	}
	if wantMsg != "" && msg != wantMsg {
		t.Fatalf("ошибка %q, ожидалась %q", msg, wantMsg)
	}
}

const budgetFood = `{"category":"еда","limit":5000}`

func TestBudgetsAPI(t *testing.T) {
	h := newRouter(t)

	t.Run("list_empty", func(t *testing.T) {
		rec := doRequest(h, http.MethodGet, "/api/budgets", "")
		requireJSON(t, rec, http.StatusOK)
		if got := strings.TrimSpace(rec.Body.String()); got != "[]" {
			t.Fatalf("ожидался [], получено %s", got)
		}
	})

	invalid := []struct{ name, body string }{
		{"bad_json", `{"category":`},
		{"not_object", `[]`},
		{"empty_body", ``},
		{"unknown_field", `{"category":"еда","limit":1,"x":1}`},
		{"trailing_data", `{"category":"еда","limit":1} {}`},
		{"limit_as_string", `{"category":"еда","limit":"много"}`},
		{"limit_zero", `{"category":"еда","limit":0}`},
		{"limit_negative", `{"category":"еда","limit":-10}`},
		{"empty_category", `{"category":"","limit":10}`},
		{"blank_category", `{"category":"   ","limit":10}`},
	}
	for _, tc := range invalid {
		tc := tc
		t.Run("invalid_"+tc.name, func(t *testing.T) {
			requireError(t, doRequest(h, http.MethodPost, "/api/budgets", tc.body), http.StatusBadRequest, "")
		})
	}

	t.Run("invalid_requests_store_nothing", func(t *testing.T) {
		rec := doRequest(h, http.MethodGet, "/api/budgets", "")
		requireJSON(t, rec, http.StatusOK)
		if got := decodeBody[[]BudgetResponse](t, rec); len(got) != 0 {
			t.Fatalf("невалидные запросы сохранили бюджеты: %+v", got)
		}
	})

	t.Run("create_ok", func(t *testing.T) {
		rec := doRequest(h, http.MethodPost, "/api/budgets", budgetFood)
		requireJSON(t, rec, http.StatusCreated)
		got := decodeBody[BudgetResponse](t, rec)
		if got.Category != "еда" || got.Limit != 5000 {
			t.Fatalf("неверный ответ: %+v", got)
		}
	})

	t.Run("list_contains_created", func(t *testing.T) {
		rec := doRequest(h, http.MethodGet, "/api/budgets", "")
		requireJSON(t, rec, http.StatusOK)
		list := decodeBody[[]BudgetResponse](t, rec)
		if len(list) != 1 || list[0].Category != "еда" || list[0].Limit != 5000 {
			t.Fatalf("неверный список: %+v", list)
		}
	})

	t.Run("update_existing", func(t *testing.T) {
		rec := doRequest(h, http.MethodPost, "/api/budgets", `{"category":" еда ","limit":7000,"period":"месяц"}`)
		requireJSON(t, rec, http.StatusCreated)
		if got := decodeBody[BudgetResponse](t, rec); got.Category != "еда" || got.Limit != 7000 || got.Period != "месяц" {
			t.Fatalf("неверный ответ: %+v", got)
		}
		list := decodeBody[[]BudgetResponse](t, doRequest(h, http.MethodGet, "/api/budgets", ""))
		if len(list) != 1 || list[0].Limit != 7000 {
			t.Fatalf("бюджет не обновился: %+v", list)
		}
	})
}

func TestTransactionsAPI(t *testing.T) {
	h := newRouter(t)

	t.Run("list_empty", func(t *testing.T) {
		rec := doRequest(h, http.MethodGet, "/api/transactions", "")
		requireJSON(t, rec, http.StatusOK)
		if got := strings.TrimSpace(rec.Body.String()); got != "[]" {
			t.Fatalf("ожидался [], получено %s", got)
		}
	})

	t.Run("setup_budget", func(t *testing.T) {
		requireJSON(t, doRequest(h, http.MethodPost, "/api/budgets", budgetFood), http.StatusCreated)
	})

	t.Run("ok", func(t *testing.T) {
		rec := doRequest(h, http.MethodPost, "/api/transactions",
			`{"amount":-1000,"category":"еда","description":"обед","date":"2025-09-10"}`)
		requireJSON(t, rec, http.StatusCreated)
		got := decodeBody[TransactionResponse](t, rec)
		want := TransactionResponse{ID: 1, Amount: -1000, Category: "еда", Description: "обед", Date: "2025-09-10"}
		if got != want {
			t.Fatalf("получено %+v, ожидалось %+v", got, want)
		}
	})

	t.Run("list_contains_created", func(t *testing.T) {
		rec := doRequest(h, http.MethodGet, "/api/transactions", "")
		requireJSON(t, rec, http.StatusOK)
		list := decodeBody[[]TransactionResponse](t, rec)
		if len(list) != 1 || list[0].ID != 1 || list[0].Category != "еда" || list[0].Amount != -1000 {
			t.Fatalf("неверный список: %+v", list)
		}
	})

	t.Run("exceeded", func(t *testing.T) {
		rec := doRequest(h, http.MethodPost, "/api/transactions",
			`{"amount":-4500,"category":"еда","description":"банкет","date":"2025-09-11"}`)
		requireError(t, rec, http.StatusConflict, "budget exceeded")
	})

	t.Run("exceeded_not_stored", func(t *testing.T) {
		list := decodeBody[[]TransactionResponse](t, doRequest(h, http.MethodGet, "/api/transactions", ""))
		if len(list) != 1 {
			t.Fatalf("отклонённая транзакция сохранилась: %+v", list)
		}
	})

	t.Run("exact_limit_ok", func(t *testing.T) {
		rec := doRequest(h, http.MethodPost, "/api/transactions", `{"amount":-4000,"category":"еда","date":"2025-09-12"}`)
		requireJSON(t, rec, http.StatusCreated)
		if id := decodeBody[TransactionResponse](t, rec).ID; id != 2 {
			t.Fatalf("ожидался ID 2, получено %d", id)
		}
	})

	t.Run("income_ok", func(t *testing.T) {
		requireJSON(t, doRequest(h, http.MethodPost, "/api/transactions", `{"amount":100,"category":"еда"}`), http.StatusCreated)
	})

	invalid := []struct{ name, body string }{
		{"bad_json", `{"amount":`},
		{"unknown_field", `{"amount":1,"category":"еда","x":1}`},
		{"amount_as_string", `{"amount":"много","category":"еда"}`},
		{"zero_amount", `{"amount":0,"category":"еда"}`},
		{"empty_category", `{"amount":10,"category":""}`},
		{"bad_date", `{"amount":10,"category":"еда","date":"10.09.2025"}`},
	}
	for _, tc := range invalid {
		tc := tc
		t.Run("invalid_"+tc.name, func(t *testing.T) {
			requireError(t, doRequest(h, http.MethodPost, "/api/transactions", tc.body), http.StatusBadRequest, "")
		})
	}

	t.Run("final_count", func(t *testing.T) {
		list := decodeBody[[]TransactionResponse](t, doRequest(h, http.MethodGet, "/api/transactions", ""))
		if len(list) != 3 {
			t.Fatalf("ожидалось 3 транзакции, получено %d: %+v", len(list), list)
		}
	})
}

// Маршруты ошибок и /ping не обращаются к ledger, поэтому этот тест можно запускать параллельно.
func TestRoutingErrors(t *testing.T) {
	t.Parallel()
	h := NewRouter()

	tests := []struct {
		name     string
		method   string
		path     string
		want     int
		wantMsg  string
		wantAllw string
	}{
		{"unknown_root", http.MethodGet, "/unknown", http.StatusNotFound, "not found", ""},
		{"unknown_api", http.MethodGet, "/api/nope", http.StatusNotFound, "not found", ""},
		{"transactions_delete", http.MethodDelete, "/api/transactions", http.StatusMethodNotAllowed, "method not allowed", "GET, POST"},
		{"budgets_put", http.MethodPut, "/api/budgets", http.StatusMethodNotAllowed, "method not allowed", "GET, POST"},
		{"ping_post", http.MethodPost, "/ping", http.StatusMethodNotAllowed, "method not allowed", "GET"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rec := doRequest(h, tc.method, tc.path, "")
			requireError(t, rec, tc.want, tc.wantMsg)
			if got := rec.Header().Get("Allow"); got != tc.wantAllw {
				t.Fatalf("Allow %q, ожидался %q", got, tc.wantAllw)
			}
		})
	}

	t.Run("ping_ok", func(t *testing.T) {
		t.Parallel()
		rec := doRequest(h, http.MethodGet, "/ping", "")
		requireJSON(t, rec, http.StatusOK)
		if got := decodeBody[map[string]string](t, rec); got["message"] != "pong" {
			t.Fatalf("неверный ответ: %v", got)
		}
	})
}

func TestLoggingMiddleware(t *testing.T) {
	h := newRouter(t)

	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(io.Discard) })

	doRequest(h, http.MethodGet, "/api/budgets", "")
	doRequest(h, http.MethodPost, "/api/budgets", `{"category":"еда","limit":1}`)

	out := buf.String()
	for _, want := range []string{"GET /api/budgets", "POST /api/budgets"} {
		if !strings.Contains(out, want) {
			t.Fatalf("в логе нет %q: %s", want, out)
		}
	}
}
