package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prodops-chronicles/prodops/internal/domain"
	"github.com/prodops-chronicles/prodops/internal/repository"
)

type RunRepo struct{ pool *pgxpool.Pool }

func NewRunRepo(pool *pgxpool.Pool) *RunRepo { return &RunRepo{pool: pool} }

func (r *RunRepo) CreateRun(ctx context.Context, playerID uuid.UUID) (*domain.Run, error) {
	run := &domain.Run{}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO public.runs (player_id, status)
		VALUES ($1, 'in_progress')
		RETURNING id, player_id, status, started_at, completed_at, final_performance_score
	`, playerID).Scan(&run.ID, &run.PlayerID, &run.Status,
		&run.StartedAt, &run.CompletedAt, &run.FinalPerformanceScore)
	if err != nil {
		return nil, err
	}
	return run, nil
}

func (r *RunRepo) GetActiveRun(ctx context.Context, playerID uuid.UUID) (*domain.Run, error) {
	run := &domain.Run{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, player_id, status, started_at, completed_at, final_performance_score
		FROM public.runs
		WHERE player_id = $1 AND status = 'in_progress'
		LIMIT 1
	`, playerID).Scan(&run.ID, &run.PlayerID, &run.Status,
		&run.StartedAt, &run.CompletedAt, &run.FinalPerformanceScore)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	return run, err
}

func (r *RunRepo) GetRun(ctx context.Context, runID uuid.UUID) (*domain.Run, error) {
	run := &domain.Run{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, player_id, status, started_at, completed_at, final_performance_score
		FROM public.runs WHERE id = $1
	`, runID).Scan(&run.ID, &run.PlayerID, &run.Status,
		&run.StartedAt, &run.CompletedAt, &run.FinalPerformanceScore)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	return run, err
}

func (r *RunRepo) ListRuns(ctx context.Context, playerID uuid.UUID) ([]*domain.Run, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, player_id, status, started_at, completed_at, final_performance_score
		FROM public.runs WHERE player_id = $1
		ORDER BY started_at DESC
	`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []*domain.Run
	for rows.Next() {
		run := &domain.Run{}
		if err := rows.Scan(&run.ID, &run.PlayerID, &run.Status,
			&run.StartedAt, &run.CompletedAt, &run.FinalPerformanceScore); err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (r *RunRepo) CompleteRun(ctx context.Context, ex repository.Executor, runID uuid.UUID, finalScore int) error {
	_, err := ex.Exec(ctx, `
		UPDATE public.runs
		SET status = 'completed', completed_at = now(), final_performance_score = $1
		WHERE id = $2
	`, finalScore, runID)
	return err
}
