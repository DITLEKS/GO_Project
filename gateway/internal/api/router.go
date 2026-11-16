package api

import (
	"net/http"

	"github.com/DITLEKS/GO_Project/ledger"
)

// NewRouter builds the HTTP routes on top of the given ledger service.
func NewRouter(svc ledger.Service) http.Handler {
	h := NewHandler(svc)
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/transactions", h.Transactions)
	apiMux.HandleFunc("/budgets", h.Budgets)
	apiMux.HandleFunc("/reports/summary", h.ReportSummary)
	apiMux.HandleFunc("/", notFound)
	mux := http.NewServeMux()
	mux.Handle("/api/", http.StripPrefix("/api", apiMux))
	mux.HandleFunc("/ping", ping)
	mux.HandleFunc("/", notFound)
	return Chain(mux, Logging, JSONContentType)
}
