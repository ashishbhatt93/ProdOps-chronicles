package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/prodops-chronicles/prodops/internal/domain"
	"github.com/prodops-chronicles/prodops/internal/repository"
)

type SyncService struct {
	players  repository.PlayerRepository
	progress repository.ProgressRepository
	decisions repository.DecisionRepository
	scores   repository.ScoreRepository
	modules  repository.ModuleRepository
}

func NewSyncService(
	players repository.PlayerRepository,
	progress repository.ProgressRepository,
	decisions repository.DecisionRepository,
	scores repository.ScoreRepository,
	modules repository.ModuleRepository,
) *SyncService {
	return &SyncService{
		players:  players,
		progress: progress,
		decisions: decisions,
		scores:   scores,
		modules:  modules,
	}
}

// ProgressExport is the full progress.json structure.
type ProgressExport struct {
	SchemaVersion    string            `json:"schema_version"`
	ExportedAt       time.Time         `json:"exported_at"`
	RunNumber        int               `json:"run_number"`
	StartedAt        time.Time         `json:"started_at"`
	PerformanceScore int               `json:"performance_score"`
	XPEarned         int               `json:"xp_earned"`
	Modules          []ModuleExport    `json:"modules"`
	YearlyReviewFlags []FlagExport     `json:"yearly_review_flags"`
}

type ModuleExport struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	Status       string           `json:"status"`
	Attempts     *int             `json:"attempts"`
	FinalScore   *int             `json:"final_score"`
	EndingID     *string          `json:"ending_id"`
	CompletedAt  *time.Time       `json:"completed_at,omitempty"`
	Decisions    []DecisionExport `json:"decisions"`
	TrackerAtEnd *TrackerExport   `json:"tracker_at_end,omitempty"`
}

type DecisionExport struct {
	ActID       string `json:"act_id"`
	OptionID    string `json:"option_id"`
	MoraleDelta int    `json:"morale_delta"`
	DebtDelta   int    `json:"debt_delta"`
}

type TrackerExport struct {
	Morale           int    `json:"morale"`
	IncidentSeverity string `json:"incident_severity"`
	TechnicalDebt    int    `json:"technical_debt"`
}

type FlagExport struct {
	FlagID        string   `json:"flag_id"`
	Severity      string   `json:"severity"`
	Note          string   `json:"note"`
	CanBeOffsetBy []string `json:"can_be_offset_by"`
	IsOffset      bool     `json:"is_offset"`
}

// TelemetryExport is the stripped version sent on tier2/tier3.
type TelemetryExport struct {
	SchemaVersion     string                  `json:"schema_version"`
	RunNumber         int                     `json:"run_number"`
	Modules           []TelemetryModuleExport `json:"modules"`
	YearlyReviewFlags []TelemetryFlagExport   `json:"yearly_review_flags"`
}

type TelemetryModuleExport struct {
	ID          string                    `json:"id"`
	Status      string                    `json:"status"`
	Attempts    *int                      `json:"attempts"`
	FinalScore  *int                      `json:"final_score"`
	EndingID    *string                   `json:"ending_id"`
	Decisions   []TelemetryDecisionExport `json:"decisions"`
}

type TelemetryDecisionExport struct {
	ActID    string `json:"act_id"`
	OptionID string `json:"option_id"`
}

type TelemetryFlagExport struct {
	FlagID   string `json:"flag_id"`
	Severity string `json:"severity"`
	IsOffset bool   `json:"is_offset"`
}

// ExportProgressSnapshot builds the full progress export for a run.
func (s *SyncService) ExportProgressSnapshot(ctx context.Context, run *domain.Run, runNumber int) (*ProgressExport, error) {
	score, err := s.scores.GetRunScore(ctx, run.ID)
	if err != nil {
		return nil, err
	}

	allModules, err := s.modules.ListAvailableModules(ctx, run.ID)
	if err != nil {
		return nil, err
	}

	flags, err := s.scores.GetFlags(ctx, run.ID)
	if err != nil {
		return nil, err
	}

	export := &ProgressExport{
		SchemaVersion:    "1.0",
		ExportedAt:       time.Now().UTC(),
		RunNumber:        runNumber,
		StartedAt:        run.StartedAt,
		PerformanceScore: score,
	}

	for _, entry := range allModules {
		me := ModuleExport{
			ID:     entry.Module.ID,
			Name:   entry.Module.Name,
			Status: string(entry.DisplayStatus),
		}
		if entry.Progress != nil {
			me.Attempts = &entry.Progress.ReplayCount
			me.FinalScore = entry.Progress.FinalScore
			me.EndingID = entry.Progress.EndingID
			me.CompletedAt = entry.Progress.CompletedAt
			export.XPEarned += entry.Progress.XPEarned

			if entry.Progress.Status == domain.ModuleStatusCompleted {
				tracker := &TrackerExport{
					Morale:           entry.Progress.Morale,
					IncidentSeverity: entry.Progress.IncidentSeverity,
					TechnicalDebt:    entry.Progress.TechnicalDebt,
				}
				me.TrackerAtEnd = tracker
			}

			decisions, _ := s.decisions.GetDecisionHistory(ctx, run.ID, entry.Module.ID)
			for _, d := range decisions {
				me.Decisions = append(me.Decisions, DecisionExport{
					ActID:       d.ActID,
					OptionID:    d.OptionID,
					MoraleDelta: d.MoraleDelta,
					DebtDelta:   d.DebtDelta,
				})
			}
		}
		export.Modules = append(export.Modules, me)
	}

	for _, f := range flags {
		export.YearlyReviewFlags = append(export.YearlyReviewFlags, FlagExport{
			FlagID:        f.FlagID,
			Severity:      string(f.Severity),
			Note:          f.Note,
			CanBeOffsetBy: f.CanBeOffsetBy,
			IsOffset:      f.IsOffset,
		})
	}

	return export, nil
}

