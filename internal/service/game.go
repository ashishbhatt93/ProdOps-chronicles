package service

import (
	"context"
	"fmt"
	"strconv"

	"github.com/google/uuid"
	appconfig "github.com/prodops-chronicles/prodops/internal/config"
	"github.com/prodops-chronicles/prodops/internal/content"
	"github.com/prodops-chronicles/prodops/internal/domain"
	"github.com/prodops-chronicles/prodops/internal/repository"
	"github.com/prodops-chronicles/prodops/internal/uow"
)

type GameService struct {
	progress     repository.ProgressRepository
	decisions    repository.DecisionRepository
	modules      repository.ModuleRepository
	scores       repository.ScoreRepository
	modSvc       *ModuleService
	uowFac       uow.Factory
	cfg          *appconfig.Config
	contentCache map[string]*content.ModuleContent
}

func NewGameService(
	progress repository.ProgressRepository,
	decisions repository.DecisionRepository,
	modules repository.ModuleRepository,
	scores repository.ScoreRepository,
	modSvc *ModuleService,
	uowFac uow.Factory,
	cfg *appconfig.Config,
) *GameService {
	return &GameService{
		progress:     progress,
		decisions:    decisions,
		modules:      modules,
		scores:       scores,
		modSvc:       modSvc,
		uowFac:       uowFac,
		cfg:          cfg,
		contentCache: make(map[string]*content.ModuleContent),
	}
}

// FilteredHints returns only the hints accessible at the current difficulty level.
// D1 = all hints, D2 = first 2, D3 = first hint only.
func (s *GameService) FilteredHints(hints []string) []string {
	max := s.cfg.MaxHintLevel()
	if max >= len(hints) {
		return hints
	}
	return hints[:max]
}

// RegisterContent is called by the internal seed handler when a pod seeds its content.
func (s *GameService) RegisterContent(moduleID string, mc *content.ModuleContent) {
	s.contentCache[moduleID] = mc
}

// GetCurrentAct returns the act the learner is currently on.
func (s *GameService) GetCurrentAct(ctx context.Context, runID uuid.UUID, moduleID string) (*content.ActContent, *domain.ModuleProgress, error) {
	mp, err := s.progress.GetModuleProgress(ctx, runID, moduleID)
	if err != nil {
		return nil, nil, err
	}

	mc, ok := s.contentCache[moduleID]
	if !ok {
		return nil, nil, fmt.Errorf("content not loaded for module %s", moduleID)
	}

	// If no current act set, start at act 1
	if mp.CurrentActID == nil {
		firstAct := mc.Acts[0]
		if err := s.progress.SetCurrentAct(ctx, runID, moduleID, firstAct.ID); err != nil {
			return nil, nil, err
		}
		mp.CurrentActID = &firstAct.ID
		return firstAct, mp, nil
	}

	for _, act := range mc.Acts {
		if act.ID == *mp.CurrentActID {
			return act, mp, nil
		}
	}
	return nil, nil, fmt.Errorf("act %s not found in module %s", *mp.CurrentActID, moduleID)
}

