package api

import (
 "bytes"
 "context"
 "encoding/json"
 "fmt"
 "io"
 "log"
 "net/http"
 "net/http/httptest"
 "os"
 "strings"
 "testing"
 "github.com/DITLEKS/GO_Project/ledger"
)
func TestMain(m *testing.M){
 log.SetOutput(io.Discard)
 if dsn:=os.Getenv("TEST_DATABASE_URL");dsn!=""{
  os.Setenv("DATABASE_URL",dsn)
  if err:=ledger.Initialize(context.Background());err!=nil{fmt.Fprintln(os.Stderr,err);os.Exit(1)}
  code:=m.Run();_ = ledger.Close();os.Exit(code)
 }
 os.Exit(m.Run())
}
func newRouter(t *testing.T)http.Handler{
 t.Helper();if os.Getenv("TEST_DATABASE_URL")==""{t.Skip("set TEST_DATABASE_URL and apply migrations; Redis required")}
 if err:=ledger.Reset();err!=nil{t.Fatal(err)}
 t.Cleanup(func(){if err:=ledger.Reset();err!=nil{t.Error(err)}});return NewRouter()
}
func doRequest(h http.Handler,method,path,body string)*httptest.ResponseRecorder{
 req:=httptest.NewRequest(method,path,strings.NewReader(body));req.Header.Set("Content-Type","application/json")
 rec:=httptest.NewRecorder();h.ServeHTTP(rec,req);return rec
}
func requireJSON(t *testing.T,rec *httptest.ResponseRecorder,status int){
 t.Helper();if rec.Code!=status || rec.Header().Get("Content-Type")!=contentTypeJSON{t.Fatalf("status/header %d %v; body %s",rec.Code,rec.Header(),rec.Body.String())}
}
func decodeBody[T any](t *testing.T,rec *httptest.ResponseRecorder)T{
 t.Helper();var v T;if err:=json.Unmarshal(rec.Body.Bytes(),&v);err!=nil{t.Fatal(err)};return v
}
func requireError(t *testing.T,rec *httptest.ResponseRecorder,status int,message string){
 t.Helper();requireJSON(t,rec,status);e:=decodeBody[ErrorResponse](t,rec)
 if e.Error=="" || (message!="" && e.Error!=message){t.Fatalf("error %q, expected %q",e.Error,message)}
}
func TestBudgetsHTTP(t *testing.T){
 h:=newRouter(t)
 rec:=doRequest(h,"GET","/api/budgets","");requireJSON(t,rec,200)
 if strings.TrimSpace(rec.Body.String())!="[]"{t.Fatal(rec.Body.String())}
 for _,body:=range []string{`{"category":"еда","limit":5000}`,`{"category":" еда ","limit":7000,"period":"месяц"}`} {
  rec=doRequest(h,"POST","/api/budgets",body);requireJSON(t,rec,201)
  if b:=decodeBody[BudgetResponse](t,rec);b.Category!="еда" || b.Limit<=0{t.Fatal(b)}
 }
 rec=doRequest(h,"GET","/api/budgets","");requireJSON(t,rec,200);list:=decodeBody[[]BudgetResponse](t,rec)
 if len(list)!=1 || list[0].Limit!=7000 || list[0].Period!="месяц"{t.Fatal(list)}
 for _,tc:=range []struct{name,body string}{
  {"bad_json","{"},{"empty",""},{"zero",`{"category":"еда","limit":0}`},
  {"negative",`{"category":"еда","limit":-1}`},{"category",`{"limit":10}`},
  {"unknown",`{"category":"еда","limit":10,"extra":1}`},{"trailing",`{} {}`},
 }{t.Run(tc.name,func(t *testing.T){requireError(t,doRequest(h,"POST","/api/budgets",tc.body),400,"")})}
 if len(decodeBody[[]BudgetResponse](t,doRequest(h,"GET","/api/budgets","")))!=1{t.Fatal("invalid budgets saved")}
}
func TestTransactionsHTTP(t *testing.T){
 h:=newRouter(t)
 rec:=doRequest(h,"GET","/api/transactions","");requireJSON(t,rec,200)
 if strings.TrimSpace(rec.Body.String())!="[]"{t.Fatal(rec.Body.String())}
 requireJSON(t,doRequest(h,"POST","/api/budgets",`{"category":"еда","limit":5000}`),201)
 t.Run("ok",func(t *testing.T){
  rec:=doRequest(h,"POST","/api/transactions",`{"amount":-3000,"category":"еда","description":"ланч","date":"2026-11-01"}`);requireJSON(t,rec,201)
  created:=decodeBody[TransactionResponse](t,rec)
  if created.ID<=0 || created.Amount!=-3000 || created.Date!="2026-11-01" || created.Description!="ланч"{t.Fatal(created)}
  rec=doRequest(h,"GET","/api/transactions","");requireJSON(t,rec,200)
  list:=decodeBody[[]TransactionResponse](t,rec);if len(list)!=1 || list[0]!=created{t.Fatal(list)}
 })
 t.Run("exceeded",func(t *testing.T){requireError(t,doRequest(h,"POST","/api/transactions",`{"amount":-2500,"category":"еда","date":"2026-11-02"}`),409,"budget exceeded")})
 for _,tc:=range []struct{name,body string}{
  {"bad_json","{"},{"zero",`{"amount":0,"category":"еда"}`},
  {"category",`{"amount":1}`},{"date",`{"amount":1,"category":"еда","date":"bad"}`},
  {"unknown",`{"amount":1,"category":"еда","x":1}`},{"trailing",`{} {}`},
 }{t.Run(tc.name,func(t *testing.T){requireError(t,doRequest(h,"POST","/api/transactions",tc.body),400,"")})}
 if len(decodeBody[[]TransactionResponse](t,doRequest(h,"GET","/api/transactions","")))!=1{t.Fatal("rejected transaction saved")}
}
func TestRoutingErrors(t *testing.T){
 t.Parallel();h:=NewRouter()
 for _,tc:=range []struct{method,path string;status int;message string}{
  {"GET","/unknown",404,"not found"},{"GET","/api/nope",404,"not found"},
  {"DELETE","/api/transactions",405,"method not allowed"},{"PUT","/api/budgets",405,"method not allowed"},
  {"POST","/ping",405,"method not allowed"},
 }{tc:=tc;t.Run(tc.method+tc.path,func(t *testing.T){t.Parallel();requireError(t,doRequest(h,tc.method,tc.path,""),tc.status,tc.message)})}
 requireJSON(t,doRequest(h,"GET","/ping",""),200)
}
func TestLoggingMiddleware(t *testing.T){
 var buf bytes.Buffer;log.SetOutput(&buf);t.Cleanup(func(){log.SetOutput(io.Discard)})
 doRequest(NewRouter(),"GET","/ping","")
 if !strings.Contains(buf.String(),"GET /ping"){t.Fatal(buf.String())}
}
