package ledger

import (
	"context"
	"math"
	"sort"
	"sync"
)

// memoryStore — хранилище в памяти (для тестов, демо и LEDGER_STORAGE=memory).
type memoryStore struct {
	mu           sync.Mutex
	transactions []Transaction
	budgets      map[string]Budget
}

func newMemoryStore(withDefaults bool) *memoryStore {
	s := &memoryStore{transactions: []Transaction{}, budgets: map[string]Budget{}}
	if withDefaults {
		s.budgets["Продукты"] = Budget{Category: "Продукты", Limit: 5000, Period: "месяц"}
		s.budgets["Транспорт"] = Budget{Category: "Транспорт", Limit: 2000, Period: "месяц"}
	}
	return s
}

func (s *memoryStore) setBudget(_ context.Context, b Budget) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.budgets[b.Category] = b
	return nil
}

func (s *memoryStore) getBudget(_ context.Context, category string) (Budget, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.budgets[category]
	return b, ok, nil
}

func (s *memoryStore) listBudgets(_ context.Context) ([]Budget, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]Budget, 0, len(s.budgets))
	for _, b := range s.budgets {
		result = append(result, b)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Category < result[j].Category })
	return result, nil
}

// spentLocked — сумма расходов (отрицательных сумм) по категории; вызывать под s.mu.
func (s *memoryStore) spentLocked(category string) float64 {
	var total float64
	for _, tx := range s.transactions {
		if tx.Category == category && tx.Amount < 0 {
			total += math.Abs(tx.Amount)
		}
	}
	return total
}

func (s *memoryStore) spentByCategory(_ context.Context, category string) (float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.spentLocked(category), nil
}

func (s *memoryStore) addTransaction(_ context.Context, tx Transaction) (Transaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if tx.Amount < 0 {
		if b, ok := s.budgets[tx.Category]; ok {
			spent := s.spentLocked(tx.Category)
			amount := math.Abs(tx.Amount)
			if spent+amount > b.Limit {
				return Transaction{}, budgetExceeded(tx.Category, b.Limit, spent, amount)
			}
		}
	}

	tx.ID = len(s.transactions) + 1
	s.transactions = append(s.transactions, tx)
	return tx, nil
}

func (s *memoryStore) listTransactions(_ context.Context) ([]Transaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]Transaction, len(s.transactions))
	copy(result, s.transactions)
	return result, nil
}

func (s *memoryStore) summary(_ context.Context, from, to string) ([]CategorySummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	totals := map[string]float64{}
	for _, tx := range s.transactions {
		if tx.Date != "" && tx.Date >= from && tx.Date <= to {
			totals[tx.Category] += tx.Amount
		}
	}
	items := make([]CategorySummary, 0, len(totals))
	for category, total := range totals {
		items = append(items, CategorySummary{Category: category, Total: total})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Category < items[j].Category })
	return items, nil
}
