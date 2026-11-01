package ledger

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
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

// budgets — хранилище бюджетов в памяти: ключ — название категории.
// Изначально заполнено парой примеров.
var budgets = map[string]Budget{
	"Продукты":  {Category: "Продукты", Limit: 5000, Period: "месяц"},
	"Транспорт": {Category: "Транспорт", Limit: 2000, Period: "месяц"},
}

// SetBudget добавляет новый бюджет или обновляет существующий
// (например, позволяет изменить лимит для категории).
// Перед сохранением вызывается b.Validate(); при ошибке бюджет не сохраняется.
func SetBudget(b Budget) error {
	b.Category = strings.TrimSpace(b.Category)
	if err := b.Validate(); err != nil {
		return err
	}
	budgets[b.Category] = b
	return nil
}

// GetBudget возвращает бюджет категории и признак его наличия.
func GetBudget(category string) (Budget, bool) {
	b, ok := budgets[category]
	return b, ok
}

// ListBudgets возвращает копию всех бюджетов, отсортированную по категории.
func ListBudgets() []Budget {
	result := make([]Budget, 0, len(budgets))
	for _, b := range budgets {
		result = append(result, b)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Category < result[j].Category })
	return result
}

// SpentByCategory возвращает сумму расходов по категории.
// Расходами считаются транзакции с отрицательной суммой; доходы не учитываются.
func SpentByCategory(category string) float64 {
	var total float64
	for _, tx := range transactions {
		if tx.Category == category && tx.Amount < 0 {
			total += math.Abs(tx.Amount)
		}
	}
	return total
}

// checkBudget проверяет, не превысит ли транзакция лимит бюджета своей категории.
// Если бюджет для категории не задан или транзакция является доходом — проверка пропускается.
func checkBudget(tx Transaction) error {
	if tx.Amount >= 0 {
		return nil
	}
	b, ok := budgets[tx.Category]
	if !ok {
		return nil
	}
	spent := SpentByCategory(tx.Category)
	amount := math.Abs(tx.Amount)
	if spent+amount > b.Limit {
		return fmt.Errorf("%w: категория %q, лимит %.2f, уже потрачено %.2f, попытка потратить ещё %.2f",
			ErrBudgetExceeded, tx.Category, b.Limit, spent, amount)
	}
	return nil
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
