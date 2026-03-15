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

type ModuleRepo struct{ pool *pgxpool.Pool }

func NewModuleRepo(pool *pgxpool.Pool) *ModuleRepo { return &ModuleRepo{pool: pool} }

func (r *ModuleRepo) SeedModules(ctx context.Context, modules []*domain.Module) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, m := range modules {
		_, err := tx.Exec(ctx, `
			INSERT INTO public.modules
			  (id, name, version, pod_name, order_index, mode, runtime,
			   description, requires_module_id, score_threshold)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
			ON CONFLICT (id) DO UPDATE SET
			  name = EXCLUDED.name, version = EXCLUDED.version,
			  pod_name = EXCLUDED.pod_name, order_index = EXCLUDED.order_index,
			  score_threshold = EXCLUDED.score_threshold
		`, m.ID, m.Name, m.Version, m.PodName, m.OrderIndex, m.Mode,
			m.Runtime, m.Description, m.RequiresModuleID, m.ScoreThreshold)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *ModuleRepo) GetModule(ctx context.Context, moduleID string) (*domain.Module, error) {
	m := &domain.Module{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, version, pod_name, order_index, mode, runtime,
		       description, requires_module_id, score_threshold
		FROM public.modules WHERE id = $1
	`, moduleID).Scan(&m.ID, &m.Name, &m.Version, &m.PodName, &m.OrderIndex,
		&m.Mode, &m.Runtime, &m.Description, &m.RequiresModuleID, &m.ScoreThreshold)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	return m, err
}

func (r *ModuleRepo) ListModules(ctx context.Context) ([]*domain.Module, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, version, pod_name, order_index, mode, runtime,
		       description, requires_module_id, score_threshold
		FROM public.modules ORDER BY order_index
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var modules []*domain.Module
	for rows.Next() {
		m := &domain.Module{}
		if err := rows.Scan(&m.ID, &m.Name, &m.Version, &m.PodName, &m.OrderIndex,
			&m.Mode, &m.Runtime, &m.Description, &m.RequiresModuleID, &m.ScoreThreshold); err != nil {
			return nil, err
		}
		modules = append(modules, m)
	}
	return modules, rows.Err()
}

func (r *ModuleRepo) ListAvailableModules(ctx context.Context, runID uuid.UUID) ([]*domain.ModuleListEntry, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT m.id, m.name, m.version, m.pod_name, m.order_index, m.mode,
		       m.runtime, m.description, m.requires_module_id, m.score_threshold,
		       u.id IS NOT NULL AS is_unlocked,
		       u.unlock_source,
		       mp.status, mp.current_act_id, mp.morale, mp.incident_severity,
		       mp.technical_debt, mp.xp_earned, mp.final_score, mp.replay_count
		FROM public.modules m
		LEFT JOIN public.module_unlocks u ON u.module_id = m.id AND u.run_id = $1
		LEFT JOIN public.module_progress mp ON mp.module_id = m.id AND mp.run_id = $1
		ORDER BY m.order_index
	`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*domain.ModuleListEntry
	for rows.Next() {
		e := &domain.ModuleListEntry{}
		var isUnlocked bool
		var unlockSource, mpStatus, currentActID *string
		var morale, technicalDebt, xpEarned, replayCount *int
		var incidentSeverity *string
		var finalScore *int

		if err := rows.Scan(
			&e.Module.ID, &e.Module.Name, &e.Module.Version, &e.Module.PodName,
			&e.Module.OrderIndex, &e.Module.Mode, &e.Module.Runtime,
			&e.Module.Description, &e.Module.RequiresModuleID, &e.Module.ScoreThreshold,
			&isUnlocked, &unlockSource,
			&mpStatus, &currentActID, &morale, &incidentSeverity,
			&technicalDebt, &xpEarned, &finalScore, &replayCount,
		); err != nil {
			return nil, err
		}

		if !isUnlocked {
			e.DisplayStatus = domain.ModuleDisplayLocked
			if e.Module.RequiresModuleID != nil {
				e.LockReason = "Complete " + *e.Module.RequiresModuleID + " first"
			}
		} else if mpStatus == nil || *mpStatus == "not_started" {
			e.DisplayStatus = domain.ModuleDisplayAvailable
		} else if *mpStatus == "in_progress" {
			e.DisplayStatus = domain.ModuleDisplayInProgress
		} else {
			e.DisplayStatus = domain.ModuleDisplayCompleted
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (r *ModuleRepo) IsUnlocked(ctx context.Context, runID uuid.UUID, moduleID string) (bool, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM public.module_unlocks
		WHERE run_id = $1 AND module_id = $2
	`, runID, moduleID).Scan(&count)
	return count > 0, err
}

func (r *ModuleRepo) UnlockModule(ctx context.Context, ex repository.Executor, runID uuid.UUID, moduleID string, source string) error {
	_, err := ex.Exec(ctx, `
		INSERT INTO public.module_unlocks (run_id, module_id, unlock_source)
		VALUES ($1, $2, $3)
		ON CONFLICT (run_id, module_id) DO NOTHING
	`, runID, moduleID, source)
	return err
}

func (r *ModuleRepo) GetUnlock(ctx context.Context, runID uuid.UUID, moduleID string) (*domain.ModuleUnlock, error) {
	u := &domain.ModuleUnlock{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, run_id, module_id, unlocked_at, unlock_source
		FROM public.module_unlocks WHERE run_id = $1 AND module_id = $2
	`, runID, moduleID).Scan(&u.ID, &u.RunID, &u.ModuleID, &u.UnlockedAt, &u.UnlockSource)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	return u, err
}
