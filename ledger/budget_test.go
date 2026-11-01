package ledger

import (
 "errors"
 "io"
 "strings"
 "testing"
)
type errReader struct{}
func(errReader)Read([]byte)(int,error){return 0,errors.New("disk failure")}
func TestSetBudget(t *testing.T){
 resetChecked(t);t.Cleanup(func(){resetChecked(t)})
 for _,b:=range []Budget{{Category:" food ",Limit:5000},{Category:"food",Limit:7000,Period:"month"}}{if err:=SetBudget(b);err!=nil{t.Fatal(err)}}
 b,ok:=getBudget(t,"food")
 if !ok || b.Limit!=7000 || b.Period!="month" || len(listBudgets(t))!=1{t.Fatal(b)}
 if _,ok:=getBudget(t,"missing");ok{t.Fatal("unexpected budget")}
 for _,b:=range []Budget{{Limit:1},{Category:"food",Limit:0}}{if err:=SetBudget(b);err==nil{t.Fatal("invalid budget saved")}}
}
func TestLoadBudgets(t *testing.T){
 resetChecked(t);t.Cleanup(func(){resetChecked(t)})
 if err:=LoadBudgets(strings.NewReader(`[{"category":"food","limit":5000,"period":"month"},{"category":"transport","limit":2000}]`));err!=nil{t.Fatal(err)}
 if len(listBudgets(t))!=2{t.Fatal("expected 2 budgets")}
 for _,r:=range []io.Reader{nil,errReader{},strings.NewReader(" "),strings.NewReader(`{"category":"x"}`),strings.NewReader(`[{"category":"x","limit":-5}]`)}{if err:=LoadBudgets(r);err==nil{t.Fatal("invalid input accepted")}}
}
