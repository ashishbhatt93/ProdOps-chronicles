package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	appconfig "github.com/prodops-chronicles/prodops/internal/config"
	"github.com/prodops-chronicles/prodops/internal/domain"
	"github.com/prodops-chronicles/prodops/internal/repository"
)

// runtimeAdapter is a local interface so this package doesn't import the
// concrete compose/k3s packages (dependency inversion).
type runtimeAdapter interface {
	StartModule(ctx context.Context, moduleID string) error
	StopModule(ctx context.Context, moduleID string) error
	WriteModuleDefinition(ctx context.Context, moduleID string) error
	RemoveModuleDefinition(ctx context.Context, moduleID string) error
}

type ModuleService struct {
	modules repository.ModuleRepository
	scores  repository.ScoreRepository
	rt      runtimeAdapter
	cfg     *appconfig.Config
}

func NewModuleService(
	modules repository.ModuleRepository,
	scores repository.ScoreRepository,
	rt runtimeAdapter,
	cfg *appconfig.Config,
) *ModuleService {
	return &ModuleService{modules: modules, scores: scores, rt: rt, cfg: cfg}
}

// GetModuleStatus returns the display entry for a single module.
func (s *ModuleService) GetModuleStatus(ctx context.Context, runID uuid.UUID, moduleID string) (*domain.ModuleListEntry, error) {
	entries, err := s.modules.ListAvailableModules(ctx, runID)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.Module.ID == moduleID {
			return e, nil
		}
	}
	return nil, domain.ErrNotFound
}

// ListModules returns all modules that are:
//  1. enabled in base_configs.yaml (enabled: true)
//  2. accessible at the player's difficulty level (min_difficulty <= current)
//
// Modules not meeting either condition are completely absent from the list —
// they are not shown as locked, they simply do not appear.
func (s *ModuleService) ListModules(ctx context.Context, runID uuid.UUID) ([]*domain.ModuleListEntry, error) {
	all, err := s.modules.ListAvailableModules(ctx, runID)
	if err != nil {
		return nil, err
	}

	var visible []*domain.ModuleListEntry
	for _, entry := range all {
		if s.cfg.IsModuleEnabled(entry.Module.ID) {
			visible = append(visible, entry)
		}
	}
	return visible, nil
}

// EnableModule:
//  - Layer 1: base_configs.yaml must have enabled: true for this module
//    AND the player's difficulty must meet min_difficulty.
//  - Layer 2: DB must show the module is unlocked for this run.
//
// Both layers must pass. Failing Layer 1 returns ErrLocked with a message
// explaining whether the module is disabled or requires a higher difficulty.
func (s *ModuleService) EnableModule(ctx context.Context, runID uuid.UUID, moduleID string) error {
	// Layer 1 — base_configs gate
	modCfg, ok := s.cfg.Modules[moduleID]
	if !ok || !modCfg.Enabled {
		return fmt.Errorf("%w: module %s is not enabled in base_configs.yaml", domain.ErrLocked, moduleID)
	}
	if !s.cfg.IsModuleEnabled(moduleID) {
		return fmt.Errorf("%w: module %s requires difficulty %s (current: %s — %s)",
			domain.ErrLocked, moduleID,
			appconfig.DifficultyLabel(modCfg.MinDifficulty),
			string(s.cfg.Difficulty),
			s.cfg.DifficultyLabel(),
		)
	}

	// Layer 2 — DB unlock gate
	unlocked, err := s.modules.IsUnlocked(ctx, runID, moduleID)
	if err != nil {
		return err
	}
	if !unlocked {
		return fmt.Errorf("%w: module %s has not been unlocked yet — complete the prerequisite module", domain.ErrLocked, moduleID)
	}

	if err := s.rt.WriteModuleDefinition(ctx, moduleID); err != nil {
		return fmt.Errorf("write module definition: %w", err)
	}
	if err := s.rt.StartModule(ctx, moduleID); err != nil {
		return fmt.Errorf("start module pod: %w", err)
	}
	return nil
}

// DisableModule stops the pod and removes its compose definition.
func (s *ModuleService) DisableModule(ctx context.Context, moduleID string) error {
	if err := s.rt.StopModule(ctx, moduleID); err != nil {
		return fmt.Errorf("stop module pod: %w", err)
	}
	return s.rt.RemoveModuleDefinition(ctx, moduleID)
}

// UnlockNextModule is called by the game service after a module completes.
// It only unlocks modules that are enabled and accessible at the current difficulty.
func (s *ModuleService) UnlockNextModule(ctx context.Context, ex repository.Executor, runID uuid.UUID, completedModuleID string) error {
	modules, err := s.modules.ListModules(ctx)
	if err != nil {
		return err
	}
	for _, m := range modules {
		if m.RequiresModuleID != nil && *m.RequiresModuleID == completedModuleID {
			// Only unlock if this module is enabled and reachable at this difficulty
			if !s.cfg.IsModuleEnabled(m.ID) {
				continue
			}
			return s.modules.UnlockModule(ctx, ex, runID, m.ID, "earned")
		}
	}
	return nil // no dependent module enabled at this difficulty
}

// ForceUnlockModule bypasses the prerequisite check.
// Records a yearly review flag regardless.
func (s *ModuleService) ForceUnlockModule(ctx context.Context, runID uuid.UUID, moduleID string) error {
	// Still enforce the base_configs enabled + difficulty gate —
	// --force skips prerequisites, not the installation configuration.
	if !s.cfg.IsModuleEnabled(moduleID) {
		modCfg, ok := s.cfg.Modules[moduleID]
		if !ok || !modCfg.Enabled {
			return fmt.Errorf("%w: module %s is not enabled in base_configs.yaml — edit the file and restart the backend", domain.ErrLocked, moduleID)
		}
		return fmt.Errorf("%w: module %s requires difficulty %s — update base_configs.yaml difficulty to unlock it",
			domain.ErrLocked, moduleID,
			appconfig.DifficultyLabel(modCfg.MinDifficulty))
	}

	existing, _ := s.modules.GetUnlock(ctx, runID, moduleID)
	if existing != nil {
		return fmt.Errorf("%w: module already unlocked", domain.ErrConflict)
	}

	if err := s.modules.UnlockModule(ctx, nil, runID, moduleID, "forced"); err != nil {
		return err
	}

	return s.scores.AddFlag(ctx, nil, runID, &domain.YearlyReviewFlag{
		RunID:    runID,
		ModuleID: moduleID,
		FlagID:   fmt.Sprintf("force-unlock-%s", moduleID),
		Severity: domain.FlagSeverityMinor,
		Note:     fmt.Sprintf("Module %s unlocked with --force flag", moduleID),
	})
}

// ScoreThreshold returns the effective score threshold for a module,
// preferring any override in base_configs.yaml over the module YAML default.
func (s *ModuleService) ScoreThreshold(moduleID string, yamlDefault int) int {
	return s.cfg.ScoreThreshold(moduleID, yamlDefault)
}
