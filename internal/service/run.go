package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/prodops-chronicles/prodops/internal/domain"
	"github.com/prodops-chronicles/prodops/internal/repository"
	"github.com/prodops-chronicles/prodops/internal/uow"
)

type RunService struct {
	runs    repository.RunRepository
	players repository.PlayerRepository
	scores  repository.ScoreRepository
	uowFac  uow.Factory
}

func NewRunService(
	runs repository.RunRepository,
	players repository.PlayerRepository,
	scores repository.ScoreRepository,
	uowFac uow.Factory,
) *RunService {
	return &RunService{runs: runs, players: players, scores: scores, uowFac: uowFac}
}

func (s *RunService) StartRun(ctx context.Context) (*domain.Run, error) {
	player, err := s.players.GetIdentity(ctx)
	if err != nil {
		return nil, fmt.Errorf("player not set up: %w", err)
	}

	// Enforce one active run
	existing, err := s.runs.GetActiveRun(ctx, player.ID)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("%w: a run is already in progress", domain.ErrConflict)
	}

	run, err := s.runs.CreateRun(ctx, player.ID)
	if err != nil {
		return nil, err
	}

	// Update player's current_run_id
	if err := s.players.SetCurrentRun(ctx, player.ID, &run.ID); err != nil {
		return nil, err
	}
	return run, nil
}

func (s *RunService) GetActiveRun(ctx context.Context) (*domain.Run, error) {
	player, err := s.players.GetIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return s.runs.GetActiveRun(ctx, player.ID)
}

func (s *RunService) ListRuns(ctx context.Context) ([]*domain.Run, error) {
	player, err := s.players.GetIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return s.runs.ListRuns(ctx, player.ID)
}

// CompleteRun atomically: marks run complete + clears current_run_id.
func (s *RunService) CompleteRun(ctx context.Context, runID uuid.UUID) error {
	player, err := s.players.GetIdentity(ctx)
	if err != nil {
		return err
	}

	finalScore, err := s.scores.GetRunScore(ctx, runID)
	if err != nil {
		return err
	}

	u, err := s.uowFac.Begin(ctx)
	if err != nil {
		return err
	}
	defer u.Rollback(ctx)

	if err := s.runs.CompleteRun(ctx, u.Executor(), runID, finalScore); err != nil {
		return err
	}
	if err := s.players.SetCurrentRun(ctx, player.ID, nil); err != nil {
		return err
	}
	return u.Commit(ctx)
}
