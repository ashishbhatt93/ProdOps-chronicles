package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prodops-chronicles/prodops/internal/repository"
)

// NewPool creates and validates a pgxpool connection.
func NewPool(ctx context.Context, connString string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}
	return pool, nil
}

// ── pgxpool.Pool adapter ──────────────────────────────────────────────────────

// poolExecutor wraps *pgxpool.Pool to satisfy repository.Executor.
type poolExecutor struct{ pool *pgxpool.Pool }

func PoolExecutor(pool *pgxpool.Pool) repository.Executor {
	return &poolExecutor{pool: pool}
}

func (p *poolExecutor) Exec(ctx context.Context, sql string, args ...any) (interface{ RowsAffected() int64 }, error) {
	tag, err := p.pool.Exec(ctx, sql, args...)
	return &pgxCommandTag{tag}, err
}

func (p *poolExecutor) Query(ctx context.Context, sql string, args ...any) (repository.Rows, error) {
	rows, err := p.pool.Query(ctx, sql, args...)
	return &pgxRows{rows}, err
}

func (p *poolExecutor) QueryRow(ctx context.Context, sql string, args ...any) repository.Row {
	return p.pool.QueryRow(ctx, sql, args...)
}

// ── pgx.Tx adapter ────────────────────────────────────────────────────────────

// txExecutor wraps pgx.Tx to satisfy repository.Executor.
type txExecutor struct{ tx pgx.Tx }

func TxExecutor(tx pgx.Tx) repository.Executor {
	return &txExecutor{tx: tx}
}

func (t *txExecutor) Exec(ctx context.Context, sql string, args ...any) (interface{ RowsAffected() int64 }, error) {
	tag, err := t.tx.Exec(ctx, sql, args...)
	return &pgxCommandTag{tag}, err
}

func (t *txExecutor) Query(ctx context.Context, sql string, args ...any) (repository.Rows, error) {
	rows, err := t.tx.Query(ctx, sql, args...)
	return &pgxRows{rows}, err
}

func (t *txExecutor) QueryRow(ctx context.Context, sql string, args ...any) repository.Row {
	return t.tx.QueryRow(ctx, sql, args...)
}

// ── helpers ───────────────────────────────────────────────────────────────────

type pgxCommandTag struct{ tag interface{ RowsAffected() int64 } }

func (c *pgxCommandTag) RowsAffected() int64 { return c.tag.RowsAffected() }

type pgxRows struct{ rows pgx.Rows }

func (r *pgxRows) Next() bool        { return r.rows.Next() }
func (r *pgxRows) Scan(d ...any) error { return r.rows.Scan(d...) }
func (r *pgxRows) Close()            { r.rows.Close() }
func (r *pgxRows) Err() error        { return r.rows.Err() }
