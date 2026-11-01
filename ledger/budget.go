package ledger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"strings"
	"time"
)

// Budget описывает бюджет (лимит расходов) для категории.
type Budget struct {
	Category string  `json:"category"`         // категория, на которую выделен бюджет
	Limit    float64 `json:"limit"`            // лимит бюджета
	Period   string  `json:"period,omitempty"` // период (например, "месяц"), необязательное поле
}

// Validate реализует Validatable: лимит должен быть положительным, категория — непустой.
// Метод ничего не логирует, только возвращает ошибку.
func (b Budget) Validate() error {
	if strings.TrimSpace(b.Category) == "" {
		return errors.New("категория бюджета не может быть пустой")
	}
	if !(b.Limit > 0) || math.IsInf(b.Limit, 0) {
		return fmt.Errorf("лимит бюджета для категории %q должен быть больше нуля, получено %.2f", strings.TrimSpace(b.Category), b.Limit)
	}
	return nil
}

// ErrBudgetExceeded возвращается, если транзакция превышает бюджет категории.
var ErrBudgetExceeded = errors.New("budget exceeded")

func budgetExceeded(category string, limit, spent, amount float64) error {
	return fmt.Errorf("%w: категория %q, лимит %.2f, уже потрачено %.2f, попытка потратить ещё %.2f",
		ErrBudgetExceeded, category, limit, spent, amount)
}

// Ключ и TTL кэша списка бюджетов (работает только при подключённом Redis).
const (
	budgetsCacheKey = "budgets:all"
	budgetsCacheTTL = 15 * time.Second
)

// SetBudget валидирует бюджет и делает UPSERT по категории; после записи сбрасывает кэш budgets:all.
func SetBudget(b Budget) error {
	b.Category = strings.TrimSpace(b.Category)
	if err := b.Validate(); err != nil {
		return err
	}

	ctx := context.Background()
	if err := store.setBudget(ctx, b); err != nil {
		return fmt.Errorf("не удалось сохранить бюджет: %w", err)
	}
	if cacheClient != nil {
		if err := cacheClient.Del(ctx, budgetsCacheKey); err != nil {
			log.Printf("кэш бюджетов: не удалось инвалидировать %s (сработает TTL): %v", budgetsCacheKey, err)
		}
	}
	return nil
}

// GetBudget возвращает бюджет категории и признак его наличия.
func GetBudget(category string) (Budget, bool, error) {
	b, ok, err := store.getBudget(context.Background(), category)
	if err != nil {
		return Budget{}, false, fmt.Errorf("не удалось прочитать бюджет %q: %w", category, err)
	}
	return b, ok, nil
}

// ListBudgets возвращает все бюджеты, отсортированные по категории.
// При наличии Redis результат кэшируется на короткий TTL.
func ListBudgets() ([]Budget, error) {
	ctx := context.Background()
	if cacheClient != nil {
		var cached []Budget
		hit, err := cacheClient.GetJSON(ctx, budgetsCacheKey, &cached)
		if err != nil {
			log.Printf("кэш бюджетов: ошибка чтения: %v", err)
		} else if hit {
			return cached, nil
		}
	}

	list, err := store.listBudgets(ctx)
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать бюджеты: %w", err)
	}
	if cacheClient != nil {
		if err := cacheClient.SetJSON(ctx, budgetsCacheKey, list, budgetsCacheTTL); err != nil {
			log.Printf("кэш бюджетов: ошибка записи: %v", err)
		}
	}
	return list, nil
}

// SpentByCategory возвращает сумму расходов по категории.
// Расходами считаются транзакции с отрицательной суммой; доходы не учитываются.
func SpentByCategory(category string) (float64, error) {
	spent, err := store.spentByCategory(context.Background(), category)
	if err != nil {
		return 0, fmt.Errorf("не удалось посчитать расходы по категории %q: %w", category, err)
	}
	return spent, nil
}

// LoadBudgets читает бюджеты в формате JSON (массив объектов) из r
// и добавляет каждый из них через SetBudget.
func LoadBudgets(r io.Reader) error {
	if r == nil {
		return errors.New("источник данных бюджетов не задан (nil reader)")
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("ошибка чтения бюджетов: %w", err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return errors.New("ошибка парсинга бюджетов: источник пуст")
	}

	var list []Budget
	if err := json.Unmarshal(data, &list); err != nil {
		return fmt.Errorf("ошибка парсинга JSON бюджетов: %w", err)
	}

	for i, b := range list {
		if err := SetBudget(b); err != nil {
			return fmt.Errorf("некорректный бюджет #%d в JSON: %w", i+1, err)
		}
	}
	return nil
}
