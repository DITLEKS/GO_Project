package api

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"sync"

	"github.com/DITLEKS/GO_Project/ledger"
)

const (
	contentTypeJSON = "application/json; charset=utf-8"
	maxBodyBytes    = 1 << 20
)

// Handler содержит HTTP-обработчики. Хранилище ledger — глобальные переменные в памяти
// без блокировок, поэтому все обращения к нему сериализуются мьютексом (HTTP-запросы обслуживаются конкурентно).
type Handler struct {
	mu sync.Mutex
}

// NewHandler создаёт набор обработчиков.
func NewHandler() *Handler {
	return &Handler{}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("ошибка записи ответа: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, ErrorResponse{Error: msg})
}

func methodNotAllowed(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func notFound(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotFound, "not found")
}

func ping(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "pong"})
}

// decodeJSON читает ровно один JSON-объект из тела запроса; неизвестные поля — ошибка.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("после JSON-объекта есть лишние данные")
	}
	return nil
}

// Transactions обслуживает /api/transactions: GET — список, POST — создание.
func (h *Handler) Transactions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listTransactions(w)
	case http.MethodPost:
		h.createTransaction(w, r)
	default:
		methodNotAllowed(w, "GET, POST")
	}
}

// Budgets обслуживает /api/budgets: GET — список, POST — создание/обновление.
func (h *Handler) Budgets(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listBudgets(w)
	case http.MethodPost:
		h.createBudget(w, r)
	default:
		methodNotAllowed(w, "GET, POST")
	}
}

func (h *Handler) createTransaction(w http.ResponseWriter, r *http.Request) {
	var req CreateTransactionRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "некорректный JSON: "+err.Error())
		return
	}

	tx := req.ToDomain()
	if err := tx.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if err := ledger.AddTransaction(tx); err != nil {
		if errors.Is(err, ledger.ErrBudgetExceeded) {
			writeError(w, http.StatusConflict, "budget exceeded")
			return
		}
		log.Printf("неожиданная ошибка AddTransaction: %v", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// AddTransaction не возвращает созданную запись; последняя в списке — наша (мьютекс захвачен).
	list := ledger.ListTransactions()
	if len(list) == 0 {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusCreated, NewTransactionResponse(list[len(list)-1]))
}

func (h *Handler) listTransactions(w http.ResponseWriter) {
	h.mu.Lock()
	list := ledger.ListTransactions()
	h.mu.Unlock()

	writeJSON(w, http.StatusOK, NewTransactionResponses(list))
}

func (h *Handler) createBudget(w http.ResponseWriter, r *http.Request) {
	var req CreateBudgetRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "некорректный JSON: "+err.Error())
		return
	}

	b := req.ToDomain()
	if err := b.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.mu.Lock()
	err := ledger.SetBudget(b)
	h.mu.Unlock()
	if err != nil {
		log.Printf("неожиданная ошибка SetBudget: %v", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusCreated, NewBudgetResponse(b))
}

func (h *Handler) listBudgets(w http.ResponseWriter) {
	h.mu.Lock()
	list := ledger.ListBudgets()
	h.mu.Unlock()

	writeJSON(w, http.StatusOK, NewBudgetResponses(list))
}
