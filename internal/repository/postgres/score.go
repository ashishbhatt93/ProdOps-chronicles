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

type ScoreRepo struct{ pool *pgxpool.Pool }

func NewScoreRepo(pool *pgxpool.Pool) *ScoreRepo { return &ScoreRepo{pool: pool} }

func (r *ScoreRepo) AppendScore(ctx context.Context, ex repository.Executor, runID uuid.UUID, moduleID string, delta int, reason string) error {
	_, err := ex.Exec(ctx, `
		INSERT INTO public.performance_scores (run_id, module_id, delta, reason)
		VALUES ($1, $2, $3, $4)
	`, runID, moduleID, delta, reason)
	return err
}

func (r *ScoreRepo) GetRunScore(ctx context.Context, runID uuid.UUID) (int, error) {
	var score *int
	err := r.pool.QueryRow(ctx, `
		SELECT SUM(delta) FROM public.performance_scores WHERE run_id = $1
	`, runID).Scan(&score)
	if err != nil {
		return 0, err
	}
	if score == nil {
		return 0, nil
	}
	return *score, nil
}

func (r *ScoreRepo) GetScoreHistory(ctx context.Context, runID uuid.UUID) ([]*domain.PerformanceScore, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, run_id, module_id, delta, reason, recorded_at
		FROM public.performance_scores WHERE run_id = $1 ORDER BY recorded_at
	`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*domain.PerformanceScore
	for rows.Next() {
		ps := &domain.PerformanceScore{}
		if err := rows.Scan(&ps.ID, &ps.RunID, &ps.ModuleID, &ps.Delta,
			&ps.Reason, &ps.RecordedAt); err != nil {
			return nil, err
		}
		list = append(list, ps)
	}
	return list, rows.Err()
}

func (r *ScoreRepo) AddFlag(ctx context.Context, ex repository.Executor, runID uuid.UUID, f *domain.YearlyReviewFlag) error {
	offsetBy, _ := json.Marshal(f.CanBeOffsetBy)
	_, err := ex.Exec(ctx, `
		INSERT INTO public.yearly_review_flags
		  (run_id, module_id, flag_id, severity, note, can_be_offset_by)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (run_id, flag_id) DO NOTHING
	`, runID, f.ModuleID, f.FlagID, string(f.Severity), f.Note, offsetBy)
	return err
}

func (r *ScoreRepo) OffsetFlag(ctx context.Context, runID uuid.UUID, flagID, offsetByModuleID string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE public.yearly_review_flags
		SET is_offset = true, offset_by_module_id = $1
		WHERE run_id = $2 AND flag_id = $3
	`, offsetByModuleID, runID, flagID)
	return err
}

func (r *ScoreRepo) GetFlags(ctx context.Context, runID uuid.UUID) ([]*domain.YearlyReviewFlag, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, run_id, module_id, flag_id, severity, note,
		       can_be_offset_by, is_offset, offset_by_module_id, created_at
		FROM public.yearly_review_flags WHERE run_id = $1 ORDER BY created_at
	`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*domain.YearlyReviewFlag
	for rows.Next() {
		f := &domain.YearlyReviewFlag{}
		var canOffsetJSON []byte
		if err := rows.Scan(&f.ID, &f.RunID, &f.ModuleID, &f.FlagID, &f.Severity,
			&f.Note, &canOffsetJSON, &f.IsOffset, &f.OffsetByModuleID, &f.CreatedAt); err != nil {
			return nil, err
		}
		if canOffsetJSON != nil {
			json.Unmarshal(canOffsetJSON, &f.CanBeOffsetBy)
		}
		list = append(list, f)
	}
	return list, rows.Err()
}

func (r *ScoreRepo) CheckAndOffsetFlags(ctx context.Context, runID uuid.UUID, completedModuleID string) error {
	flags, err := r.GetFlags(ctx, runID)
	if err != nil {
		return err
	}
	for _, f := range flags {
		if f.IsOffset {
			continue
		}
		for _, offsetBy := range f.CanBeOffsetBy {
			if offsetBy == completedModuleID {
				if err := r.OffsetFlag(ctx, runID, f.FlagID, completedModuleID); err != nil {
					return err
				}
				break
			}
		}
	}
	return nil
}

// GetFlag returns a single flag by flagID, used for yearly review display.
func (r *ScoreRepo) GetFlag(ctx context.Context, runID uuid.UUID, flagID string) (*domain.YearlyReviewFlag, error) {
	f := &domain.YearlyReviewFlag{}
	var canOffsetJSON []byte
	err := r.pool.QueryRow(ctx, `
		SELECT id, run_id, module_id, flag_id, severity, note,
		       can_be_offset_by, is_offset, offset_by_module_id, created_at
		FROM public.yearly_review_flags WHERE run_id = $1 AND flag_id = $2
	`, runID, flagID).Scan(&f.ID, &f.RunID, &f.ModuleID, &f.FlagID, &f.Severity,
		&f.Note, &canOffsetJSON, &f.IsOffset, &f.OffsetByModuleID, &f.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if canOffsetJSON != nil {
		json.Unmarshal(canOffsetJSON, &f.CanBeOffsetBy)
	}
	return f, nil
}