// StripForTelemetry uses an allowlist — never a denylist.
func (s *SyncService) StripForTelemetry(export *ProgressExport) *TelemetryExport {
	te := &TelemetryExport{
		SchemaVersion: export.SchemaVersion,
		RunNumber:     export.RunNumber,
	}
	for _, m := range export.Modules {
		tm := TelemetryModuleExport{
			ID:         m.ID,
			Status:     m.Status,
			Attempts:   m.Attempts,
			FinalScore: m.FinalScore,
			EndingID:   m.EndingID,
		}
		for _, d := range m.Decisions {
			tm.Decisions = append(tm.Decisions, TelemetryDecisionExport{
				ActID:    d.ActID,
				OptionID: d.OptionID,
			})
		}
		te.Modules = append(te.Modules, tm)
	}
	for _, f := range export.YearlyReviewFlags {
		te.YearlyReviewFlags = append(te.YearlyReviewFlags, TelemetryFlagExport{
			FlagID:   f.FlagID,
			Severity: f.Severity,
			IsOffset: f.IsOffset,
		})
	}
	return te
}

// SerialiseToJSON returns the indented JSON bytes of an export.
func SerialiseToJSON(v any) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}

// SerialiseToMarkdown renders a human-readable PROGRESS.md.
func SerialiseToMarkdown(export *ProgressExport) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# ProdOps Progress — Run #%d\n\n", export.RunNumber))
	sb.WriteString(fmt.Sprintf("> Started: %s · Last sync: %s · Performance: %+d\n\n",
		export.StartedAt.Format("2 Jan 2006"),
		export.ExportedAt.Format("2 Jan 2006"),
		export.PerformanceScore))

	sb.WriteString("## Modules\n\n")
	sb.WriteString("| Module | Status | Score | Ending |\n")
	sb.WriteString("|--------|--------|-------|--------|\n")
	for _, m := range export.Modules {
		icon := statusIcon(m.Status)
		score := "—"
		if m.FinalScore != nil {
			score = fmt.Sprintf("%d%%", *m.FinalScore)
		}
		ending := "—"
		if m.EndingID != nil {
			ending = *m.EndingID
		}
		sb.WriteString(fmt.Sprintf("| %s | %s %s | %s | %s |\n",
			m.Name, icon, m.Status, score, ending))
	}

	for _, m := range export.Modules {
		if len(m.Decisions) == 0 {
			continue
		}
		sb.WriteString(fmt.Sprintf("\n## %s — Decision Log\n\n", m.Name))
		sb.WriteString("| Act | Option | Morale | Debt |\n")
		sb.WriteString("|-----|--------|--------|------|\n")
		for _, d := range m.Decisions {
			sb.WriteString(fmt.Sprintf("| %s | %s | %+d | %+d |\n",
				d.ActID, d.OptionID, d.MoraleDelta, d.DebtDelta))
		}
	}

	if len(export.YearlyReviewFlags) > 0 {
		sb.WriteString("\n## Yearly Review Flags\n\n")
		for _, f := range export.YearlyReviewFlags {
			offset := ""
			if f.IsOffset {
				offset = " *(offset)*"
			}
			sb.WriteString(fmt.Sprintf("⚠️ **%s** — `%s`: %s%s\n",
				f.Severity, f.FlagID, f.Note, offset))
		}
	}

	return sb.String()
}

func statusIcon(status string) string {
	switch status {
	case "completed":
		return "✅"
	case "in_progress":
		return "🔄"
	default:
		return "🔒"
	}
}

// PushToRemote commits and pushes the progress directory to the configured remote.
func PushToRemote(repoDir, remote, message string) error {
	cmds := [][]string{
		{"git", "-C", repoDir, "add", "."},
		{"git", "-C", repoDir, "commit", "-m", message},
		{"git", "-C", repoDir, "push", remote, "main"},
	}
	for _, args := range cmds {
		out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s: %w\n%s", strings.Join(args, " "), err, out)
		}
	}
	return nil
}
