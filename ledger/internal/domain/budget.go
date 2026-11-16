package domain

import (
	"errors"
	"fmt"
	"math"
	"strings"
)

// ErrBudgetExceeded is returned when an expense does not fit the category limit.
var ErrBudgetExceeded = errors.New("budget exceeded")

// Budget is a cumulative spending limit for a category.
type Budget struct {
	Category string  `json:"category"`
	Limit    float64 `json:"limit"`
	Period   string  `json:"period,omitempty"`
}

// Validate only returns errors, never logs.
func (b Budget) Validate() error {
	if strings.TrimSpace(b.Category) == "" {
		return errors.New("категория бюджета не может быть пустой")
	}
	if !(b.Limit > 0) || math.IsInf(b.Limit, 0) || b.Limit > MaxAmount || math.Round(b.Limit*100) == 0 {
		return fmt.Errorf("лимит бюджета для категории %q должен быть больше нуля, получено %.2f", strings.TrimSpace(b.Category), b.Limit)
	}
	return nil
}