// MakeDecision atomically: UpdateTrackers + RecordDecision + AppendCompletedAct.
// If all acts are done, triggers resolveEnding.
func (s *GameService) MakeDecision(ctx context.Context, runID uuid.UUID, moduleID, actID, optionID string) (*DecisionResult, error) {
	mc, ok := s.contentCache[moduleID]
	if !ok {
		return nil, fmt.Errorf("content not loaded for module %s", moduleID)
	}

	act := findAct(mc, actID)
	if act == nil {
		return nil, fmt.Errorf("%w: act %s not found", domain.ErrNotFound, actID)
	}
	if act.Decision == nil {
		return nil, fmt.Errorf("%w: act %s has no decision", domain.ErrInvalidInput, actID)
	}

	opt := findOption(act, optionID)
	if opt == nil {
		return nil, fmt.Errorf("%w: option %s not found", domain.ErrNotFound, optionID)
	}

	mp, err := s.progress.GetModuleProgress(ctx, runID, moduleID)
	if err != nil {
		return nil, err
	}

	// Build tracker delta
	delta := domain.TrackerState{
		Morale:           opt.TrackerEffects.MoraleDelta,
		TechnicalDebt:    opt.TrackerEffects.DebtDelta,
		IncidentSeverity: opt.TrackerEffects.SeverityChange,
	}

	u, err := s.uowFac.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer u.Rollback(ctx)

	ex := u.Executor()

	// 1. Update trackers
	if err := s.progress.UpdateTrackers(ctx, ex, runID, moduleID, delta); err != nil {
		return nil, err
	}

	// 2. Record decision + snapshot
	d := &domain.DecisionMade{
		RunID:          runID,
		ModuleID:       moduleID,
		ActID:          actID,
		OptionID:       optionID,
		MoraleDelta:    opt.TrackerEffects.MoraleDelta,
		DebtDelta:      opt.TrackerEffects.DebtDelta,
		SeverityChange: nilIfEmpty(opt.TrackerEffects.SeverityChange),
	}

	// Compute new tracker state for snapshot
	newState := domain.TrackerState{
		Morale:           clampMorale(mp.Morale + opt.TrackerEffects.MoraleDelta),
		TechnicalDebt:    max(0, mp.TechnicalDebt+opt.TrackerEffects.DebtDelta),
		IncidentSeverity: coalesceString(opt.TrackerEffects.SeverityChange, mp.IncidentSeverity),
	}

	decision, err := s.decisions.RecordDecision(ctx, ex, d, newState)
	if err != nil {
		return nil, err
	}

	// 3. Append completed act
	if err := s.progress.AppendCompletedAct(ctx, ex, runID, moduleID, actID); err != nil {
		return nil, err
	}

	// 4. Add yearly review flag if present
	if opt.YearlyFlag != nil {
		flag := &domain.YearlyReviewFlag{
			RunID:         runID,
			ModuleID:      moduleID,
			FlagID:        opt.YearlyFlag.ID,
			Severity:      domain.FlagSeverity(opt.YearlyFlag.Severity),
			Note:          opt.YearlyFlag.Note,
			CanBeOffsetBy: opt.YearlyFlag.CanBeOffsetBy,
		}
		if err := s.scores.AddFlag(ctx, ex, runID, flag); err != nil {
			return nil, err
		}
	}

	if err := u.Commit(ctx); err != nil {
		return nil, err
	}

	result := &DecisionResult{
		Decision:   decision,
		Narrative:  opt.Narrative,
		NewTracker: newState,
	}

	// Determine next act
	nextAct := findNextAct(mc, actID, mp.CompletedActs)
	if nextAct != nil {
		if err := s.progress.SetCurrentAct(ctx, runID, moduleID, nextAct.ID); err != nil {
			return nil, err
		}
		result.NextActID = nextAct.ID
	} else {
		// All acts done — resolve ending
		ending, err := s.resolveEnding(ctx, runID, moduleID, newState, mp.ReplayCount+1)
		if err != nil {
			return nil, err
		}
		result.Ending = ending
	}

	return result, nil
}

// resolveEnding atomically: CompleteModule + AppendScore + AddFlags + UnlockNext + RecordCompletion.
func (s *GameService) resolveEnding(ctx context.Context, runID uuid.UUID, moduleID string, trackerState domain.TrackerState, attemptNum int) (*content.EndingContent, error) {
	mc := s.contentCache[moduleID]
	ending := evaluateEnding(trackerState, mc.Endings)
	if ending == nil {
		// Fallback to last ending if none match
		ending = mc.Endings[len(mc.Endings)-1]
	}

	mod, err := s.modules.GetModule(ctx, moduleID)
	if err != nil {
		return nil, err
	}

	// Score = base threshold + performance delta
	finalScore := mod.ScoreThreshold + ending.PerformanceDelta

	u, err := s.uowFac.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer u.Rollback(ctx)

	ex := u.Executor()

	if err := s.progress.CompleteModule(ctx, ex, runID, moduleID, ending.ID, finalScore); err != nil {
		return nil, err
	}
	if err := s.scores.AppendScore(ctx, ex, runID, moduleID, ending.PerformanceDelta,
		fmt.Sprintf("module %s completed: %s", moduleID, ending.ID)); err != nil {
		return nil, err
	}
	if err := s.scores.CheckAndOffsetFlags(ctx, runID, moduleID); err != nil {
		return nil, err
	}
	if err := s.modSvc.UnlockNextModule(ctx, ex, runID, moduleID); err != nil {
		return nil, err
	}
	if err := s.progress.RecordCompletion(ctx, ex, runID, moduleID, ending.ID,
		attemptNum, finalScore, trackerState); err != nil {
		return nil, err
	}

	return ending, u.Commit(ctx)
}

