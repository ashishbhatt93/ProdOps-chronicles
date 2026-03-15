package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/prodops-chronicles/prodops/internal/domain"
	"github.com/prodops-chronicles/prodops/internal/repository"
)

type ProgressService struct {
	progress  repository.ProgressRepository
	decisions repository.DecisionRepository
	scores    repository.ScoreRepository
	modules   repository.ModuleRepository
}

func NewProgressService(
	progress repository.ProgressRepository,
	decisions repository.DecisionRepository,
	scores repository.ScoreRepository,
	modules repository.ModuleRepository,
) *ProgressService {
	return &ProgressService{
		progress:  progress,
		decisions: decisions,
		scores:    scores,
		modules:   modules,
	}
}

type RunSummary struct {
	Run            *domain.Run
	PerformScore   int
	XPTotal        int
	ModuleProgress []*domain.ModuleListEntry
	Flags          []*domain.YearlyReviewFlag
}

func (s *ProgressService) GetProgressSummary(ctx context.Context, runID uuid.UUID) (*RunSummary, error) {
	score, err := s.scores.GetRunScore(ctx, runID)
	if err != nil {
		return nil, err
	}

	modules, err := s.modules.ListAvailableModules(ctx, runID)
	if err != nil {
		return nil, err
	}

	flags, err := s.scores.GetFlags(ctx, runID)
	if err != nil {
		return nil, err
	}

	xpTotal := 0
	for _, m := range modules {
		if m.Progress != nil {
			xpTotal += m.Progress.XPEarned
		}
	}

	return &RunSummary{
		PerformScore:   score,
		XPTotal:        xpTotal,
		ModuleProgress: modules,
		Flags:          flags,
	}, nil
}

type ModuleDetail struct {
	Progress   *domain.ModuleProgress
	Decisions  []*domain.DecisionMade
	Snapshots  []*domain.TrackerSnapshot
	Completion *domain.ModuleCompletion
}

func (s *ProgressService) GetModuleDetail(ctx context.Context, runID uuid.UUID, moduleID string) (*ModuleDetail, error) {
	mp, err := s.progress.GetModuleProgress(ctx, runID, moduleID)
	if err != nil {
		return nil, err
	}

	decisions, err := s.decisions.GetDecisionHistory(ctx, runID, moduleID)
	if err != nil {
		return nil, err
	}

	snapshots, err := s.decisions.GetSnapshotsForModule(ctx, runID, moduleID)
	if err != nil {
		return nil, err
	}

	var completion *domain.ModuleCompletion
	completion, _ = s.progress.GetLatestCompletion(ctx, runID, moduleID) // ignore ErrNotFound

	return &ModuleDetail{
		Progress:   mp,
		Decisions:  decisions,
		Snapshots:  snapshots,
		Completion: completion,
	}, nil
}
