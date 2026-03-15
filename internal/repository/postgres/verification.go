package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prodops-chronicles/prodops/internal/domain"
	"github.com/prodops-chronicles/prodops/internal/repository"
)

type VerificationRepo struct{ pool *pgxpool.Pool }

func NewVerificationRepo(pool *pgxpool.Pool) *VerificationRepo {
	return &VerificationRepo{pool: pool}
}

func (r *VerificationRepo) GetTaskCompletion(ctx context.Context, runID uuid.UUID, taskID string) (*domain.TaskCompletion, error) {
	tc := &domain.TaskCompletion{}
	var checkResultsJSON []byte
	err := r.pool.QueryRow(ctx, `
		SELECT id, run_id, module_id, act_id, task_id, attempt_count,
		       first_passed_at, is_locked, last_attempted_at, xp_awarded, check_results
		FROM public.task_completions
		WHERE run_id = $1 AND task_id = $2
	`, runID, taskID).Scan(&tc.ID, &tc.RunID, &tc.ModuleID, &tc.ActID, &tc.TaskID,
		&tc.AttemptCount, &tc.FirstPassedAt, &tc.IsLocked,
		&tc.LastAttemptedAt, &tc.XPAwarded, &checkResultsJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if checkResultsJSON != nil {
		json.Unmarshal(checkResultsJSON, &tc.CheckResults)
	}
	return tc, nil
}

func (r *VerificationRepo) IsLocked(ctx context.Context, runID uuid.UUID, taskID string) (bool, error) {
	var isLocked bool
	err := r.pool.QueryRow(ctx, `
		SELECT is_locked FROM public.task_completions
		WHERE run_id = $1 AND task_id = $2
	`, runID, taskID).Scan(&isLocked)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil // no row means not started
	}
	return isLocked, err
}

func (r *VerificationRepo) RecordAttempt(ctx context.Context, runID uuid.UUID, moduleID, actID, taskID string, passed bool, results []domain.CheckResult) error {
	resultsJSON, err := json.Marshal(results)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO public.task_completions
		  (run_id, module_id, act_id, task_id, attempt_count, first_passed_at,
		   last_attempted_at, check_results)
		VALUES ($1, $2, $3, $4, 1,
		        CASE WHEN $5 THEN now() ELSE NULL END,
		        now(), $6)
		ON CONFLICT (run_id, task_id) DO UPDATE SET
		  attempt_count     = task_completions.attempt_count + 1,
		  first_passed_at   = COALESCE(task_completions.first_passed_at,
		                              CASE WHEN $5 THEN now() ELSE NULL END),
		  last_attempted_at = now(),
		  check_results     = $6
	`, runID, moduleID, actID, taskID, passed, resultsJSON)
	return err
}

func (r *VerificationRepo) LockTask(ctx context.Context, runID uuid.UUID, taskID string, xpAwarded int) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE public.task_completions
		SET is_locked = true, xp_awarded = $1
		WHERE run_id = $2 AND task_id = $3
	`, xpAwarded, runID, taskID)
	return err
}

func (r *VerificationRepo) ListCompletedTasks(ctx context.Context, runID uuid.UUID, moduleID string) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT task_id FROM public.task_completions
		WHERE run_id = $1 AND module_id = $2 AND is_locked = true
	`, runID, moduleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
