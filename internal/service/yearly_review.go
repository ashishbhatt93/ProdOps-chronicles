package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/prodops-chronicles/prodops/internal/domain"
	"github.com/prodops-chronicles/prodops/internal/repository"
)

type YearlyReviewService struct {
	scores   repository.ScoreRepository
	progress repository.ProgressRepository
	modules  repository.ModuleRepository
}

func NewYearlyReviewService(
	scores repository.ScoreRepository,
	progress repository.ProgressRepository,
	modules repository.ModuleRepository,
) *YearlyReviewService {
	return &YearlyReviewService{scores: scores, progress: progress, modules: modules}
}

type YearlyReview struct {
	PerformanceScore int
	Flags            []*domain.YearlyReviewFlag
	CompletedModules []string
	IsAvailable      bool
}

// GetYearlyReview returns the full yearly review state for a run.
func (s *YearlyReviewService) GetYearlyReview(ctx context.Context, runID uuid.UUID) (*YearlyReview, error) {
	score, err := s.scores.GetRunScore(ctx, runID)
	if err != nil {
		return nil, err
	}

	flags, err := s.scores.GetFlags(ctx, runID)
	if err != nil {
		return nil, err
	}

	allProgress, err := s.progress.ListModuleProgress(ctx, runID)
	if err != nil {
		return nil, err
	}

	var completed []string
	for _, mp := range allProgress {
		if mp.Status == domain.ModuleStatusCompleted {
			completed = append(completed, mp.ModuleID)
		}
	}

	return &YearlyReview{
		PerformanceScore: score,
		Flags:            flags,
		CompletedModules: completed,
		IsAvailable:      s.isYearlyReviewAvailable(allProgress),
	}, nil
}

// IsYearlyReviewAvailable returns true once all v1.0 modules are complete.
func (s *YearlyReviewService) IsYearlyReviewAvailable(ctx context.Context, runID uuid.UUID) (bool, error) {
	allProgress, err := s.progress.ListModuleProgress(ctx, runID)
	if err != nil {
		return false, err
	}
	return s.isYearlyReviewAvailable(allProgress), nil
}

func (s *YearlyReviewService) isYearlyReviewAvailable(allProgress []*domain.ModuleProgress) bool {
	if len(allProgress) == 0 {
		return false
	}
	for _, mp := range allProgress {
		if mp.Status != domain.ModuleStatusCompleted {
			return false
		}
	}
	return true
}
