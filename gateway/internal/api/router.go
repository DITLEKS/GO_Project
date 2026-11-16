package api

import (
	"net/http"
	"time"

	"github.com/DITLEKS/GO_Project/ledger"
)

// NewRouter builds the HTTP routes on top of the given ledger service.
// The request timeout comes from GATEWAY_REQUEST_TIMEOUT (default 2s).
func NewRouter(svc ledger.Service) http.Handler {
	return NewRouterWithTimeout(svc, RequestTimeout())
}

// NewRouterWithTimeout is NewRouter with an explicit request timeout.
// The Timeout middleware wraps every /api/ route.
func NewRouterWithTimeout(svc ledger.Service, timeout time.Duration) http.Handler {
	h := NewHandler(svc)
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/transactions", h.Transactions)
	apiMux.HandleFunc("/budgets", h.Budgets)
	apiMux.HandleFunc("/reports/summary", h.ReportSummary)
	apiMux.HandleFunc("/", notFound)
	mux := http.NewServeMux()
	mux.Handle("/api/", http.StripPrefix("/api", Timeout(timeout)(apiMux)))
	mux.HandleFunc("/ping", ping)
	mux.HandleFunc("/", notFound)
	return Chain(mux, Logging, JSONContentType)
}
