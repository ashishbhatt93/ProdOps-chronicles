package content

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Loader reads module content from a directory structure:
//
//	<root>/
//	  module.yaml
//	  acts/01-*.yaml
//	  endings/endings.yaml
//	  community/acts/*.yaml
type Loader struct {
	root string
}

func NewLoader(root string) *Loader {
	return &Loader{root: root}
}

// Load reads, parses, validates and returns the full ModuleContent.
func (l *Loader) Load() (*ModuleContent, error) {
	// ── module.yaml ──────────────────────────────────────────────────────────
	raw, err := os.ReadFile(filepath.Join(l.root, "module.yaml"))
	if err != nil {
		return nil, fmt.Errorf("read module.yaml: %w", err)
	}

	var manifest moduleManifest
	if err := yaml.Unmarshal(raw, &manifest); err != nil {
		return nil, fmt.Errorf("parse module.yaml: %w", err)
	}

	mc := &ModuleContent{
		ID:             manifest.ID,
		Name:           manifest.Name,
		Version:        manifest.Version,
		OrderIndex:     manifest.OrderIndex,
		Mode:           manifest.Mode,
		Runtime:        manifest.Runtime,
		ScoreThreshold: manifest.ScoreThreshold,
		Description:    manifest.Description,
	}

	// ── acts ─────────────────────────────────────────────────────────────────
	for _, entry := range manifest.Acts {
		actPath := filepath.Join(l.root, entry.File)
		act, err := l.loadAct(actPath)
		if err != nil {
			return nil, fmt.Errorf("load act %s: %w", entry.ID, err)
		}
		mc.Acts = append(mc.Acts, act)
	}

	// community acts (sorted by filename — contributor acts appended)
	communityDir := filepath.Join(l.root, "community", "acts")
	communityFiles, _ := filepath.Glob(filepath.Join(communityDir, "*.yaml"))
	for _, f := range communityFiles {
		act, err := l.loadAct(f)
		if err != nil {
			return nil, fmt.Errorf("load community act %s: %w", f, err)
		}
		mc.Acts = append(mc.Acts, act)
	}

	// ── endings ───────────────────────────────────────────────────────────────
	endingsPath := filepath.Join(l.root, manifest.EndingsFile)
	endings, err := l.loadEndings(endingsPath)
	if err != nil {
		return nil, fmt.Errorf("load endings: %w", err)
	}
	mc.Endings = endings

	return mc, Validate(mc)
}

func (l *Loader) loadAct(path string) (*ActContent, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var ra rawAct
	if err := yaml.Unmarshal(raw, &ra); err != nil {
		return nil, err
	}
	return mapAct(&ra), nil
}

func (l *Loader) loadEndings(path string) ([]*EndingContent, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var re rawEndings
	if err := yaml.Unmarshal(raw, &re); err != nil {
		return nil, err
	}
	var endings []*EndingContent
	for _, r := range re.Endings {
		endings = append(endings, mapEnding(&r))
	}
	return endings, nil
}

// ── raw YAML structs (unexported) ────────────────────────────────────────────

type moduleManifest struct {
	ID             string      `yaml:"id"`
	Name           string      `yaml:"name"`
	Version        string      `yaml:"version"`
	OrderIndex     int         `yaml:"order_index"`
	Mode           string      `yaml:"mode"`
	Runtime        string      `yaml:"runtime"`
	ScoreThreshold int         `yaml:"score_threshold"`
	Description    string      `yaml:"description"`
	Acts           []actEntry  `yaml:"acts"`
	EndingsFile    string      `yaml:"endings_file"`
}

type actEntry struct {
	ID   string `yaml:"id"`
	File string `yaml:"file"`
}

type rawAct struct {
	ID        string       `yaml:"id"`
	Title     string       `yaml:"title"`
	ActNumber int          `yaml:"act_number"`
	Narrative string       `yaml:"narrative"`
	Tasks     []rawTask    `yaml:"tasks"`
	Decision  *rawDecision `yaml:"decision"`
}

type rawTask struct {
	ID     string     `yaml:"id"`
	Title  string     `yaml:"title"`
	XP     int        `yaml:"xp"`
	Checks []rawCheck `yaml:"checks"`
	Hints  []string   `yaml:"hints"`
}

