// Demo writes examples to PostgreSQL. Use a disposable database.
package main

import (
 "bufio"
 "context"
 "errors"
 "fmt"
 "log"
 "os"
 "strings"
 "github.com/DITLEKS/GO_Project/ledger"
)
func loadBudgetsFromFile(path string)error{
 f,err:=os.Open(path);if err!=nil{return err};defer f.Close()
 return ledger.LoadBudgets(bufio.NewReader(f))
}
func main(){
 if err:=ledger.Initialize(context.Background());err!=nil{log.Fatal(err)};defer ledger.Close()
 for _,path:=range []string{"budgets.json","missing.json"}{if err:=loadBudgetsFromFile(path);err!=nil{log.Printf("load %s: %v",path,err)}}
 if err:=ledger.LoadBudgets(strings.NewReader(`{"not":"an array"}`));err!=nil{log.Printf("expected I/O example error: %v",err)}
 for _,v:=range []ledger.Validatable{ledger.Transaction{Amount:-100,Category:"food"},ledger.Budget{Category:"food",Limit:5000},ledger.Transaction{},ledger.Budget{}}{fmt.Printf("%T Validate: %v\n",v,ledger.CheckValid(v))}
 if err:=ledger.SetBudget(ledger.Budget{Category:"food",Limit:5000});err!=nil{log.Fatal(err)}
 for _,amount:=range []float64{-3000,-2500,10000,0}{
  err:=ledger.AddTransaction(ledger.Transaction{Amount:amount,Category:"food",Date:"2026-11-01"})
  if errors.Is(err,ledger.ErrBudgetExceeded){fmt.Println("budget exceeded")}else{fmt.Printf("amount %.2f: %v\n",amount,err)}
 }
 transactions,err:=ledger.ListTransactions();if err!=nil{log.Fatal(err)}
 budgets,err:=ledger.ListBudgets();if err!=nil{log.Fatal(err)}
 fmt.Printf("transactions: %+v\nbudgets: %+v\n",transactions,budgets)
}
