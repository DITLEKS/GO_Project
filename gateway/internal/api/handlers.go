package api

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/DITLEKS/GO_Project/ledger"
)

const (
	contentTypeJSON    = "application/json; charset=utf-8"
	maxBodyBytes       = 1 << 20
	reportRangeMessage = "from/to must be YYYY-MM-DD and from <= to"
)

// Handler depends only on the ledger.Service interface.
// Validation, the budget rule and storage live behind it.
type Handler struct {
	svc ledger.Service
}

func NewHandler(svc ledger.Service) *Handler { return &Handler{svc: svc} }

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

// writeServiceError maps service errors to the unchanged HTTP contract:
// validation → 400 with the validation message, budget → 409, the rest → 500.
func writeServiceError(w http.ResponseWriter, op string, err error) {
	var invalid *ledger.ValidationError
	switch {
	case errors.As(err, &invalid):
		writeError(w, http.StatusBadRequest, invalid.Error())
	case errors.Is(err, ledger.ErrBudgetExceeded):
		writeError(w, http.StatusConflict, "budget exceeded")
	default:
		log.Printf("неожиданная ошибка %s: %v", op, err)
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func (h *Handler) Transactions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listTransactions(w, r)
	case http.MethodPost:
		h.createTransaction(w, r)
	default:
		methodNotAllowed(w, "GET, POST")
	}
}

func (h *Handler) Budgets(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listBudgets(w, r)
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
	created, err := h.svc.AddTransaction(r.Context(), req.ToDomain())
	if err != nil {
		writeServiceError(w, "AddTransaction", err)
		return
	}
	writeJSON(w, http.StatusCreated, NewTransactionResponse(created))
}

func (h *Handler) listTransactions(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.ListTransactions(r.Context())
	if err != nil {
		writeServiceError(w, "ListTransactions", err)
		return
	}
	writeJSON(w, http.StatusOK, NewTransactionResponses(list))
}

func (h *Handler) createBudget(w http.ResponseWriter, r *http.Request) {
	var req CreateBudgetRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "некорректный JSON: "+err.Error())
		return
	}
	saved, err := h.svc.SetBudget(r.Context(), req.ToDomain())
	if err != nil {
		writeServiceError(w, "SetBudget", err)
		return
	}
	writeJSON(w, http.StatusCreated, NewBudgetResponse(saved))
}

func (h *Handler) listBudgets(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.ListBudgets(r.Context())
	if err != nil {
		writeServiceError(w, "ListBudgets", err)
		return
	}
	writeJSON(w, http.StatusOK, NewBudgetResponses(list))
}

// Additive endpoint; existing routes and DTOs are unchanged.
func (h *Handler) ReportSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	from, errFrom := time.Parse("2006-01-02", r.URL.Query().Get("from"))
	to, errTo := time.Parse("2006-01-02", r.URL.Query().Get("to"))
	if errFrom != nil || errTo != nil {
		writeError(w, http.StatusBadRequest, reportRangeMessage)
		return
	}
	report, err := h.svc.ReportSummary(r.Context(), from, to)
	if err != nil {
		var invalid *ledger.ValidationError
		if errors.As(err, &invalid) {
			writeError(w, http.StatusBadRequest, reportRangeMessage)
			return
		}
		writeServiceError(w, "ReportSummary", err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}
