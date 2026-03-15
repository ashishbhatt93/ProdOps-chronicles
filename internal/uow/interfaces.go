package uow

import (
	"context"

	"github.com/prodops-chronicles/prodops/internal/repository"
)

// UnitOfWork holds an open transaction and exposes the Executor for
// repositories that participate in atomic operations.
type UnitOfWork interface {
	Executor() repository.Executor
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// Factory creates new UnitOfWork instances.
type Factory interface {
	Begin(ctx context.Context) (UnitOfWork, error)
}
