// Checks infrastructure; Gateway embeds ledger and builds its own service (no RPC yet).
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/DITLEKS/GO_Project/ledger"
)

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	_, closeFn, err := ledger.NewService(ctx)
	if err != nil {
		return err
	}
	defer closeFn()
	log.Print("Ledger ready; Gateway imports ledger directly and builds its own service")
	<-ctx.Done()
	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("Ledger startup failed: %v", err)
	}
}
