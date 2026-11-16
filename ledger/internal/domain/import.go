package domain

// ImportError describes one rejected element of a batch import.
type ImportError struct {
	// Index is the position of the element in the input array, starting at 0.
	Index int    `json:"index"`
	Error string `json:"error"`
}

// ImportResult is the summary of a batch import.
// When the import is interrupted, Accepted + Rejected is less than the input size.
type ImportResult struct {
	Accepted int           `json:"accepted"`
	Rejected int           `json:"rejected"`
	Errors   []ImportError `json:"errors"`
}
