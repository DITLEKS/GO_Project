package main

import (
 "context"
 "errors"
 "log"
 "net/http"
 "os"
 "os/signal"
 "syscall"
 "time"
 "github.com/DITLEKS/GO_Project/gateway/internal/api"
 "github.com/DITLEKS/GO_Project/ledger"
)
func run()error{
 ctx,stop:=signal.NotifyContext(context.Background(),os.Interrupt,syscall.SIGTERM);defer stop()
 if err:=ledger.Initialize(ctx);err!=nil{return err};defer ledger.Close()
 srv:=&http.Server{Addr:":8080",Handler:api.NewRouter(),ReadHeaderTimeout:5*time.Second}
 done:=make(chan error,1);go func(){done<-srv.ListenAndServe()}()
 log.Print("Gateway service started on :8080 (PostgreSQL + Redis)")
 select{
 case err:=<-done:if errors.Is(err,http.ErrServerClosed){return nil};return err
 case <-ctx.Done():
  shutdown,cancel:=context.WithTimeout(context.Background(),10*time.Second);defer cancel()
  if err:=srv.Shutdown(shutdown);err!=nil{_ = srv.Close();return err};return nil
 }
}
func main(){if err:=run();err!=nil{log.Fatalf("Gateway startup failed: %v",err)}}
