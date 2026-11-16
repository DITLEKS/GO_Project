package api

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/DITLEKS/GO_Project/ledger"
)

// slowService (see timeout_test.go) must implement the whole ledger.Service.
func (slowService) ImportTransactions(ctx context.Context, _ []ledger.Transaction, _ int) (ledger.ImportResult, error) {
	return ledger.ImportResult{}, waitDone(ctx)
}

// bulkStub records what the handler passed to the service and returns a fixed summary.
type bulkStub struct {
	slowService
	txs     []ledger.Transaction
	workers int
	result  ledger.ImportResult
}

func (s *bulkStub) ImportTransactions(_ context.Context, txs []ledger.Transaction, workers int) (ledger.ImportResult, error) {
	s.txs, s.workers = txs, workers
	return s.result, nil
}

func TestBulkImportHandler(t *testing.T) {
	stub := &bulkStub{result: ledger.ImportResult{Accepted: 2, Rejected: 1, Errors: []ledger.ImportError{{Index: 1, Error: "budget exceeded"}}}}
	h := NewRouterWithTimeout(stub, time.Second)
	body := `[{"amount":-10,"category":" еда ","description":"обед","date":"2026-11-01"},{"amount":-99,"category":"еда"},{"amount":5,"category":"подарок"}]`
	rec := doRequest(h, "POST", "/api/transactions/bulk?workers=2", body)
	// Partial rejection is still 200: the client gets the full report.
	requireJSON(t, rec, http.StatusOK)
	got := decodeBody[ledger.ImportResult](t, rec)
	if got.Accepted != 2 || got.Rejected != 1 || len(got.Errors) != 1 || got.Errors[0] != (ledger.ImportError{Index: 1, Error: "budget exceeded"}) {
		t.Fatalf("unexpected summary: %+v", got)
	}
	if stub.workers != 2 || len(stub.txs) != 3 {
		t.Fatalf("service got workers=%d, %d transactions", stub.workers, len(stub.txs))
	}
	want := ledger.Transaction{Amount: -10, Category: "еда", Description: "обед", Date: "2026-11-01"}
	if stub.txs[0] != want {
		t.Fatalf("got %+v, want %+v", stub.txs[0], want)
	}

	// Without ?workers the service default is used (0 is passed down).
	stub.result = ledger.ImportResult{}
	rec = doRequest(h, "POST", "/api/transactions/bulk", `[]`)
	requireJSON(t, rec, http.StatusOK)
	if stub.workers != 0 || len(stub.txs) != 0 {
		t.Fatalf("service got workers=%d, %d transactions", stub.workers, len(stub.txs))
	}
	if !strings.Contains(rec.Body.String(), `"errors":[]`) {
		t.Fatalf("errors must be an empty array: %s", rec.Body.String())
	}
}

func TestBulkImportBadRequests(t *testing.T) {
	stub := &bulkStub{}
	h := NewRouterWithTimeout(stub, time.Second)
	for _, tc := range []struct{ name, path, body string }{
		{"not_array", "/api/transactions/bulk", `{"amount":1,"category":"x"}`},
		{"null", "/api/transactions/bulk", `null`},
		{"empty_body", "/api/transactions/bulk", ``},
		{"bad_json", "/api/transactions/bulk", `[`},
		{"unknown_field", "/api/transactions/bulk", `[{"amount":1,"category":"x","extra":1}]`},
		{"trailing", "/api/transactions/bulk", `[] []`},
		{"workers_zero", "/api/transactions/bulk?workers=0", `[]`},
		{"workers_text", "/api/transactions/bulk?workers=abc", `[]`},
		{"workers_too_many", "/api/transactions/bulk?workers=999", `[]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requireError(t, doRequest(h, "POST", tc.path, tc.body), http.StatusBadRequest, "")
		})
	}
	requireError(t, doRequest(h, "GET", "/api/transactions/bulk", ""), http.StatusMethodNotAllowed, "method not allowed")
}

func TestBulkImportTimeout(t *testing.T) {
	h := NewRouterWithTimeout(slowService{}, 20*time.Millisecond)
	rec := doRequest(h, "POST", "/api/transactions/bulk", `[{"amount":-1,"category":"еда"}]`)
	requireError(t, rec, http.StatusGatewayTimeout, timeoutMessage)
}

// Real PostgreSQL and Redis: the worker pool runs under -race in CI.
func TestBulkImportHTTP(t *testing.T) {
	h := newRouter(t)
	requireJSON(t, doRequest(h, "POST", "/api/budgets", `{"category":"еда","limit":100}`), http.StatusCreated)
	body := `[
		{"amount":-60,"category":"еда","date":"2026-11-01"},
		{"amount":-60,"category":"еда","date":"2026-11-02"},
		{"amount":0,"category":"еда"},
		{"amount":10,"category":"еда","date":"2026-11-03"},
		{"amount":-5,"category":"такси","date":"2026-11-03"}
	]`
	rec := doRequest(h, "POST", "/api/transactions/bulk?workers=4", body)
	requireJSON(t, rec, http.StatusOK)
	got := decodeBody[ledger.ImportResult](t, rec)
	// Only one of the two expenses of 60 fits the limit of 100.
	if got.Accepted != 3 || got.Rejected != 2 || len(got.Errors) != 2 {
		t.Fatalf("unexpected summary: %+v", got)
	}
	first, second := got.Errors[0], got.Errors[1]
	if (first.Index != 0 && first.Index != 1) || first.Error != "budget exceeded" {
		t.Fatalf("expected a budget error for index 0 or 1: %+v", first)
	}
	if second.Index != 2 || second.Error == "" || second.Error == "budget exceeded" {
		t.Fatalf("expected a validation error for index 2: %+v", second)
	}
	list := decodeBody[[]TransactionResponse](t, doRequest(h, "GET", "/api/transactions", ""))
	if len(list) != 3 {
		t.Fatalf("saved %d transactions, expected 3", len(list))
	}
}
