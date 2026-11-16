package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/DITLEKS/GO_Project/ledger/internal/domain"
)

// heartbeatInterval is how often a running report calculation logs that it is alive.
const heartbeatInterval = 300 * time.Millisecond

// ReportSummary includes both boundary dates; grouping totals keep their signs.
// Cache failures are fail-open; the repository is authoritative.
// ctx is passed to every storage call: when it is cancelled, the calculation stops.
func (s *Ledger) ReportSummary(ctx context.Context, from, to time.Time) (domain.ReportSummary, error) {
	fromDate, toDate := from.Format(domain.DateLayout), to.Format(domain.DateLayout)
	if fromDate > toDate {
		return domain.ReportSummary{}, &domain.ValidationError{Err: errors.New("from must be before or equal to to")}
	}
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()
	key := ""
	if s.cache != nil {
		version, err := s.cache.Version(ctx)
		if err == nil {
			key = fmt.Sprintf("report:summary:%s:%s:v%s", fromDate, toDate, version)
			if data, err := s.cache.Get(ctx, key); err == nil {
				var cached domain.ReportSummary
				if json.Unmarshal(data, &cached) == nil && cached.From == fromDate && cached.To == toDate && cached.Categories != nil {
					log.Printf("report cache hit: %s", key)
					return cached, nil
				}
			}
		} else {
			log.Printf("report cache unavailable; querying storage: %v", err)
		}
	}
	rows, err := s.categoryTotals(ctx, fromDate, toDate)
	if err != nil {
		return domain.ReportSummary{}, err
	}
	result := domain.ReportSummary{From: fromDate, To: toDate, Categories: make([]domain.CategorySummary, 0, len(rows))}
	for _, row := range rows {
		result.Categories = append(result.Categories, domain.CategorySummary{Category: row.Category, Total: row.Total})
		result.Income += row.Income
		result.Expenses += row.Expenses
	}
	result.Income = roundMoney(result.Income)
	result.Expenses = roundMoney(result.Expenses)
	result.Balance = roundMoney(result.Income - result.Expenses)
	log.Printf("report cache miss: %s", key)
	if s.cache != nil && key != "" {
		if data, err := json.Marshal(result); err == nil {
			if err := s.cache.Set(ctx, key, data); err != nil {
				log.Printf("report cache set failed: %v", err)
			}
		}
	}
	return result, nil
}

// categoryTotals gets the list of categories and calculates each one in its own goroutine.
// Every goroutine writes only to its own element of the result slices, so there is
// no shared mutable state; the results are combined after wg.Wait().
func (s *Ledger) categoryTotals(ctx context.Context, from, to string) ([]domain.CategoryTotals, error) {
	categories, err := s.expenses.CategoriesInPeriod(ctx, from, to)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			log.Printf("report: calculation interrupted: %v", ctxErr)
			return nil, ctxErr
		}
		return nil, err
	}

	stopHeartbeat := startHeartbeat(ctx, heartbeatInterval)
	defer stopHeartbeat()

	results := make([]domain.CategoryTotals, len(categories))
	errs := make([]error, len(categories))
	var wg sync.WaitGroup
	for i, category := range categories {
		wg.Add(1)
		go func(i int, category string) {
			defer wg.Done()
			// Do not start work that is already cancelled.
			if err := ctx.Err(); err != nil {
				errs[i] = err
				return
			}
			results[i], errs[i] = s.expenses.TotalsForCategory(ctx, category, from, to)
		}(i, category)
	}
	wg.Wait()

	if ctxErr := ctx.Err(); ctxErr != nil {
		log.Printf("report: calculation interrupted: %v", ctxErr)
		return nil, ctxErr
	}
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return results, nil
}

// startHeartbeat logs a short message every interval while a calculation is running.
// The returned stop function stops the ticker and waits for the goroutine to exit;
// the goroutine also exits by itself when ctx is cancelled.
func startHeartbeat(ctx context.Context, interval time.Duration) (stop func()) {
	done := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				log.Print("report: calculation in progress")
			case <-ctx.Done():
				log.Printf("report: heartbeat stopped: %v", ctx.Err())
				return
			case <-done:
				log.Print("report: heartbeat stopped: calculation finished")
				return
			}
		}
	}()
	return func() {
		close(done)
		<-finished
	}
}

// Reset deletes ALL ledger data and resets IDs. Never call at startup.
// It is not part of Service: tests reach it through a separate interface.
func (s *Ledger) Reset(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()
	if err := s.purger.DeleteAll(ctx); err != nil {
		return err
	}
	s.invalidateReports(ctx)
	return nil
}

func (s *Ledger) invalidateReports(ctx context.Context) {
	if s.cache == nil {
		return
	}
	if err := s.cache.Invalidate(ctx); err != nil {
		log.Printf("report cache invalidation failed (TTL limits staleness): %v", err)
	}
}
