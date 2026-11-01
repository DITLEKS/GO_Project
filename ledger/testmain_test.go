package ledger

import (
 "context"
 "fmt"
 "os"
 "testing"
)
// Destructive integration tests opt in through a separate test DSN.
func TestMain(m *testing.M){
 if dsn:=os.Getenv("TEST_DATABASE_URL");dsn!=""{
  os.Setenv("DATABASE_URL",dsn)
  if err:=Initialize(context.Background());err!=nil{fmt.Fprintln(os.Stderr,err);os.Exit(1)}
  code:=m.Run();_ = Close();os.Exit(code)
 }
 os.Exit(m.Run())
}
func resetChecked(t *testing.T){t.Helper();if os.Getenv("TEST_DATABASE_URL")==""{t.Skip("PostgreSQL integration: set TEST_DATABASE_URL and apply migrations; Redis required")};if err:=Reset();err!=nil{t.Fatal(err)}}
func listTransactions(t *testing.T)[]Transaction{t.Helper();v,err:=ListTransactions();if err!=nil{t.Fatal(err)};return v}
func listBudgets(t *testing.T)[]Budget{t.Helper();v,err:=ListBudgets();if err!=nil{t.Fatal(err)};return v}
func getBudget(t *testing.T,category string)(Budget,bool){t.Helper();b,ok,err:=GetBudget(category);if err!=nil{t.Fatal(err)};return b,ok}
func spentByCategory(t *testing.T,category string)float64{t.Helper();v,err:=SpentByCategory(category);if err!=nil{t.Fatal(err)};return v}
