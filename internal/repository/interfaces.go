package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/prodops-chronicles/prodops/internal/domain"
)

// Executor is satisfied by both *pgxpool.Pool and pgx.Tx.
// All postgres repository methods accept this interface so they work
// inside or outside a transaction (Unit of Work pattern).
type Executor interface {
	Exec(ctx context.Context, sql string, args ...any) (interface{ RowsAffected() int64 }, error)
	Query(ctx context.Context, sql string, args ...any) (Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) Row
}

type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Close()
	Err() error
}

type Row interface {
	Scan(dest ...any) error
}

// ── PlayerRepository ──────────────────────────────────────────────────────────

type PlayerRepository interface {
	GetIdentity(ctx context.Context) (*domain.PlayerIdentity, error)
	CreateIdentity(ctx context.Context, displayName string) (*domain.PlayerIdentity, error)
	SetCurrentRun(ctx context.Context, playerID uuid.UUID, runID *uuid.UUID) error

	GetProfile(ctx context.Context, playerID uuid.UUID) (*domain.PlayerProfile, error)
	SetGitUsername(ctx context.Context, playerID uuid.UUID, username, email string) error
	SetSSHKeyPath(ctx context.Context, playerID uuid.UUID, path string) error
	SetSyncRemote(ctx context.Context, playerID uuid.UUID, remote string) error
	SetTelemetryConsent(ctx context.Context, playerID uuid.UUID, consent domain.TelemetryConsent) error
}

// ── RunRepository ─────────────────────────────────────────────────────────────

type RunRepository interface {
	CreateRun(ctx context.Context, playerID uuid.UUID) (*domain.Run, error)
	GetActiveRun(ctx context.Context, playerID uuid.UUID) (*domain.Run, error)
	GetRun(ctx context.Context, runID uuid.UUID) (*domain.Run, error)
	ListRuns(ctx context.Context, playerID uuid.UUID) ([]*domain.Run, error)
	CompleteRun(ctx context.Context, ex Executor, runID uuid.UUID, finalScore int) error
}

// ── ModuleRepository ──────────────────────────────────────────────────────────

type ModuleRepository interface {
	SeedModules(ctx context.Context, modules []*domain.Module) error
	GetModule(ctx context.Context, moduleID string) (*domain.Module, error)
	ListModules(ctx context.Context) ([]*domain.Module, error)
	ListAvailableModules(ctx context.Context, runID uuid.UUID) ([]*domain.ModuleListEntry, error)

	IsUnlocked(ctx context.Context, runID uuid.UUID, moduleID string) (bool, error)
	UnlockModule(ctx context.Context, ex Executor, runID uuid.UUID, moduleID string, source string) error
	GetUnlock(ctx context.Context, runID uuid.UUID, moduleID string) (*domain.ModuleUnlock, error)
}

// ── ProgressRepository ────────────────────────────────────────────────────────

type ProgressRepository interface {
	GetModuleProgress(ctx context.Context, runID uuid.UUID, moduleID string) (*domain.ModuleProgress, error)
	ListModuleProgress(ctx context.Context, runID uuid.UUID) ([]*domain.ModuleProgress, error)

	StartModule(ctx context.Context, runID uuid.UUID, moduleID, firstActID string) (*domain.ModuleProgress, error)
	SetCurrentAct(ctx context.Context, runID uuid.UUID, moduleID, actID string) error
	AppendCompletedAct(ctx context.Context, ex Executor, runID uuid.UUID, moduleID, actID string) error
	AppendCompletedTask(ctx context.Context, ex Executor, runID uuid.UUID, moduleID, taskID string) error
	UpdateTrackers(ctx context.Context, ex Executor, runID uuid.UUID, moduleID string, delta domain.TrackerState) error
	CompleteModule(ctx context.Context, ex Executor, runID uuid.UUID, moduleID, endingID string, finalScore int) error
	ResetForReplay(ctx context.Context, runID uuid.UUID, moduleID string) error
	IncrementReplayCount(ctx context.Context, runID uuid.UUID, moduleID string) error

	RecordCompletion(ctx context.Context, ex Executor, runID uuid.UUID, moduleID, endingID string, attemptNum, score int, trackerState domain.TrackerState) error
	GetLatestCompletion(ctx context.Context, runID uuid.UUID, moduleID string) (*domain.ModuleCompletion, error)

	GetSetupMeta(ctx context.Context, runID uuid.UUID) (*domain.PlayerSetupMeta, error)
	SetTelemetryConsent(ctx context.Context, runID uuid.UUID, consent string) error
	SetDefaultBranchName(ctx context.Context, runID uuid.UUID, name string) error
}

// ── DecisionRepository ────────────────────────────────────────────────────────

type DecisionRepository interface {
	// RecordDecision atomically writes the decision row + snapshot row
	RecordDecision(ctx context.Context, ex Executor, d *domain.DecisionMade, snapshot domain.TrackerState) (*domain.DecisionMade, error)

	GetDecisionHistory(ctx context.Context, runID uuid.UUID, moduleID string) ([]*domain.DecisionMade, error)
	GetFullRunHistory(ctx context.Context, runID uuid.UUID) ([]*domain.DecisionMade, error)

	GetSnapshotAt(ctx context.Context, decisionID uuid.UUID) (*domain.TrackerSnapshot, error)
	GetSnapshotsForModule(ctx context.Context, runID uuid.UUID, moduleID string) ([]*domain.TrackerSnapshot, error)
}

// ── VerificationRepository ───────────────────────────────────────────────────

type VerificationRepository interface {
	GetTaskCompletion(ctx context.Context, runID uuid.UUID, taskID string) (*domain.TaskCompletion, error)
	IsLocked(ctx context.Context, runID uuid.UUID, taskID string) (bool, error)
	RecordAttempt(ctx context.Context, runID uuid.UUID, moduleID, actID, taskID string, passed bool, results []domain.CheckResult) error
	LockTask(ctx context.Context, runID uuid.UUID, taskID string, xpAwarded int) error
	ListCompletedTasks(ctx context.Context, runID uuid.UUID, moduleID string) ([]string, error)
}

// ── ScoreRepository ───────────────────────────────────────────────────────────

type ScoreRepository interface {
	AppendScore(ctx context.Context, ex Executor, runID uuid.UUID, moduleID string, delta int, reason string) error
	GetRunScore(ctx context.Context, runID uuid.UUID) (int, error)
	GetScoreHistory(ctx context.Context, runID uuid.UUID) ([]*domain.PerformanceScore, error)

	AddFlag(ctx context.Context, ex Executor, runID uuid.UUID, f *domain.YearlyReviewFlag) error
	OffsetFlag(ctx context.Context, runID uuid.UUID, flagID, offsetByModuleID string) error
	GetFlags(ctx context.Context, runID uuid.UUID) ([]*domain.YearlyReviewFlag, error)
	CheckAndOffsetFlags(ctx context.Context, runID uuid.UUID, completedModuleID string) error
}
