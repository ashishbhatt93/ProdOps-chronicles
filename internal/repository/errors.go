package repository

import "github.com/prodops-chronicles/prodops/internal/domain"

// Re-export domain sentinel errors so callers import only one package.
var (
	ErrNotFound     = domain.ErrNotFound
	ErrConflict     = domain.ErrConflict
	ErrInvalidInput = domain.ErrInvalidInput
)
