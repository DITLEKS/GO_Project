package ledger

// Validatable задаёт контракт на проверку валидности данных структуры.
// Методы Validate() не логируют — они только возвращают error.
type Validatable interface {
	Validate() error
}

// CheckValid принимает любой Validatable и возвращает результат его проверки
// (полиморфизм через интерфейс).
func CheckValid(v Validatable) error {
	return v.Validate()
}
