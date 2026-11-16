package api

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/DITLEKS/GO_Project/ledger"
)

// maxBulkItems limits the number of transactions in one bulk request.
const maxBulkItems = 1000

// parseWorkers reads the optional ?workers=N parameter.
// An empty value means the default of Ledger (0 is passed to the service).
func parseWorkers(raw string) (int, error) {
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > ledger.MaxImportWorkers {
		return 0, fmt.Errorf("workers must be an integer from 1 to %d", ledger.MaxImportWorkers)
	}
	return n, nil
}

// TransactionsBulk: POST /api/transactions/bulk?workers=N.
// The body is a JSON array of the same objects as in POST /api/transactions.
// The answer is 200 with the summary even when some elements are rejected;
// cancellation or timeout gives 504, as for the other endpoints.
func (h *Handler) TransactionsBulk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	workers, err := parseWorkers(r.URL.Query().Get("workers"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var items []CreateTransactionRequest
	if err := decodeJSON(w, r, &items); err != nil {
		writeError(w, http.StatusBadRequest, "некорректный JSON: "+err.Error())
		return
	}
	if items == nil {
		writeError(w, http.StatusBadRequest, "некорректный JSON: ожидался массив транзакций")
		return
	}
	if len(items) > maxBulkItems {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("too many transactions: %d, maximum is %d", len(items), maxBulkItems))
		return
	}
	txs := make([]ledger.Transaction, 0, len(items))
	for _, item := range items {
		txs = append(txs, item.ToDomain())
	}
	result, err := h.svc.ImportTransactions(r.Context(), txs, workers)
	if err != nil {
		// The partial summary is logged; the client gets the agreed timeout answer.
		log.Printf("ImportTransactions: частичный результат: accepted=%d rejected=%d из %d", result.Accepted, result.Rejected, len(txs))
		writeServiceError(w, r, "ImportTransactions", err)
		return
	}
	if result.Errors == nil {
		result.Errors = []ledger.ImportError{}
	}
	writeJSON(w, http.StatusOK, result)
}
