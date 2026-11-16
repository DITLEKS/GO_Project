package service

import (
	"context"
	"errors"
	"log"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/DITLEKS/GO_Project/ledger/internal/domain"
)

const (
	// DefaultImportWorkers is used when the caller passes workers <= 0.
	DefaultImportWorkers = 4
	// MaxImportWorkers keeps the pool below the PostgreSQL connection limit (10).
	MaxImportWorkers = 8
)

// importJob is one input element with its position in the input array.
type importJob struct {
	index int
	tx    domain.Transaction
}

// importOutcome is the result of one job; message is empty on success.
type importOutcome struct {
	index   int
	message string
}

// ImportTransactions imports a batch with a worker pool:
//
//	producer -> jobs -> N workers -> results -> collector
//
// Every element goes through AddTransaction, so validation and the budget rule
// are the same as for a single transaction. jobs is closed after all elements
// are sent, results is closed after all workers have finished, and the collector
// exits when results is closed.
//
// When ctx is cancelled the function stops early and returns the partial
// summary together with the context error.
func (s *Ledger) ImportTransactions(ctx context.Context, txs []domain.Transaction, workers int) (domain.ImportResult, error) {
	result := domain.ImportResult{Errors: make([]domain.ImportError, 0)}
	if len(txs) == 0 {
		return result, ctx.Err()
	}
	if workers <= 0 {
		workers = DefaultImportWorkers
	}
	if workers > MaxImportWorkers {
		workers = MaxImportWorkers
	}
	if workers > len(txs) {
		workers = len(txs)
	}

	jobs := make(chan importJob)
	results := make(chan importOutcome)
	// Shared counters are updated by several workers, so they are atomic.
	var accepted, rejected atomic.Int64

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.importWorker(ctx, jobs, results, &accepted, &rejected)
		}()
	}

	// The collector is the only goroutine that touches the error list.
	collected := make(chan []domain.ImportError, 1)
	go func() {
		list := make([]domain.ImportError, 0)
		for outcome := range results {
			if outcome.message != "" {
				list = append(list, domain.ImportError{Index: outcome.index, Error: outcome.message})
			}
		}
		sort.Slice(list, func(a, b int) bool { return list[a].Index < list[b].Index })
		collected <- list
	}()

	// Producer: send the elements one by one, stop when ctx is cancelled.
feed:
	for i, tx := range txs {
		select {
		case <-ctx.Done():
			break feed
		case jobs <- importJob{index: i, tx: tx}:
		}
	}
	close(jobs)

	wg.Wait()
	close(results)

	result.Errors = <-collected
	result.Accepted = int(accepted.Load())
	result.Rejected = int(rejected.Load())
	if err := ctx.Err(); err != nil {
		log.Printf("import: interrupted after %d accepted, %d rejected of %d: %v", result.Accepted, result.Rejected, len(txs), err)
		return result, err
	}
	return result, nil
}

// importWorker reads jobs until the channel is closed or ctx is cancelled.
func (s *Ledger) importWorker(ctx context.Context, jobs <-chan importJob, results chan<- importOutcome, accepted, rejected *atomic.Int64) {
	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-jobs:
			if !ok {
				return
			}
			_, err := s.AddTransaction(ctx, job.tx)
			if err != nil && ctx.Err() != nil {
				// Interrupted, not rejected by a rule: the element is not counted.
				return
			}
			if err != nil {
				rejected.Add(1)
				results <- importOutcome{index: job.index, message: importErrorMessage(err)}
				continue
			}
			accepted.Add(1)
			results <- importOutcome{index: job.index}
		}
	}
}

// importErrorMessage returns a message that is safe to show to the client.
func importErrorMessage(err error) string {
	var invalid *domain.ValidationError
	switch {
	case errors.As(err, &invalid):
		return invalid.Error()
	case errors.Is(err, domain.ErrBudgetExceeded):
		return "budget exceeded"
	default:
		log.Printf("import: transaction failed: %v", err)
		return "internal error"
	}
}
