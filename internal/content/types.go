package content

// ModuleContent is the parsed, validated in-memory representation of a module's YAML.
type ModuleContent struct {
	ID             string
	Name           string
	Version        string
	OrderIndex     int
	Mode           string
	Runtime        string
	ScoreThreshold int
	Description    string
	Acts           []*ActContent
	Endings        []*EndingContent
}

type ActContent struct {
	ID        string
	Title     string
	ActNumber int
	Narrative string
	Tasks     []*TaskContent
	Decision  *DecisionContent
}

type TaskContent struct {
	ID     string
	Title  string
	XP     int
	Checks []*CheckContent
	Hints  []string
}

type CheckContent struct {
	Type           string // exit_code | file_match | regex_output
	Command        string
	ExpectedExit   int
	Path           string
	Contains       string
	Pattern        string
	FailureMessage string
}

type DecisionContent struct {
	Prompt  string
	Options []*OptionContent
}

type OptionContent struct {
	ID             string
	Label          string
	Narrative      string
	TrackerEffects TrackerEffects
	YearlyFlag     *YearlyFlagContent
}

type TrackerEffects struct {
	MoraleDelta    int
	DebtDelta      int
	SeverityChange string
}

type YearlyFlagContent struct {
	ID            string
	Severity      string
	Note          string
	CanBeOffsetBy []string
}

type EndingContent struct {
	ID               string
	Title            string
	Conditions       []EndingCondition
	Narrative        string
	PostMortem       string
	PerformanceDelta int
	NextModule       string
}

type EndingCondition struct {
	Field string // morale | technical_debt | incident_severity
	Op    string // >= | <= | > | < | =
	Value string // stored as string, parsed at eval time
}
