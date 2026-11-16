package domain

// CategorySummary is a signed total for one category.
type CategorySummary struct {
	Category string  `json:"category"`
	Total    float64 `json:"total"`
}

// ReportSummary is the report for an inclusive date interval.
type ReportSummary struct {
	From       string            `json:"from"`
	To         string            `json:"to"`
	Income     float64           `json:"income"`
	Expenses   float64           `json:"expenses"`
	Balance    float64           `json:"balance"`
	Categories []CategorySummary `json:"categories"`
}

// CategoryTotals is one aggregated row used to build ReportSummary.
type CategoryTotals struct {
	Category string
	Total    float64
	Income   float64
	Expenses float64
}
