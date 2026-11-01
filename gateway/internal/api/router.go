package api

import "net/http"

func NewRouter() http.Handler {
 h:=NewHandler()
 apiMux:=http.NewServeMux()
 apiMux.HandleFunc("/transactions",h.Transactions)
 apiMux.HandleFunc("/budgets",h.Budgets)
 apiMux.HandleFunc("/reports/summary",h.ReportSummary)
 apiMux.HandleFunc("/",notFound)
 mux:=http.NewServeMux()
 mux.Handle("/api/",http.StripPrefix("/api",apiMux))
 mux.HandleFunc("/ping",ping)
 mux.HandleFunc("/",notFound)
 return Chain(mux,Logging,JSONContentType)
}
