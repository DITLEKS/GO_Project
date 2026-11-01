package ledger

import (
 "bytes"
 "database/sql"
 "encoding/json"
 "errors"
 "fmt"
 "io"
 "math"
 "strings"
)

type Budget struct {
 Category string `json:"category"`
 Limit float64 `json:"limit"`
 Period string `json:"period,omitempty"`
}
func (b Budget) Validate() error {
 if strings.TrimSpace(b.Category)=="" { return errors.New("категория бюджета не может быть пустой") }
 if !(b.Limit>0) || math.IsInf(b.Limit,0) || b.Limit>999999999999.99 || math.Round(b.Limit*100)==0 { return fmt.Errorf("лимит бюджета для категории %q должен быть больше нуля, получено %.2f", strings.TrimSpace(b.Category),b.Limit) }
 return nil
}
var ErrBudgetExceeded = errors.New("budget exceeded")

func LoadBudgets(r io.Reader) error {
 if r==nil { return errors.New("источник данных бюджетов не задан (nil reader)") }
 data,err:=io.ReadAll(r); if err!=nil { return fmt.Errorf("ошибка чтения бюджетов: %w",err) }
 if len(bytes.TrimSpace(data))==0 { return errors.New("ошибка парсинга бюджетов: источник пуст") }
 var list []Budget
 if err:=json.Unmarshal(data,&list);err!=nil { return fmt.Errorf("ошибка парсинга JSON бюджетов: %w",err) }
 for i,b:=range list { if err:=SetBudget(b);err!=nil { return fmt.Errorf("некорректный бюджет #%d в JSON: %w",i+1,err) } }
 return nil
}
func SetBudget(b Budget) error {
 b.Category=strings.TrimSpace(b.Category)
 if err:=b.Validate();err!=nil { return err }
 db,err:=dbConn();if err!=nil { return err }
 ctx,cancel:=operationContext();defer cancel()
 tx,err:=db.BeginTx(ctx,nil);if err!=nil { return err };defer tx.Rollback()
 if err:=lockCategory(ctx,tx,b.Category);err!=nil { return err }
 _,err=tx.ExecContext(ctx,`INSERT INTO budgets(category,limit_amount,period) VALUES($1,$2,$3)
        ON CONFLICT(category) DO UPDATE SET limit_amount=EXCLUDED.limit_amount,period=EXCLUDED.period`,b.Category,fmt.Sprintf("%.2f",b.Limit),b.Period)
 if err!=nil { return fmt.Errorf("upsert budget: %w",err) };return tx.Commit()
}
func GetBudget(category string)(Budget,bool,error){
 db,err:=dbConn();if err!=nil { return Budget{},false,err }
 ctx,cancel:=operationContext();defer cancel()
 var b Budget
 err=db.QueryRowContext(ctx,"SELECT category,limit_amount,period FROM budgets WHERE category=$1",strings.TrimSpace(category)).Scan(&b.Category,&b.Limit,&b.Period)
 if errors.Is(err,sql.ErrNoRows) { return Budget{},false,nil };return b,err==nil,err
}
func ListBudgets()([]Budget,error){
 db,err:=dbConn();if err!=nil { return nil,err }
 ctx,cancel:=operationContext();defer cancel()
 rows,err:=db.QueryContext(ctx,"SELECT category,limit_amount,period FROM budgets ORDER BY category")
 if err!=nil { return nil,fmt.Errorf("list budgets: %w",err) };defer rows.Close()
 result:=make([]Budget,0)
 for rows.Next(){var b Budget;if err:=rows.Scan(&b.Category,&b.Limit,&b.Period);err!=nil { return nil,err };result=append(result,b)}
 return result,rows.Err()
}
func SpentByCategory(category string)(float64,error){
 db,err:=dbConn();if err!=nil { return 0,err }
 ctx,cancel:=operationContext();defer cancel()
 var spent float64
 err=db.QueryRowContext(ctx,"SELECT COALESCE(SUM(-amount),0) FROM expenses WHERE category=$1 AND amount<0",category).Scan(&spent)
 return spent,err
}
