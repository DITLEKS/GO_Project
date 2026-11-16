package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/DITLEKS/GO_Project/ledger/internal/domain"
)

// LoadBudgets reads a JSON array of budgets from r and saves each one through the service.
func LoadBudgets(ctx context.Context, svc Service, r io.Reader) error {
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
	var list []domain.Budget
	if err := json.Unmarshal(data, &list); err != nil {
		return fmt.Errorf("ошибка парсинга JSON бюджетов: %w", err)
	}
	for i, b := range list {
		if _, err := svc.SetBudget(ctx, b); err != nil {
			return fmt.Errorf("некорректный бюджет #%d в JSON: %w", i+1, err)
		}
	}
	return nil
}
