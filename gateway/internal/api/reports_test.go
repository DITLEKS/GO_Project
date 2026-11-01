package api

import (
 "net/http"
 "testing"
 "github.com/DITLEKS/GO_Project/ledger"
)
func TestReportSummaryHTTP(t *testing.T){
 h:=newRouter(t)
 requireJSON(t,doRequest(h,http.MethodPost,"/api/budgets",`{"category":"еда","limit":1000}`),http.StatusCreated)
 requireJSON(t,doRequest(h,http.MethodPost,"/api/transactions",`{"amount":-125.50,"category":"еда","date":"2026-11-01"}`),http.StatusCreated)
 for _,name:=range []string{"cache_miss","cache_hit"}{t.Run(name,func(t *testing.T){
  rec:=doRequest(h,http.MethodGet,"/api/reports/summary?from=2026-11-01&to=2026-11-30","");requireJSON(t,rec,http.StatusOK)
  report:=decodeBody[ledger.ReportSummary](t,rec)
  if report.Expenses!=125.50 || report.Balance!=-125.50 || len(report.Categories)!=1{t.Fatalf("unexpected report: %+v",report)}
 })}
 for _,query:=range []string{"","?from=no&to=2026-11-30","?from=2026-12-01&to=2026-11-01"}{requireError(t,doRequest(h,http.MethodGet,"/api/reports/summary"+query,""),http.StatusBadRequest,"")}
 requireError(t,doRequest(h,http.MethodPost,"/api/reports/summary",""),http.StatusMethodNotAllowed,"method not allowed")
}
