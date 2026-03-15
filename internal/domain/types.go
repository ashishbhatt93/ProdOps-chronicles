package domain

import (
	"time"

	"github.com/google/uuid"
)

// ── Run ───────────────────────────────────────────────────────────────────────

type RunStatus string

const (
	RunStatusInProgress RunStatus = "in_progress"
	RunStatusCompleted  RunStatus = "completed"
)

type Run struct {
	ID                   uuid.UUID
	PlayerID             uuid.UUID
	Status               RunStatus
	StartedAt            time.Time
	CompletedAt          *time.Time
	FinalPerformanceScore *int
}

// ── Player ────────────────────────────────────────────────────────────────────

type TelemetryConsent string

const (
	TelemetryTier1 TelemetryConsent = "tier1" // local only (default)
	TelemetryTier2 TelemetryConsent = "tier2" // anonymous telemetry opt-in
	TelemetryTier3 TelemetryConsent = "tier3" // public repo opt-in
)

type PlayerIdentity struct {
	ID           uuid.UUID
	DisplayName  string
	CreatedAt    time.Time
	CurrentRunID *uuid.UUID
}

type PlayerProfile struct {
	ID               uuid.UUID
	PlayerID         uuid.UUID
	GitUsername      string
	GitEmail         string
	SSHKeyPath       string
	SyncRemote       string
	TelemetryConsent TelemetryConsent
	UpdatedAt        time.Time
}

type GitIdentity struct {
	Username string
	Email    string
	Source   string // path to .gitconfig block or "manual"
}

// ── Module ────────────────────────────────────────────────────────────────────

type Module struct {
	ID               string
	Name             string
	Version          string
	PodName          string
	OrderIndex       int
	Mode             string
	Runtime          string
	Description      string
	RequiresModuleID *string
	ScoreThreshold   int
}

type ModuleUnlock struct {
	ID           uuid.UUID
	RunID        uuid.UUID
	ModuleID     string
	UnlockedAt   time.Time
	UnlockSource string // "earned" | "forced"
}

// ── Module Progress ───────────────────────────────────────────────────────────

type ModuleStatus string

const (
	ModuleStatusNotStarted ModuleStatus = "not_started"
	ModuleStatusInProgress ModuleStatus = "in_progress"
	ModuleStatusCompleted  ModuleStatus = "completed"
)

type ModuleProgress struct {
	ID               uuid.UUID
	RunID            uuid.UUID
	ModuleID         string
	Status           ModuleStatus
	CurrentActID     *string
	CompletedActs    []string
	CompletedTasks   []string
	Morale           int
	IncidentSeverity string
	TechnicalDebt    int
	XPEarned         int
	FinalScore       *int
	EndingID         *string
	ReplayCount      int
	StartedAt        *time.Time
	CompletedAt      *time.Time
	LastActiveAt     time.Time
}

// ── Tracker ───────────────────────────────────────────────────────────────────

type TrackerState struct {
	Morale           int    `json:"morale"`
	IncidentSeverity string `json:"incident_severity"`
	TechnicalDebt    int    `json:"technical_debt"`
}

// ── Decisions ────────────────────────────────────────────────────────────────

type DecisionMade struct {
	ID             uuid.UUID
	RunID          uuid.UUID
	ModuleID       string
	ActID          string
	OptionID       string
	DecidedAt      time.Time
	MoraleDelta    int
	SeverityChange *string
	DebtDelta      int
}

type TrackerSnapshot struct {
	ID              uuid.UUID
	RunID           uuid.UUID
	ModuleID        string
	ActID           string
	DecisionID      uuid.UUID
	State           TrackerState
	SnapshotTakenAt time.Time
}

// ── Task Completions ──────────────────────────────────────────────────────────

type TaskCompletion struct {
	ID              uuid.UUID
	RunID           uuid.UUID
	ModuleID        string
	ActID           string
	TaskID          string
	AttemptCount    int
	FirstPassedAt   *time.Time
	IsLocked        bool
	LastAttemptedAt time.Time
	XPAwarded       int
	CheckResults    []CheckResult
}

type CheckResult struct {
	Type           string `json:"type"`
	Passed         bool   `json:"passed"`
	ExecutionError bool   `json:"execution_error"`
	Output         string `json:"output"`
	Detail         string `json:"detail"`
}

// ── Performance ───────────────────────────────────────────────────────────────

type PerformanceScore struct {
	ID         uuid.UUID
	RunID      uuid.UUID
	ModuleID   string
	Delta      int
	Reason     string
	RecordedAt time.Time
}

// ── Yearly Review ─────────────────────────────────────────────────────────────

type FlagSeverity string

const (
	FlagSeverityMinor    FlagSeverity = "minor"
	FlagSeverityModerate FlagSeverity = "moderate"
	FlagSeveritySevere   FlagSeverity = "severe"
)

type YearlyReviewFlag struct {
	ID               uuid.UUID
	RunID            uuid.UUID
	ModuleID         string
	FlagID           string
	Severity         FlagSeverity
	Note             string
	CanBeOffsetBy    []string
	IsOffset         bool
	OffsetByModuleID *string
	CreatedAt        time.Time
}

// ── Module Completions ────────────────────────────────────────────────────────

type ModuleCompletion struct {
	ID            uuid.UUID
	RunID         uuid.UUID
	ModuleID      string
	AttemptNumber int
	EndingID      string
	FinalScore    int
	CompletedAt   time.Time
	TrackerState  TrackerState
}

// ── Setup Meta ────────────────────────────────────────────────────────────────

type PlayerSetupMeta struct {
	ID                 uuid.UUID
	RunID              uuid.UUID
	TelemetryConsent   *string
	DefaultBranchName  *string
	UpdatedAt          time.Time
}

// ── Unlock display status (derived, not stored) ───────────────────────────────

type ModuleDisplayStatus string

const (
	ModuleDisplayAvailable  ModuleDisplayStatus = "available"
	ModuleDisplayLocked     ModuleDisplayStatus = "locked"
	ModuleDisplayInProgress ModuleDisplayStatus = "in_progress"
	ModuleDisplayCompleted  ModuleDisplayStatus = "completed"
)

type ModuleListEntry struct {
	Module        Module
	DisplayStatus ModuleDisplayStatus
	LockReason    string // populated when locked
	Progress      *ModuleProgress
}
