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

func loadBudgetsFromFile(ctx context.Context, svc ledger.Service, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return ledger.LoadBudgets(ctx, svc, bufio.NewReader(f))
}

func run() error {
	ctx := context.Background()
	svc, closeFn, err := ledger.NewService(ctx)
	if err != nil {
		return err
	}
	defer closeFn()
	for _, path := range []string{"budgets.json", "missing.json"} {
		if err := loadBudgetsFromFile(ctx, svc, path); err != nil {
			log.Printf("load %s: %v", path, err)
		}
	}
	if err := ledger.LoadBudgets(ctx, svc, strings.NewReader(`{"not":"an array"}`)); err != nil {
		log.Printf("expected I/O example error: %v", err)
	}
	for _, v := range []ledger.Validatable{ledger.Transaction{Amount: -100, Category: "food"}, ledger.Budget{Category: "food", Limit: 5000}, ledger.Transaction{}, ledger.Budget{}} {
		fmt.Printf("%T Validate: %v\n", v, ledger.CheckValid(v))
	}
	if _, err := svc.SetBudget(ctx, ledger.Budget{Category: "food", Limit: 5000}); err != nil {
		return err
	}
	for _, amount := range []float64{-3000, -2500, 10000, 0} {
		_, err := svc.AddTransaction(ctx, ledger.Transaction{Amount: amount, Category: "food", Date: "2026-11-01"})
		if errors.Is(err, ledger.ErrBudgetExceeded) {
			fmt.Println("budget exceeded")
		} else {
			fmt.Printf("amount %.2f: %v\n", amount, err)
		}
	}
	transactions, err := svc.ListTransactions(ctx)
	if err != nil {
		return err
	}
	budgets, err := svc.ListBudgets(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("transactions: %+v\nbudgets: %+v\n", transactions, budgets)
	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
