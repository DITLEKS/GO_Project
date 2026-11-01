package api

import (
	"strings"

	"github.com/DITLEKS/GO_Project/ledger"
)

// CreateTransactionRequest — JSON-тело запроса на создание транзакции.
type CreateTransactionRequest struct {
	Amount      float64 `json:"amount"`
	Category    string  `json:"category"`
	Description string  `json:"description"`
	Date        string  `json:"date"`
}

// TransactionResponse — публичное представление ledger.Transaction.
type TransactionResponse struct {
	ID          int     `json:"id"`
	Amount      float64 `json:"amount"`
	Category    string  `json:"category"`
	Description string  `json:"description"`
	Date        string  `json:"date"`
}

// CreateBudgetRequest — JSON-тело запроса на создание/обновление бюджета.
// Поле period необязательное: без него период бюджета очищается.
type CreateBudgetRequest struct {
	Category string  `json:"category"`
	Limit    float64 `json:"limit"`
	Period   string  `json:"period,omitempty"`
}

// BudgetResponse — публичное представление ledger.Budget.
type BudgetResponse struct {
	Category string  `json:"category"`
	Limit    float64 `json:"limit"`
	Period   string  `json:"period,omitempty"`
}

// ErrorResponse — единый формат ошибок: {"error":"..."}.
type ErrorResponse struct {
	Error string `json:"error"`
}

// ToDomain преобразует запрос в доменную транзакцию (ID назначает ledger).
func (r CreateTransactionRequest) ToDomain() ledger.Transaction {
	return ledger.Transaction{
		Amount:      r.Amount,
		Category:    strings.TrimSpace(r.Category),
		Description: r.Description,
		Date:        r.Date,
	}
}

// ToDomain преобразует запрос в доменный бюджет.
func (r CreateBudgetRequest) ToDomain() ledger.Budget {
	return ledger.Budget{
		Category: strings.TrimSpace(r.Category),
		Limit:    r.Limit,
		Period:   r.Period,
	}
}

// NewTransactionResponse строит ответ по доменной транзакции.
func NewTransactionResponse(tx ledger.Transaction) TransactionResponse {
	return TransactionResponse{
		ID:          tx.ID,
		Amount:      tx.Amount,
		Category:    tx.Category,
		Description: tx.Description,
		Date:        tx.Date,
	}
}

// NewBudgetResponse строит ответ по доменному бюджету.
func NewBudgetResponse(b ledger.Budget) BudgetResponse {
	return BudgetResponse{Category: b.Category, Limit: b.Limit, Period: b.Period}
}

// NewTransactionResponses преобразует список; всегда возвращает не-nil срез (в JSON будет []).
func NewTransactionResponses(list []ledger.Transaction) []TransactionResponse {
	result := make([]TransactionResponse, 0, len(list))
	for _, tx := range list {
		result = append(result, NewTransactionResponse(tx))
	}
	return result
}

// NewBudgetResponses преобразует список бюджетов; всегда возвращает не-nil срез.
func NewBudgetResponses(list []ledger.Budget) []BudgetResponse {
	result := make([]BudgetResponse, 0, len(list))
	for _, b := range list {
		result = append(result, NewBudgetResponse(b))
	}
	return result
}
