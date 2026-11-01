package ledger

import "testing"

func TestAddAndListTransactions(t *testing.T){
 resetChecked(t);t.Cleanup(func(){resetChecked(t)})
 for _,amount:=range []float64{100,-50}{if err:=AddTransaction(Transaction{Amount:amount,Category:"test"});err!=nil{t.Fatal(err)}}
 if err:=AddTransaction(Transaction{Category:"test"});err==nil{t.Fatal("zero accepted")}
 list:=listTransactions(t)
 if len(list)!=2 || list[0].ID!=2 || list[1].ID!=1{t.Fatal(list)}
}
