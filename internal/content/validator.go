package content

import (
	"fmt"
	"strconv"
)

var validCheckTypes = map[string]bool{
	"exit_code":    true,
	"file_match":   true,
	"regex_output": true,
}

var validOps = map[string]bool{
	">=": true, "<=": true,
	">": true, "<": true, "=": true,
}

// Validate checks required fields and known types across the full module.
// Returns the first error found.
func Validate(mc *ModuleContent) error {
	if mc.ID == "" {
		return fmt.Errorf("module.yaml: id is required")
	}
	if mc.Version == "" {
		return fmt.Errorf("module %s: version is required", mc.ID)
	}
	if len(mc.Acts) == 0 {
		return fmt.Errorf("module %s: at least one act is required", mc.ID)
	}
	if len(mc.Endings) == 0 {
		return fmt.Errorf("module %s: at least one ending is required", mc.ID)
	}

	for _, act := range mc.Acts {
		if err := validateAct(mc.ID, act); err != nil {
			return err
		}
	}
	for _, ending := range mc.Endings {
		if err := validateEnding(mc.ID, ending); err != nil {
			return err
		}
	}
	return nil
}

func validateAct(moduleID string, act *ActContent) error {
	if act.ID == "" {
		return fmt.Errorf("module %s: act missing id", moduleID)
	}
	if act.Narrative == "" {
		return fmt.Errorf("module %s act %s: narrative is required", moduleID, act.ID)
	}
	if len(act.Tasks) == 0 {
		return fmt.Errorf("module %s act %s: at least one task is required", moduleID, act.ID)
	}
	for _, task := range act.Tasks {
		if err := validateTask(moduleID, act.ID, task); err != nil {
			return err
		}
	}
	if act.Decision != nil {
		if len(act.Decision.Options) < 2 {
			return fmt.Errorf("module %s act %s: decision requires at least 2 options", moduleID, act.ID)
		}
	}
	return nil
}

func validateTask(moduleID, actID string, task *TaskContent) error {
	if task.ID == "" {
		return fmt.Errorf("module %s act %s: task missing id", moduleID, actID)
	}
	if len(task.Checks) == 0 {
		return fmt.Errorf("module %s act %s task %s: at least one check required", moduleID, actID, task.ID)
	}
	for _, check := range task.Checks {
		if !validCheckTypes[check.Type] {
			return fmt.Errorf("module %s task %s: unknown check type %q (valid: exit_code, file_match, regex_output)",
				moduleID, task.ID, check.Type)
		}
		switch check.Type {
		case "exit_code":
			if check.Command == "" {
				return fmt.Errorf("module %s task %s: exit_code check requires command", moduleID, task.ID)
			}
		case "file_match":
			if check.Path == "" || check.Contains == "" {
				return fmt.Errorf("module %s task %s: file_match check requires path and contains", moduleID, task.ID)
			}
		case "regex_output":
			if check.Command == "" || check.Pattern == "" {
				return fmt.Errorf("module %s task %s: regex_output check requires command and pattern", moduleID, task.ID)
			}
		}
	}
	return nil
}

func validateEnding(moduleID string, ending *EndingContent) error {
	if ending.ID == "" {
		return fmt.Errorf("module %s: ending missing id", moduleID)
	}
	for _, cond := range ending.Conditions {
		if !validOps[cond.Op] {
			return fmt.Errorf("module %s ending %s: invalid operator %q", moduleID, ending.ID, cond.Op)
		}
		// numeric fields must have numeric values
		if cond.Field == "morale" || cond.Field == "technical_debt" {
			if _, err := strconv.Atoi(cond.Value); err != nil {
				return fmt.Errorf("module %s ending %s: field %s value %q must be an integer",
					moduleID, ending.ID, cond.Field, cond.Value)
			}
		}
	}
	return nil
}
