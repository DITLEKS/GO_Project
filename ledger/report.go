package ledger

import (
 "context"
 "encoding/json"
 "errors"
 "fmt"
 "log"
 "math"
 "time"
)

type CategorySummary struct { Category string `json:"category"`; Total float64 `json:"total"` }
type ReportSummary struct {
 From string `json:"from"`
 To string `json:"to"`
 Income float64 `json:"income"`
 Expenses float64 `json:"expenses"`
 Balance float64 `json:"balance"`
 Categories []CategorySummary `json:"categories"`
}
// GetReportSummary includes both boundary dates; grouping totals keep their signs.
// Redis failures after initialization are fail-open; PostgreSQL is authoritative.
func GetReportSummary(ctx context.Context,from,to time.Time)(ReportSummary,error){
 fromDate,toDate:=from.Format("2006-01-02"),to.Format("2006-01-02")
 if fromDate>toDate { return ReportSummary{},errors.New("from must be before or equal to to") }
 ctx,cancel:=context.WithTimeout(ctx,5*time.Second);defer cancel()
 key:="";c:=cacheConn()
 if c!=nil {
  version,err:=c.Version(ctx)
  if err==nil {
   key=fmt.Sprintf("report:summary:%s:%s:v%s",fromDate,toDate,version)
   if data,err:=c.Get(ctx,key);err==nil {
    var cached ReportSummary
    if json.Unmarshal(data,&cached)==nil && cached.From==fromDate && cached.To==toDate && cached.Categories!=nil { log.Printf("report cache hit: %s",key);return cached,nil }
   }
  } else { log.Printf("report cache unavailable; querying PostgreSQL: %v",err) }
 }
 db,err:=dbConn();if err!=nil { return ReportSummary{},err }
 result:=ReportSummary{From:fromDate,To:toDate,Categories:make([]CategorySummary,0)}
 // One SQL snapshot for category values and totals.
 rows,err:=db.QueryContext(ctx,`SELECT category,SUM(amount),
        SUM(CASE WHEN amount>0 THEN amount ELSE 0 END),
        SUM(CASE WHEN amount<0 THEN -amount ELSE 0 END)
        FROM expenses WHERE date >= $1::date AND date <= $2::date GROUP BY category ORDER BY category`,fromDate,toDate)
 if err!=nil { return ReportSummary{},fmt.Errorf("report aggregation: %w",err) };defer rows.Close()
 for rows.Next(){
  var category CategorySummary;var income,expenses float64
  if err:=rows.Scan(&category.Category,&category.Total,&income,&expenses);err!=nil { return ReportSummary{},err }
  result.Categories=append(result.Categories,category);result.Income+=income;result.Expenses+=expenses
 }
 if err:=rows.Err();err!=nil { return ReportSummary{},err }
 result.Income=roundMoney(result.Income);result.Expenses=roundMoney(result.Expenses);result.Balance=roundMoney(result.Income-result.Expenses)
 log.Printf("report cache miss: %s",key)
 if c!=nil && key!="" {if data,err:=json.Marshal(result);err==nil {if err:=c.Set(ctx,key,data);err!=nil {log.Printf("report cache set failed: %v",err)}}}
 return result,nil
}
func roundMoney(v float64)float64{return math.Round(v*100)/100}
