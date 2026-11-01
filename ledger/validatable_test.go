package ledger

import "testing"

func TestCheckValidPolymorphism(t *testing.T){
 t.Parallel()
 for _,v:=range []Validatable{Transaction{Amount:1,Category:"A"},Budget{Category:"A",Limit:1}}{if err:=CheckValid(v);err!=nil{t.Errorf("%T: %v",v,err)}}
 if CheckValid(Transaction{})==nil || CheckValid(Budget{})==nil{t.Error("expected invalid empty structures")}
}
func TestInvalidNotStored(t *testing.T){
 resetChecked(t);t.Cleanup(func(){resetChecked(t)})
 if err:=AddTransaction(Transaction{Amount:5,Category:""});err==nil{t.Fatal("expected validation error")}
 if err:=SetBudget(Budget{Category:"X",Limit:-1});err==nil{t.Fatal("expected validation error")}
 if len(listTransactions(t))!=0 || len(listBudgets(t))!=0{t.Fatal("invalid data saved")}
}
