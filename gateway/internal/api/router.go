package api

import "net/http"

// NewRouter собирает маршруты с префиксом /api и подключает middleware ко всем маршрутам.
func NewRouter() http.Handler {
	h := NewHandler()

	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/transactions", h.Transactions)
	apiMux.HandleFunc("/budgets", h.Budgets)
	apiMux.HandleFunc("/", notFound)

	mux := http.NewServeMux()
	mux.Handle("/api/", http.StripPrefix("/api", apiMux))
	mux.HandleFunc("/ping", ping)
	mux.HandleFunc("/", notFound)

	return Chain(mux, Logging, JSONContentType)
}
