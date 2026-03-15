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

type ProgressRepo struct{ pool *pgxpool.Pool }

func NewProgressRepo(pool *pgxpool.Pool) *ProgressRepo { return &ProgressRepo{pool: pool} }

func scanProgress(row repository.Row) (*domain.ModuleProgress, error) {
	mp := &domain.ModuleProgress{}
	err := row.Scan(
		&mp.ID, &mp.RunID, &mp.ModuleID, &mp.Status,
		&mp.CurrentActID, &mp.CompletedActs, &mp.CompletedTasks,
		&mp.Morale, &mp.IncidentSeverity, &mp.TechnicalDebt,
		&mp.XPEarned, &mp.FinalScore, &mp.EndingID, &mp.ReplayCount,
		&mp.StartedAt, &mp.CompletedAt, &mp.LastActiveAt,
	)
	return mp, err
}

const progressSelect = `
	SELECT id, run_id, module_id, status, current_act_id, completed_acts,
	       completed_tasks, morale, incident_severity, technical_debt,
	       xp_earned, final_score, ending_id, replay_count,
	       started_at, completed_at, last_active_at
	FROM public.module_progress`

func (r *ProgressRepo) GetModuleProgress(ctx context.Context, runID uuid.UUID, moduleID string) (*domain.ModuleProgress, error) {
	mp, err := scanProgress(r.pool.QueryRow(ctx,
		progressSelect+` WHERE run_id = $1 AND module_id = $2`, runID, moduleID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	return mp, err
}

func (r *ProgressRepo) ListModuleProgress(ctx context.Context, runID uuid.UUID) ([]*domain.ModuleProgress, error) {
	rows, err := r.pool.Query(ctx, progressSelect+` WHERE run_id = $1 ORDER BY last_active_at DESC`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*domain.ModuleProgress
	for rows.Next() {
		mp, err := scanProgress(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, mp)
	}
	return list, rows.Err()
}

func (r *ProgressRepo) StartModule(ctx context.Context, runID uuid.UUID, moduleID, firstActID string) (*domain.ModuleProgress, error) {
	mp, err := scanProgress(r.pool.QueryRow(ctx, `
		INSERT INTO public.module_progress
		  (run_id, module_id, status, current_act_id, started_at, last_active_at)
		VALUES ($1, $2, 'in_progress', $3, now(), now())
		ON CONFLICT (run_id, module_id) DO UPDATE
		  SET status = 'in_progress', current_act_id = $3,
		      started_at = COALESCE(module_progress.started_at, now()),
		      last_active_at = now()
		RETURNING `+progressSelect[7:], // trim "SELECT "
		runID, moduleID, firstActID))
	if err != nil {
		return nil, err
	}
	return mp, nil
}

func (r *ProgressRepo) SetCurrentAct(ctx context.Context, runID uuid.UUID, moduleID, actID string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE public.module_progress
		SET current_act_id = $1, last_active_at = now()
		WHERE run_id = $2 AND module_id = $3
	`, actID, runID, moduleID)
	return err
}

func (r *ProgressRepo) AppendCompletedAct(ctx context.Context, ex repository.Executor, runID uuid.UUID, moduleID, actID string) error {
	_, err := ex.Exec(ctx, `
		UPDATE public.module_progress
		SET completed_acts = array_append(completed_acts, $1), last_active_at = now()
		WHERE run_id = $2 AND module_id = $3
	`, actID, runID, moduleID)
	return err
}

func (r *ProgressRepo) AppendCompletedTask(ctx context.Context, ex repository.Executor, runID uuid.UUID, moduleID, taskID string) error {
	_, err := ex.Exec(ctx, `
		UPDATE public.module_progress
		SET completed_tasks = array_append(completed_tasks, $1), last_active_at = now()
		WHERE run_id = $2 AND module_id = $3
	`, taskID, runID, moduleID)
	return err
}

func (r *ProgressRepo) UpdateTrackers(ctx context.Context, ex repository.Executor, runID uuid.UUID, moduleID string, delta domain.TrackerState) error {
	_, err := ex.Exec(ctx, `
		UPDATE public.module_progress SET
		  morale           = GREATEST(0, LEAST(200, morale + $1)),
		  technical_debt   = GREATEST(0, technical_debt + $2),
		  incident_severity = CASE WHEN $3 != '' THEN $3 ELSE incident_severity END,
		  last_active_at   = now()
		WHERE run_id = $4 AND module_id = $5
	`, delta.Morale, delta.TechnicalDebt, delta.IncidentSeverity, runID, moduleID)
	return err
}

func (r *ProgressRepo) CompleteModule(ctx context.Context, ex repository.Executor, runID uuid.UUID, moduleID, endingID string, finalScore int) error {
	_, err := ex.Exec(ctx, `
		UPDATE public.module_progress SET
		  status = 'completed', ending_id = $1, final_score = $2,
		  completed_at = now(), last_active_at = now()
		WHERE run_id = $3 AND module_id = $4
	`, endingID, finalScore, runID, moduleID)
	return err
}

func (r *ProgressRepo) ResetForReplay(ctx context.Context, runID uuid.UUID, moduleID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Reset progress row
	_, err = tx.Exec(ctx, `
		UPDATE public.module_progress SET
		  status = 'in_progress', current_act_id = NULL,
		  completed_acts = '{}', completed_tasks = '{}',
		  morale = 100, incident_severity = 'P2', technical_debt = 0,
		  xp_earned = 0, final_score = NULL, ending_id = NULL,
		  completed_at = NULL, last_active_at = now()
		WHERE run_id = $1 AND module_id = $2
	`, runID, moduleID)
	if err != nil {
		return err
	}

	// Unlock task completions for re-verification
	_, err = tx.Exec(ctx, `
		UPDATE public.task_completions
		SET is_locked = false
		WHERE run_id = $1 AND module_id = $2
	`, runID, moduleID)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *ProgressRepo) IncrementReplayCount(ctx context.Context, runID uuid.UUID, moduleID string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE public.module_progress
		SET replay_count = replay_count + 1
		WHERE run_id = $1 AND module_id = $2
	`, runID, moduleID)
	return err
}

func (r *ProgressRepo) RecordCompletion(ctx context.Context, ex repository.Executor, runID uuid.UUID, moduleID, endingID string, attemptNum, score int, trackerState domain.TrackerState) error {
	stateJSON := map[string]any{
		"morale":            trackerState.Morale,
		"incident_severity": trackerState.IncidentSeverity,
		"technical_debt":    trackerState.TechnicalDebt,
	}
	_, err := ex.Exec(ctx, `
		INSERT INTO public.module_completions
		  (run_id, module_id, attempt_number, ending_id, final_score, tracker_state)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, runID, moduleID, attemptNum, endingID, score, stateJSON)
	return err
}

func (r *ProgressRepo) GetLatestCompletion(ctx context.Context, runID uuid.UUID, moduleID string) (*domain.ModuleCompletion, error) {
	mc := &domain.ModuleCompletion{}
	var stateJSON map[string]any
	err := r.pool.QueryRow(ctx, `
		SELECT id, run_id, module_id, attempt_number, ending_id,
		       final_score, completed_at, tracker_state
		FROM public.module_completions
		WHERE run_id = $1 AND module_id = $2
		ORDER BY attempt_number DESC LIMIT 1
	`, runID, moduleID).Scan(&mc.ID, &mc.RunID, &mc.ModuleID, &mc.AttemptNumber,
		&mc.EndingID, &mc.FinalScore, &mc.CompletedAt, &stateJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if stateJSON != nil {
		if v, ok := stateJSON["morale"].(float64); ok {
			mc.TrackerState.Morale = int(v)
		}
		if v, ok := stateJSON["incident_severity"].(string); ok {
			mc.TrackerState.IncidentSeverity = v
		}
		if v, ok := stateJSON["technical_debt"].(float64); ok {
			mc.TrackerState.TechnicalDebt = int(v)
		}
	}
	return mc, nil
}

func (r *ProgressRepo) GetSetupMeta(ctx context.Context, runID uuid.UUID) (*domain.PlayerSetupMeta, error) {
	meta := &domain.PlayerSetupMeta{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, run_id, telemetry_consent, default_branch_name, updated_at
		FROM public.player_setup_meta WHERE run_id = $1
	`, runID).Scan(&meta.ID, &meta.RunID, &meta.TelemetryConsent,
		&meta.DefaultBranchName, &meta.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	return meta, err
}

func (r *ProgressRepo) SetTelemetryConsent(ctx context.Context, runID uuid.UUID, consent string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO public.player_setup_meta (run_id, telemetry_consent)
		VALUES ($1, $2)
		ON CONFLICT (run_id) DO UPDATE SET telemetry_consent = $2, updated_at = now()
	`, runID, consent)
	return err
}

func (r *ProgressRepo) SetDefaultBranchName(ctx context.Context, runID uuid.UUID, name string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO public.player_setup_meta (run_id, default_branch_name)
		VALUES ($1, $2)
		ON CONFLICT (run_id) DO UPDATE SET default_branch_name = $2, updated_at = now()
	`, runID, name)
	return err
}
