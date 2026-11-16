package ledger

import "context"

// Resetter is implemented by the service returned from NewService.
// Reset deletes ALL ledger data and resets IDs. It is deliberately not part of
// Service: only tests use it, with a dedicated TEST_DATABASE_URL.
type Resetter interface {
	Reset(ctx context.Context) error
}
