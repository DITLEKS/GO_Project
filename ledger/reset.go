package ledger

// Reset deletes ALL ledger data and resets IDs. Never call at startup.
// Tests must use a dedicated TEST_DATABASE_URL, not production data.
func Reset() error {
 db,err:=dbConn();if err!=nil { return err }
 ctx,cancel:=operationContext();defer cancel()
 if _,err:=db.ExecContext(ctx,"TRUNCATE expenses, budgets RESTART IDENTITY");err!=nil { return err }
 invalidateReports(ctx)
 return nil
}
