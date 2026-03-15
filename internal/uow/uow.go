package uow

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prodops-chronicles/prodops/internal/repository"
	"github.com/prodops-chronicles/prodops/internal/repository/postgres"
)

// pgxUnitOfWork wraps a pgx.Tx.
type pgxUnitOfWork struct {
	tx  pgx.Tx
	ex  repository.Executor
}

func (u *pgxUnitOfWork) Executor() repository.Executor { return u.ex }
func (u *pgxUnitOfWork) Commit(ctx context.Context) error  { return u.tx.Commit(ctx) }
func (u *pgxUnitOfWork) Rollback(ctx context.Context) error { return u.tx.Rollback(ctx) }

// pgxFactory creates pgxUnitOfWork instances from a pool.
type pgxFactory struct{ pool *pgxpool.Pool }

func NewFactory(pool *pgxpool.Pool) Factory {
	return &pgxFactory{pool: pool}
}

func (f *pgxFactory) Begin(ctx context.Context) (UnitOfWork, error) {
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &pgxUnitOfWork{
		tx: tx,
		ex: postgres.TxExecutor(tx),
	}, nil
}
