package ledger

import (
 "errors"
 "math"
 "strings"
 "testing"
)
func TestTransactionValidateTable(t *testing.T){
 t.Parallel()
 cases:=[]struct{name string;tx Transaction;contains string}{
  {"income",Transaction{Amount:100,Category:"salary",Date:"2026-11-01"},""},
  {"expense",Transaction{Amount:-50,Category:"food"},""},
  {"cent",Transaction{Amount:0.01,Category:"food"},""},
  {"zero",Transaction{Category:"food"},"сумма"},
  {"nan",Transaction{Amount:math.NaN(),Category:"food"},"сумма"},
  {"inf",Transaction{Amount:math.Inf(1),Category:"food"},"сумма"},
  {"negative_inf",Transaction{Amount:math.Inf(-1),Category:"food"},"сумма"},
  {"category",Transaction{Amount:1},"категория"},
  {"blank",Transaction{Amount:1,Category:"  "},"категория"},
  {"date_format",Transaction{Amount:1,Category:"food",Date:"01.11.2026"},"дата"},
  {"date_value",Transaction{Amount:1,Category:"food",Date:"2026-13-40"},"дата"},
  {"too_small",Transaction{Amount:0.001,Category:"food"},"сумма"},
  {"too_large",Transaction{Amount:1e12,Category:"food"},"сумма"},
 }
 for _,tc:=range cases{tc:=tc;t.Run(tc.name,func(t *testing.T){t.Parallel();checkValidation(t,tc.tx.Validate(),tc.contains)})}
}
func TestBudgetValidateTable(t *testing.T){
 t.Parallel()
 cases:=[]struct{name string;b Budget;contains string}{
  {"positive",Budget{Category:"food",Limit:5000},""},
  {"period",Budget{Category:"food",Limit:1,Period:"month"},""},
  {"cent",Budget{Category:"food",Limit:0.01},""},
  {"zero",Budget{Category:"food"},"лимит"},
  {"negative",Budget{Category:"food",Limit:-1},"лимит"},
  {"nan",Budget{Category:"food",Limit:math.NaN()},"лимит"},
  {"inf",Budget{Category:"food",Limit:math.Inf(1)},"лимит"},
  {"blank",Budget{Category:"  ",Limit:1},"категория"},
  {"empty",Budget{Limit:1},"категория"},
  {"too_small",Budget{Category:"food",Limit:0.001},"лимит"},
  {"too_large",Budget{Category:"food",Limit:1e12},"лимит"},
 }
 for _,tc:=range cases{tc:=tc;t.Run(tc.name,func(t *testing.T){t.Parallel();checkValidation(t,tc.b.Validate(),tc.contains)})}
}
func checkValidation(t *testing.T,err error,contains string){
 t.Helper();if contains==""{if err!=nil{t.Fatal(err)};return}
 if err==nil || !strings.Contains(err.Error(),contains){t.Fatalf("got %v, expected error containing %q",err,contains)}
}
func TestBudgetExceededFlow(t *testing.T){
 resetChecked(t);t.Cleanup(func(){resetChecked(t)})
 if err:=SetBudget(Budget{Category:"еда",Limit:5000});err!=nil{t.Fatal(err)}
 for _,tc:=range []struct{name string;amount float64;category string;exceeded bool}{
  {"within",-3000,"еда",false},{"exceeded",-2500,"еда",true},
  {"exact",-2000,"еда",false},{"cent_over",-0.01,"еда",true},
  {"income",10000,"еда",false},{"no_budget",-99999,"такси",false},
 }{t.Run(tc.name,func(t *testing.T){
  before:=len(listTransactions(t));err:=AddTransaction(Transaction{Amount:tc.amount,Category:tc.category})
  if tc.exceeded{if !errors.Is(err,ErrBudgetExceeded) || len(listTransactions(t))!=before{t.Fatalf("rejected transaction saved: %v",err)}}else if err!=nil || len(listTransactions(t))!=before+1{t.Fatal(err)}
 })}
 if got:=spentByCategory(t,"еда");got!=5000{t.Fatal(got)}
}
func TestReset(t *testing.T){
 resetChecked(t);t.Cleanup(func(){resetChecked(t)})
 if err:=SetBudget(Budget{Category:"x",Limit:100});err!=nil{t.Fatal(err)}
 if err:=AddTransaction(Transaction{Amount:1,Category:"x"});err!=nil{t.Fatal(err)}
 resetChecked(t)
 if len(listBudgets(t))!=0 || len(listTransactions(t))!=0{t.Fatal("Reset did not clear state")}
 tx,err:=CreateTransaction(Transaction{Amount:1,Category:"x"});if err!=nil || tx.ID!=1{t.Fatal(tx,err)}
}
