// Package ledger is the public entry point of the Ledger module.
// Other modules cannot import ledger/internal/..., so this package re-exports
// the domain types, the service interface and the factory. It has no logic of its own.
package ledger

import (
	"context"

	"github.com/DITLEKS/GO_Project/ledger/internal/app"
	"github.com/DITLEKS/GO_Project/ledger/internal/service"
)

// Service is the application interface used by Gateway.
type Service = service.Service

// NewService reads the configuration from the environment, connects to the
// database (ctx is used for the startup Ping) and returns a ready service,
// a function that releases the resources, and an error.
// It does not run migrations.
func NewService(ctx context.Context) (Service, func() error, error) {
	return app.New(ctx)
}
