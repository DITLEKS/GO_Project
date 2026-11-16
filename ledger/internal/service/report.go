package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/DITLEKS/GO_Project/ledger/internal/domain"
)

// ReportSummary includes both boundary dates; grouping totals keep their signs.
// Cache failures are fail-open; the repository is authoritative.
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
	rows, err := s.expenses.TotalsByCategory(ctx, fromDate, toDate)
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
