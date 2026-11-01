package api

import (
 "encoding/json"
 "errors"
 "io"
 "log"
 "net/http"
 "time"
 "github.com/DITLEKS/GO_Project/ledger"
)
const (
 contentTypeJSON = "application/json; charset=utf-8"
 maxBodyBytes = 1 << 20
)
// Concurrency protection is implemented in PostgreSQL, not a process-local mutex.
type Handler struct{}
func NewHandler()*Handler{return &Handler{}}
func writeJSON(w http.ResponseWriter,status int,v any){
 w.Header().Set("Content-Type",contentTypeJSON);w.WriteHeader(status)
 if err:=json.NewEncoder(w).Encode(v);err!=nil{log.Printf("ошибка записи ответа: %v",err)}
}
func writeError(w http.ResponseWriter,status int,msg string){writeJSON(w,status,ErrorResponse{Error:msg})}
func methodNotAllowed(w http.ResponseWriter,allow string){w.Header().Set("Allow",allow);writeError(w,http.StatusMethodNotAllowed,"method not allowed")}
func notFound(w http.ResponseWriter,_ *http.Request){writeError(w,http.StatusNotFound,"not found")}
func ping(w http.ResponseWriter,r *http.Request){
 if r.Method!=http.MethodGet{methodNotAllowed(w,http.MethodGet);return}
 writeJSON(w,http.StatusOK,map[string]string{"message":"pong"})
}
func decodeJSON(w http.ResponseWriter,r *http.Request,dst any)error{
 r.Body=http.MaxBytesReader(w,r.Body,maxBodyBytes)
 dec:=json.NewDecoder(r.Body);dec.DisallowUnknownFields()
 if err:=dec.Decode(dst);err!=nil{return err}
 if err:=dec.Decode(&struct{}{});!errors.Is(err,io.EOF){return errors.New("после JSON-объекта есть лишние данные")}
 return nil
}
func(h *Handler)Transactions(w http.ResponseWriter,r *http.Request){
 switch r.Method{case http.MethodGet:h.listTransactions(w);case http.MethodPost:h.createTransaction(w,r);default:methodNotAllowed(w,"GET, POST")}
}
func(h *Handler)Budgets(w http.ResponseWriter,r *http.Request){
 switch r.Method{case http.MethodGet:h.listBudgets(w);case http.MethodPost:h.createBudget(w,r);default:methodNotAllowed(w,"GET, POST")}
}
func(h *Handler)createTransaction(w http.ResponseWriter,r *http.Request){
 var req CreateTransactionRequest
 if err:=decodeJSON(w,r,&req);err!=nil{writeError(w,400,"некорректный JSON: "+err.Error());return}
 tx:=req.ToDomain()
 if err:=tx.Validate();err!=nil{writeError(w,400,err.Error());return}
 created,err:=ledger.CreateTransaction(tx)
 if err!=nil{
  if errors.Is(err,ledger.ErrBudgetExceeded){writeError(w,409,"budget exceeded");return}
  log.Printf("неожиданная ошибка AddTransaction: %v",err);writeError(w,500,"internal server error");return
 }
 writeJSON(w,201,NewTransactionResponse(created))
}
func(h *Handler)listTransactions(w http.ResponseWriter){
 list,err:=ledger.ListTransactions()
 if err!=nil{log.Printf("ListTransactions: %v",err);writeError(w,500,"internal server error");return}
 writeJSON(w,200,NewTransactionResponses(list))
}
func(h *Handler)createBudget(w http.ResponseWriter,r *http.Request){
 var req CreateBudgetRequest
 if err:=decodeJSON(w,r,&req);err!=nil{writeError(w,400,"некорректный JSON: "+err.Error());return}
 b:=req.ToDomain()
 if err:=b.Validate();err!=nil{writeError(w,400,err.Error());return}
 if err:=ledger.SetBudget(b);err!=nil{log.Printf("неожиданная ошибка SetBudget: %v",err);writeError(w,500,"internal server error");return}
 writeJSON(w,201,NewBudgetResponse(b))
}
func(h *Handler)listBudgets(w http.ResponseWriter){
 list,err:=ledger.ListBudgets()
 if err!=nil{log.Printf("ListBudgets: %v",err);writeError(w,500,"internal server error");return}
 writeJSON(w,200,NewBudgetResponses(list))
}
// Additive endpoint; existing routes and DTOs are unchanged.
func(h *Handler)ReportSummary(w http.ResponseWriter,r *http.Request){
 if r.Method!=http.MethodGet{methodNotAllowed(w,http.MethodGet);return}
 from,errFrom:=time.Parse("2006-01-02",r.URL.Query().Get("from"))
 to,errTo:=time.Parse("2006-01-02",r.URL.Query().Get("to"))
 if errFrom!=nil || errTo!=nil || from.After(to){writeError(w,400,"from/to must be YYYY-MM-DD and from <= to");return}
 report,err:=ledger.GetReportSummary(r.Context(),from,to)
 if err!=nil{log.Printf("GetReportSummary: %v",err);writeError(w,500,"internal server error");return}
 writeJSON(w,200,report)
}
