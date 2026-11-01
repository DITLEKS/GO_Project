package ledger

import (
 "context"
 "errors"
 "net/url"
 "sync"
 "sync/atomic"
 "testing"
 "time"
)
func TestDatabaseDSN(t *testing.T){
 t.Setenv("DATABASE_URL","postgres://explicit/example")
 if got:=DatabaseDSN();got!="postgres://explicit/example"{t.Fatal(got)}
 t.Setenv("DATABASE_URL","");t.Setenv("DB_HOST","localhost");t.Setenv("DB_PORT","5433")
 t.Setenv("DB_USER","name@host");t.Setenv("DB_PASS","p:a/ss?#");t.Setenv("DB_NAME","cashapp_test")
 u,err:=url.Parse(DatabaseDSN());if err!=nil{t.Fatal(err)};pass,_:=u.User.Password()
 if u.User.Username()!="name@host" || pass!="p:a/ss?#" || u.Host!="localhost:5433" || u.Path!="/cashapp_test" || u.Query().Get("sslmode")!="disable"{t.Fatal(u)}
}
func TestConcurrentBudgetLimit(t *testing.T){
 resetChecked(t);t.Cleanup(func(){resetChecked(t)})
 if err:=SetBudget(Budget{Category:"parallel",Limit:100});err!=nil{t.Fatal(err)}
 var accepted atomic.Int32;errs:=make(chan error,20);var wg sync.WaitGroup
 for i:=0;i<20;i++{wg.Add(1);go func(){defer wg.Done();err:=AddTransaction(Transaction{Amount:-10,Category:"parallel",Date:"2026-11-01"});if err==nil{accepted.Add(1)}else if !errors.Is(err,ErrBudgetExceeded){errs<-err}}()}
 wg.Wait();close(errs);for err:=range errs{t.Error(err)}
 if accepted.Load()!=10 || len(listTransactions(t))!=10 || spentByCategory(t,"parallel")!=100{t.Fatalf("accepted %d; budget must not be exceeded",accepted.Load())}
}
func TestPersistenceAfterReconnect(t *testing.T){
 resetChecked(t);t.Cleanup(func(){resetChecked(t)})
 if err:=SetBudget(Budget{Category:"saved",Limit:500,Period:"месяц"});err!=nil{t.Fatal(err)}
 created,err:=CreateTransaction(Transaction{Amount:-123.45,Category:"saved",Date:"2026-11-01"});if err!=nil{t.Fatal(err)}
 if err:=Close();err!=nil{t.Fatal(err)}
 if err:=Initialize(context.Background());err!=nil{t.Fatal(err)}
 rows:=listTransactions(t);if len(rows)!=1 || rows[0]!=created{t.Fatalf("data lost across connection restart: %+v",rows)}
 b,ok:=getBudget(t,"saved");if !ok || b.Limit!=500 || b.Period!="месяц"{t.Fatal(b)}
}
func TestReportSummaryCache(t *testing.T){
 resetChecked(t);t.Cleanup(func(){resetChecked(t)})
 ctx:=context.Background();from,_:=time.Parse("2006-01-02","2026-10-01");to,_:=time.Parse("2006-01-02","2026-10-31")
 for _,tx:=range []Transaction{{Amount:1000,Category:"salary",Date:"2026-10-01"},{Amount:-25.50,Category:"food",Date:"2026-10-31"},{Amount:-999,Category:"food",Date:"2026-11-01"}}{if err:=AddTransaction(tx);err!=nil{t.Fatal(err)}}
 first,err:=GetReportSummary(ctx,from,to);if err!=nil{t.Fatal(err)}
 if first.Income!=1000 || first.Expenses!=25.50 || first.Balance!=974.50 || len(first.Categories)!=2{t.Fatalf("incorrect aggregation: %+v",first)}
 c:=cacheConn();version,err:=c.Version(ctx);if err!=nil{t.Fatal(err)};key:="report:summary:2026-10-01:2026-10-31:v"+version
 if _,err:=c.Get(ctx,key);err!=nil{t.Fatalf("cache entry missing: %v",err)}
 // Mark the cached JSON: the next request must use the cache, not recompute.
 if err:=c.Set(ctx,key,[]byte(`{"from":"2026-10-01","to":"2026-10-31","income":42,"expenses":0,"balance":42,"categories":[]}`));err!=nil{t.Fatal(err)}
 second,err:=GetReportSummary(ctx,from,to);if err!=nil || second.Income!=42{t.Fatalf("cache was not read: %+v %v",second,err)}
 if err:=AddTransaction(Transaction{Amount:-0.50,Category:"food",Date:"2026-10-15"});err!=nil{t.Fatal(err)}
 third,err:=GetReportSummary(ctx,from,to);if err!=nil || third.Expenses!=26 || third.Income!=1000{t.Fatalf("stale cache after write: %+v %v",third,err)}
 version,err=c.Version(ctx);if err!=nil{t.Fatal(err)};key="report:summary:2026-10-01:2026-10-31:v"+version
 // Invalid cached JSON is a miss, never a report error.
 if err:=c.Set(ctx,key,[]byte("broken JSON"));err!=nil{t.Fatal(err)}
 fresh,err:=GetReportSummary(ctx,from,to);if err!=nil || fresh.Expenses!=26{t.Fatal(fresh,err)}
 emptyFrom,_:=time.Parse("2006-01-02","2020-01-01")
 empty,err:=GetReportSummary(ctx,emptyFrom,emptyFrom);if err!=nil || empty.Categories==nil || len(empty.Categories)!=0{t.Fatal(empty,err)}
 if _,err:=GetReportSummary(ctx,to,from);err==nil{t.Fatal("reversed interval accepted")}
}
func TestListTransactionsDateOrder(t *testing.T){
 resetChecked(t);t.Cleanup(func(){resetChecked(t)})
 for _,date:=range []string{"2026-10-03","2026-10-01","2026-10-03"}{if err:=AddTransaction(Transaction{Amount:1,Category:"dates",Date:date});err!=nil{t.Fatal(err)}}
 rows:=listTransactions(t);if rows[0].ID!=3 || rows[1].ID!=1 || rows[2].ID!=2{t.Fatal(rows)}
}
