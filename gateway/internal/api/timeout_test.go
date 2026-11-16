package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/DITLEKS/GO_Project/ledger"
)

// slowService never finishes by itself: every method waits until the request
// context is cancelled and returns its error. No database is needed.
type slowService struct{}

func waitDone(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

func (slowService) SetBudget(ctx context.Context, _ ledger.Budget) (ledger.Budget, error) {
	return ledger.Budget{}, waitDone(ctx)
}

func (slowService) ListBudgets(ctx context.Context) ([]ledger.Budget, error) {
	return nil, waitDone(ctx)
}

func (slowService) AddTransaction(ctx context.Context, _ ledger.Transaction) (ledger.Transaction, error) {
	return ledger.Transaction{}, waitDone(ctx)
}

func (slowService) ListTransactions(ctx context.Context) ([]ledger.Transaction, error) {
	return nil, waitDone(ctx)
}

func (slowService) ReportSummary(ctx context.Context, _, _ time.Time) (ledger.ReportSummary, error) {
	return ledger.ReportSummary{}, waitDone(ctx)
}

func TestRequestTimeoutReturns504(t *testing.T) {
	h := NewRouterWithTimeout(slowService{}, 20*time.Millisecond)
	for _, tc := range []struct{ method, path, body string }{
		{"GET", "/api/reports/summary?from=2026-11-01&to=2026-11-30", ""},
		{"GET", "/api/transactions", ""},
		{"GET", "/api/budgets", ""},
		{"POST", "/api/budgets", `{"category":"еда","limit":10}`},
		{"POST", "/api/transactions", `{"amount":-1,"category":"еда"}`},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			start := time.Now()
			rec := doRequest(h, tc.method, tc.path, tc.body)
			requireError(t, rec, http.StatusGatewayTimeout, timeoutMessage)
			if elapsed := time.Since(start); elapsed > 2*time.Second {
				t.Fatalf("handler was not interrupted: took %s", elapsed)
			}
		})
	}
}

func TestTimeoutDoesNotChangeOtherErrors(t *testing.T) {
	h := NewRouterWithTimeout(slowService{}, 20*time.Millisecond)
	// Invalid dates are rejected before the service is called.
	requireError(t, doRequest(h, "GET", "/api/reports/summary?from=bad&to=2026-11-30", ""), http.StatusBadRequest, reportRangeMessage)
	requireError(t, doRequest(h, "GET", "/api/nope", ""), http.StatusNotFound, "not found")
	requireJSON(t, doRequest(h, "GET", "/ping", ""), http.StatusOK)
}

func TestRequestTimeoutFromEnv(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  time.Duration
	}{
		{"", DefaultRequestTimeout},
		{"50ms", 50 * time.Millisecond},
		{"3s", 3 * time.Second},
		{"abc", DefaultRequestTimeout},
		{"0s", DefaultRequestTimeout},
		{"-1s", DefaultRequestTimeout},
	} {
		t.Setenv(requestTimeoutEnv, tc.value)
		if got := RequestTimeout(); got != tc.want {
			t.Fatalf("%s=%q: got %s, want %s", requestTimeoutEnv, tc.value, got, tc.want)
		}
	}
}
