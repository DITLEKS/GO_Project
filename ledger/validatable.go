package ledger

import "github.com/DITLEKS/GO_Project/ledger/internal/domain"

type (
	// Validatable задаёт контракт на проверку валидности данных структуры.
	Validatable = domain.Validatable
	// ValidationError marks a service error caused by invalid input (HTTP 400 in Gateway).
	ValidationError = domain.ValidationError
)

// CheckValid принимает любой Validatable и возвращает результат его проверки.
func CheckValid(v Validatable) error {
	return domain.CheckValid(v)
}
