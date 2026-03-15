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

type DecisionRepo struct{ pool *pgxpool.Pool }

func NewDecisionRepo(pool *pgxpool.Pool) *DecisionRepo { return &DecisionRepo{pool: pool} }

// RecordDecision writes decision + snapshot atomically using the provided executor
// (which must be a transaction from the Unit of Work).
func (r *DecisionRepo) RecordDecision(ctx context.Context, ex repository.Executor, d *domain.DecisionMade, snapshot domain.TrackerState) (*domain.DecisionMade, error) {
	result := &domain.DecisionMade{}
	row := ex.QueryRow(ctx, `
		INSERT INTO public.decisions_made
		  (run_id, module_id, act_id, option_id, morale_delta, severity_change, debt_delta)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, run_id, module_id, act_id, option_id, decided_at,
		          morale_delta, severity_change, debt_delta
	`, d.RunID, d.ModuleID, d.ActID, d.OptionID,
		d.MoraleDelta, d.SeverityChange, d.DebtDelta)
	err := row.Scan(&result.ID, &result.RunID, &result.ModuleID, &result.ActID,
		&result.OptionID, &result.DecidedAt, &result.MoraleDelta,
		&result.SeverityChange, &result.DebtDelta)
	if err != nil {
		return nil, err
	}

	stateJSON := map[string]any{
		"morale":            snapshot.Morale,
		"incident_severity": snapshot.IncidentSeverity,
		"technical_debt":    snapshot.TechnicalDebt,
	}
	_, err = ex.Exec(ctx, `
		INSERT INTO public.tracker_snapshots
		  (run_id, module_id, act_id, decision_id, state)
		VALUES ($1,$2,$3,$4,$5)
	`, d.RunID, d.ModuleID, d.ActID, result.ID, stateJSON)
	return result, err
}

func (r *DecisionRepo) GetDecisionHistory(ctx context.Context, runID uuid.UUID, moduleID string) ([]*domain.DecisionMade, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, run_id, module_id, act_id, option_id, decided_at,
		       morale_delta, severity_change, debt_delta
		FROM public.decisions_made
		WHERE run_id = $1 AND module_id = $2
		ORDER BY decided_at
	`, runID, moduleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDecisions(rows)
}

func (r *DecisionRepo) GetFullRunHistory(ctx context.Context, runID uuid.UUID) ([]*domain.DecisionMade, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, run_id, module_id, act_id, option_id, decided_at,
		       morale_delta, severity_change, debt_delta
		FROM public.decisions_made WHERE run_id = $1 ORDER BY decided_at
	`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDecisions(rows)
}

func scanDecisions(rows pgx.Rows) ([]*domain.DecisionMade, error) {
	var list []*domain.DecisionMade
	for rows.Next() {
		d := &domain.DecisionMade{}
		if err := rows.Scan(&d.ID, &d.RunID, &d.ModuleID, &d.ActID, &d.OptionID,
			&d.DecidedAt, &d.MoraleDelta, &d.SeverityChange, &d.DebtDelta); err != nil {
			return nil, err
		}
		list = append(list, d)
	}
	return list, rows.Err()
}

func (r *DecisionRepo) GetSnapshotAt(ctx context.Context, decisionID uuid.UUID) (*domain.TrackerSnapshot, error) {
	snap := &domain.TrackerSnapshot{}
	var stateJSON map[string]any
	err := r.pool.QueryRow(ctx, `
		SELECT id, run_id, module_id, act_id, decision_id, state, snapshot_taken_at
		FROM public.tracker_snapshots WHERE decision_id = $1
	`, decisionID).Scan(&snap.ID, &snap.RunID, &snap.ModuleID, &snap.ActID,
		&snap.DecisionID, &stateJSON, &snap.SnapshotTakenAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	snap.State = jsonToTracker(stateJSON)
	return snap, nil
}

func (r *DecisionRepo) GetSnapshotsForModule(ctx context.Context, runID uuid.UUID, moduleID string) ([]*domain.TrackerSnapshot, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, run_id, module_id, act_id, decision_id, state, snapshot_taken_at
		FROM public.tracker_snapshots
		WHERE run_id = $1 AND module_id = $2 ORDER BY snapshot_taken_at
	`, runID, moduleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*domain.TrackerSnapshot
	for rows.Next() {
		snap := &domain.TrackerSnapshot{}
		var stateJSON map[string]any
		if err := rows.Scan(&snap.ID, &snap.RunID, &snap.ModuleID, &snap.ActID,
			&snap.DecisionID, &stateJSON, &snap.SnapshotTakenAt); err != nil {
			return nil, err
		}
		snap.State = jsonToTracker(stateJSON)
		list = append(list, snap)
	}
	return list, rows.Err()
}

func jsonToTracker(m map[string]any) domain.TrackerState {
	ts := domain.TrackerState{}
	if v, ok := m["morale"].(float64); ok {
		ts.Morale = int(v)
	}
	if v, ok := m["incident_severity"].(string); ok {
		ts.IncidentSeverity = v
	}
	if v, ok := m["technical_debt"].(float64); ok {
		ts.TechnicalDebt = int(v)
	}
	return ts
}
