package ledger

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"time"
)

const (
	dateLayout = "2006-01-02"
	reportTTL  = 30 * time.Second
)

// ErrInvalidRange — некорректный интервал дат отчёта.
var ErrInvalidRange = errors.New("invalid report range")

// CategorySummary — сумма (SUM(amount)) по категории; расходы отрицательные, доходы положительные.
type CategorySummary struct {
	Category string  `json:"category"`
	Total    float64 `json:"total"`
}

// ReportSummary — отчёт по категориям за интервал. Cached=true, если результат взят из Redis.
type ReportSummary struct {
	From   string            `json:"from"`
	To     string            `json:"to"`
	Items  []CategorySummary `json:"items"`
	Total  float64           `json:"total"`
	Cached bool              `json:"cached"`
}

func validateRange(from, to string) error {
	f, err := time.Parse(dateLayout, from)
	if err != nil {
		return fmt.Errorf("%w: from должна быть датой ГГГГ-ММ-ДД, получено %q", ErrInvalidRange, from)
	}
	t, err := time.Parse(dateLayout, to)
	if err != nil {
		return fmt.Errorf("%w: to должна быть датой ГГГГ-ММ-ДД, получено %q", ErrInvalidRange, to)
	}
	if f.After(t) {
		return fmt.Errorf("%w: from (%s) не может быть позже to (%s)", ErrInvalidRange, from, to)
	}
	return nil
}

// GetReportSummary строит отчёт по категориям за [from, to] (ГГГГ-ММ-ДД, включительно).
// Если подключён Redis, результат кэшируется по ключу report:summary:<from>:<to> на 30 секунд:
// cache hit — возвращаем из Redis, cache miss — считаем в БД и кладём в Redis. Кэш истекает по TTL.
func GetReportSummary(ctx context.Context, from, to string) (ReportSummary, error) {
	if err := validateRange(from, to); err != nil {
		return ReportSummary{}, err
	}

	key := fmt.Sprintf("report:summary:%s:%s", from, to)
	if cacheClient != nil {
		var cached ReportSummary
		hit, err := cacheClient.GetJSON(ctx, key, &cached)
		switch {
		case err != nil:
			log.Printf("кэш отчёта: ошибка чтения %s: %v", key, err)
		case hit:
			log.Printf("cache hit: %s", key)
			cached.Cached = true
			return cached, nil
		default:
			log.Printf("cache miss: %s", key)
		}
	}

	items, err := store.summary(ctx, from, to)
	if err != nil {
		return ReportSummary{}, fmt.Errorf("не удалось построить отчёт: %w", err)
	}
	if items == nil {
		items = []CategorySummary{}
	}
	var total float64
	for _, it := range items {
		total += it.Total
	}
	report := ReportSummary{From: from, To: to, Items: items, Total: math.Round(total*100) / 100}

	if cacheClient != nil {
		if err := cacheClient.SetJSON(ctx, key, report, reportTTL); err != nil {
			log.Printf("кэш отчёта: ошибка записи %s: %v", key, err)
		}
	}
	return report, nil
}
