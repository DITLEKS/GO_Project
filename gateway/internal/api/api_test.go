package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var testRouter = NewRouter()

func do(method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)
	return rec
}

func assertJSON(t *testing.T, rec *httptest.ResponseRecorder, status int) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("статус %d, ожидался %d, тело: %s", rec.Code, status, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != contentTypeJSON {
		t.Fatalf("Content-Type %q, ожидался %q", ct, contentTypeJSON)
	}
}

func errorText(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var e ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
		t.Fatalf("ответ не является {\"error\":...}: %v", err)
	}
	return e.Error
}

func TestCreateTransactionValidationError(t *testing.T) {
	rec := do(http.MethodPost, "/api/transactions", `{"amount":0,"category":"gw-test"}`)
	assertJSON(t, rec, http.StatusBadRequest)
	if errorText(t, rec) == "" {
		t.Fatal("ожидалось описание ошибки")
	}
}

func TestCreateTransactionBadJSON(t *testing.T) {
	assertJSON(t, do(http.MethodPost, "/api/transactions", `{oops`), http.StatusBadRequest)
	assertJSON(t, do(http.MethodPost, "/api/transactions", `{"amount":1,"category":"x","unknown":1}`), http.StatusBadRequest)
}

func TestCreateAndListTransaction(t *testing.T) {
	rec := do(http.MethodPost, "/api/transactions",
		`{"amount":450,"category":"gw-create","description":"ланч","date":"2025-09-10"}`)
	assertJSON(t, rec, http.StatusCreated)

	var created TransactionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID == 0 || created.Category != "gw-create" || created.Amount != 450 {
		t.Fatalf("неверный ответ: %+v", created)
	}

	rec = do(http.MethodGet, "/api/transactions", "")
	assertJSON(t, rec, http.StatusOK)
	var list []TransactionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, tx := range list {
		if tx.ID == created.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("транзакция %d не найдена в списке", created.ID)
	}
}

func TestBudgetExceededReturns409(t *testing.T) {
	assertJSON(t, do(http.MethodPost, "/api/budgets", `{"category":"gw-limit","limit":100}`), http.StatusCreated)
	assertJSON(t, do(http.MethodPost, "/api/transactions", `{"amount":-60,"category":"gw-limit"}`), http.StatusCreated)

	rec := do(http.MethodPost, "/api/transactions", `{"amount":-60,"category":"gw-limit"}`)
	assertJSON(t, rec, http.StatusConflict)
	if got := errorText(t, rec); got != "budget exceeded" {
		t.Fatalf("ошибка %q, ожидалось \"budget exceeded\"", got)
	}
}

func TestBudgets(t *testing.T) {
	assertJSON(t, do(http.MethodPost, "/api/budgets", `{"category":"gw-b","limit":-1}`), http.StatusBadRequest)
	assertJSON(t, do(http.MethodPost, "/api/budgets", `{"category":"  ","limit":10}`), http.StatusBadRequest)

	rec := do(http.MethodPost, "/api/budgets", `{"category":" gw-b ","limit":250}`)
	assertJSON(t, rec, http.StatusCreated)
	var b BudgetResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatal(err)
	}
	if b.Category != "gw-b" || b.Limit != 250 {
		t.Fatalf("неверный ответ: %+v", b)
	}

	rec = do(http.MethodGet, "/api/budgets", "")
	assertJSON(t, rec, http.StatusOK)
	var list []BudgetResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) == 0 {
		t.Fatal("список бюджетов пуст")
	}
}

func TestUnifiedErrors(t *testing.T) {
	assertJSON(t, do(http.MethodGet, "/api/unknown", ""), http.StatusNotFound)
	assertJSON(t, do(http.MethodDelete, "/api/transactions", ""), http.StatusMethodNotAllowed)
	assertJSON(t, do(http.MethodGet, "/ping", ""), http.StatusOK)
}

func TestEmptyListsAreArrays(t *testing.T) {
	if got := string(mustMarshal(t, NewTransactionResponses(nil))); got != "[]" {
		t.Fatalf("ожидался [], получено %s", got)
	}
	if got := string(mustMarshal(t, NewBudgetResponses(nil))); got != "[]" {
		t.Fatalf("ожидался [], получено %s", got)
	}
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