// evaluateEnding is a pure function — no DB, no IO.
// First ending whose ALL conditions pass wins.
func evaluateEnding(state domain.TrackerState, endings []*content.EndingContent) *content.EndingContent {
	for _, e := range endings {
		if allConditionsMet(state, e.Conditions) {
			return e
		}
	}
	return nil
}

func allConditionsMet(state domain.TrackerState, conditions []content.EndingCondition) bool {
	for _, c := range conditions {
		if !conditionMet(state, c) {
			return false
		}
	}
	return true
}

func conditionMet(state domain.TrackerState, c content.EndingCondition) bool {
	switch c.Field {
	case "morale":
		val, err := strconv.Atoi(c.Value)
		if err != nil {
			return false
		}
		return compareInt(state.Morale, c.Op, val)
	case "technical_debt":
		val, err := strconv.Atoi(c.Value)
		if err != nil {
			return false
		}
		return compareInt(state.TechnicalDebt, c.Op, val)
	case "incident_severity":
		return c.Op == "=" && state.IncidentSeverity == c.Value
	}
	return false
}

func compareInt(a int, op string, b int) bool {
	switch op {
	case ">=":
		return a >= b
	case "<=":
		return a <= b
	case ">":
		return a > b
	case "<":
		return a < b
	case "=":
		return a == b
	}
	return false
}

// ReplayModule resets progress for a fresh attempt.
// Tracker starting values are inherited from previous ending's tracker state.
func (s *GameService) ReplayModule(ctx context.Context, runID uuid.UUID, moduleID string) error {
	lastCompletion, err := s.progress.GetLatestCompletion(ctx, runID, moduleID)
	if err != nil {
		return fmt.Errorf("no previous completion found for %s: %w", moduleID, err)
	}

	if err := s.progress.ResetForReplay(ctx, runID, moduleID); err != nil {
		return err
	}
	if err := s.progress.IncrementReplayCount(ctx, runID, moduleID); err != nil {
		return err
	}

	mc := s.contentCache[moduleID]
	firstActID := mc.Acts[0].ID

	// Seed tracker from last completion's ending tracker state
	_, err = s.progress.StartModule(ctx, runID, moduleID, firstActID)
	if err != nil {
		return err
	}

	// Apply inherited tracker values from previous ending
	inherited := domain.TrackerState{
		Morale:           lastCompletion.TrackerState.Morale,
		TechnicalDebt:    lastCompletion.TrackerState.TechnicalDebt,
		IncidentSeverity: lastCompletion.TrackerState.IncidentSeverity,
	}
	return s.progress.UpdateTrackers(ctx, nil, runID, moduleID, inherited)
}

// ── helpers ───────────────────────────────────────────────────────────────────

type DecisionResult struct {
	Decision   *domain.DecisionMade
	Narrative  string
	NewTracker domain.TrackerState
	NextActID  string
	Ending     *content.EndingContent
}

func findAct(mc *content.ModuleContent, actID string) *content.ActContent {
	for _, a := range mc.Acts {
		if a.ID == actID {
			return a
		}
	}
	return nil
}

func findOption(act *content.ActContent, optionID string) *content.OptionContent {
	if act.Decision == nil {
		return nil
	}
	for _, o := range act.Decision.Options {
		if o.ID == optionID {
			return o
		}
	}
	return nil
}

func findNextAct(mc *content.ModuleContent, currentActID string, completedActs []string) *content.ActContent {
	completedSet := make(map[string]bool, len(completedActs))
	for _, id := range completedActs {
		completedSet[id] = true
	}
	completedSet[currentActID] = true

	for _, act := range mc.Acts {
		if !completedSet[act.ID] {
			return act
		}
	}
	return nil
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func coalesceString(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func clampMorale(v int) int {
	if v < 0 {
		return 0
	}
	if v > 200 {
		return 200
	}
	return v
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