type rawCheck struct {
	Type           string `yaml:"type"`
	Command        string `yaml:"command"`
	ExpectedExit   int    `yaml:"expected_exit"`
	Path           string `yaml:"path"`
	Contains       string `yaml:"contains"`
	Pattern        string `yaml:"pattern"`
	FailureMessage string `yaml:"failure_message"`
}

type rawDecision struct {
	Prompt  string      `yaml:"prompt"`
	Options []rawOption `yaml:"options"`
}

type rawOption struct {
	ID       string `yaml:"id"`
	Label    string `yaml:"label"`
	Narrative string `yaml:"narrative"`
	TrackerEffects struct {
		MoraleDelta    int    `yaml:"morale_delta"`
		DebtDelta      int    `yaml:"debt_delta"`
		SeverityChange string `yaml:"severity_change"`
	} `yaml:"tracker_effects"`
	YearlyReviewFlag *struct {
		ID            string   `yaml:"id"`
		Severity      string   `yaml:"severity"`
		Note          string   `yaml:"note"`
		CanBeOffsetBy []string `yaml:"can_be_offset_by"`
	} `yaml:"yearly_review_flag"`
}

type rawEndings struct {
	Endings []rawEnding `yaml:"endings"`
}

type rawEnding struct {
	ID               string `yaml:"id"`
	Title            string `yaml:"title"`
	Conditions       []struct {
		Field string `yaml:"field"`
		Op    string `yaml:"op"`
		Value string `yaml:"value"`
	} `yaml:"conditions"`
	Narrative        string `yaml:"narrative"`
	PostMortem       string `yaml:"post_mortem"`
	PerformanceDelta int    `yaml:"performance_delta"`
	NextModule       string `yaml:"next_module"`
}

// ── mappers ───────────────────────────────────────────────────────────────────

func mapAct(r *rawAct) *ActContent {
	ac := &ActContent{
		ID:        r.ID,
		Title:     r.Title,
		ActNumber: r.ActNumber,
		Narrative: r.Narrative,
	}
	for _, rt := range r.Tasks {
		t := &TaskContent{
			ID:    rt.ID,
			Title: rt.Title,
			XP:    rt.XP,
			Hints: rt.Hints,
		}
		for _, rc := range rt.Checks {
			t.Checks = append(t.Checks, &CheckContent{
				Type:           rc.Type,
				Command:        rc.Command,
				ExpectedExit:   rc.ExpectedExit,
				Path:           rc.Path,
				Contains:       rc.Contains,
				Pattern:        rc.Pattern,
				FailureMessage: rc.FailureMessage,
			})
		}
		ac.Tasks = append(ac.Tasks, t)
	}
	if r.Decision != nil {
		ac.Decision = &DecisionContent{Prompt: r.Decision.Prompt}
		for _, ro := range r.Decision.Options {
			opt := &OptionContent{
				ID:        ro.ID,
				Label:     ro.Label,
				Narrative: ro.Narrative,
				TrackerEffects: TrackerEffects{
					MoraleDelta:    ro.TrackerEffects.MoraleDelta,
					DebtDelta:      ro.TrackerEffects.DebtDelta,
					SeverityChange: ro.TrackerEffects.SeverityChange,
				},
			}
			if ro.YearlyReviewFlag != nil {
				opt.YearlyFlag = &YearlyFlagContent{
					ID:            ro.YearlyReviewFlag.ID,
					Severity:      ro.YearlyReviewFlag.Severity,
					Note:          ro.YearlyReviewFlag.Note,
					CanBeOffsetBy: ro.YearlyReviewFlag.CanBeOffsetBy,
				}
			}
			ac.Decision.Options = append(ac.Decision.Options, opt)
		}
	}
	return ac
}

func mapEnding(r *rawEnding) *EndingContent {
	e := &EndingContent{
		ID:               r.ID,
		Title:            r.Title,
		Narrative:        r.Narrative,
		PostMortem:       r.PostMortem,
		PerformanceDelta: r.PerformanceDelta,
		NextModule:       r.NextModule,
	}
	for _, c := range r.Conditions {
		e.Conditions = append(e.Conditions, EndingCondition{
			Field: c.Field,
			Op:    c.Op,
			Value: c.Value,
		})
	}
	return e
}
