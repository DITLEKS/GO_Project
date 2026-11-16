package ledger

import (
	"github.com/DITLEKS/GO_Project/ledger/internal/domain"
	"github.com/DITLEKS/GO_Project/ledger/internal/service"
)

type (
	// ImportResult is the summary returned by Service.ImportTransactions.
	ImportResult = domain.ImportResult
	// ImportError describes one rejected element of a batch import.
	ImportError = domain.ImportError
)

const (
	// DefaultImportWorkers is used when workers <= 0 is passed to ImportTransactions.
	DefaultImportWorkers = service.DefaultImportWorkers
	// MaxImportWorkers is the upper bound of the worker pool.
	MaxImportWorkers = service.MaxImportWorkers
)
