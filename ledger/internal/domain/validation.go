package domain

// Validatable задаёт контракт на проверку валидности данных структуры.
// Методы Validate() не логируют — они только возвращают error.
type Validatable interface {
	Validate() error
}

// CheckValid принимает любой Validatable и возвращает результат его проверки.
func CheckValid(v Validatable) error {
	return v.Validate()
}

// ValidationError marks an error caused by invalid input.
// The message is the message of the wrapped error, unchanged.
type ValidationError struct {
	Err error
}

func (e *ValidationError) Error() string { return e.Err.Error() }

func (e *ValidationError) Unwrap() error { return e.Err }
