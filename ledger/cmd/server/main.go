// Checks infrastructure; Gateway embeds ledger and opens its own pool (no RPC yet).
package main

import (
 "context"
 "log"
 "os"
 "os/signal"
 "syscall"
 "github.com/DITLEKS/GO_Project/ledger"
)
func main(){
 ctx,stop:=signal.NotifyContext(context.Background(),os.Interrupt,syscall.SIGTERM);defer stop()
 if err:=ledger.Initialize(ctx);err!=nil {log.Fatalf("Ledger startup failed: %v",err)}
 defer ledger.Close()
 log.Print("Ledger ready; Gateway imports ledger directly and uses its own connection pool")
 <-ctx.Done()
}
