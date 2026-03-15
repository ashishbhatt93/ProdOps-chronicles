package pod

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// CheckRequest mirrors the JSON the backend sends.
type CheckRequest struct {
	Check struct {
		Type           string `json:"type"`
		Command        string `json:"command,omitempty"`
		ExpectedExit   int    `json:"expected_exit,omitempty"`
		Path           string `json:"path,omitempty"`
		Contains       string `json:"contains,omitempty"`
		Pattern        string `json:"pattern,omitempty"`
		FailureMessage string `json:"failure_message,omitempty"`
	} `json:"check"`
	LearnerHome string `json:"learner_home"`
}

// CheckResponse is returned to the backend.
type CheckResponse struct {
	Passed         bool   `json:"passed"`
	ExecutionError bool   `json:"execution_error"`
	Output         string `json:"output"`
	Detail         string `json:"detail"`
}

// RunCheck dispatches to the correct check runner.
func RunCheck(req *CheckRequest) *CheckResponse {
	switch req.Check.Type {
	case "exit_code":
		return runExitCode(req)
	case "file_match":
		return runFileMatch(req)
	case "regex_output":
		return runRegexOutput(req)
	default:
		return &CheckResponse{
			ExecutionError: true,
			Detail:         fmt.Sprintf("unknown check type: %s", req.Check.Type),
		}
	}
}

func runExitCode(req *CheckRequest) *CheckResponse {
	cmd := buildShellCmd(req.Check.Command, req.LearnerHome)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return &CheckResponse{
				ExecutionError: true,
				Detail:         fmt.Sprintf("command execution error: %v", err),
			}
		}
	}

	passed := exitCode == req.Check.ExpectedExit
	detail := ""
	if !passed {
		detail = req.Check.FailureMessage
		if detail == "" {
			detail = fmt.Sprintf("expected exit %d, got %d", req.Check.ExpectedExit, exitCode)
		}
	}
	return &CheckResponse{
		Passed: passed,
		Output: out.String(),
		Detail: detail,
	}
}

func runFileMatch(req *CheckRequest) *CheckResponse {
	path := req.Check.Path
	// Resolve relative paths against learner home
	if !strings.HasPrefix(path, "/") {
		path = req.LearnerHome + "/" + path
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &CheckResponse{
				Passed: false,
				Detail: fmt.Sprintf("file not found: %s", path),
			}
		}
		return &CheckResponse{
			ExecutionError: true,
			Detail:         fmt.Sprintf("could not read file: %v", err),
		}
	}

	passed := strings.Contains(string(data), req.Check.Contains)
	detail := ""
	if !passed {
		detail = req.Check.FailureMessage
		if detail == "" {
			detail = fmt.Sprintf("file %s does not contain expected content", path)
		}
	}
	return &CheckResponse{
		Passed: passed,
		Output: truncate(string(data), 500),
		Detail: detail,
	}
}

func runRegexOutput(req *CheckRequest) *CheckResponse {
	cmd := buildShellCmd(req.Check.Command, req.LearnerHome)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return &CheckResponse{
				ExecutionError: true,
				Detail:         fmt.Sprintf("command execution error: %v", err),
			}
		}
	}

	re, err := regexp.Compile(req.Check.Pattern)
	if err != nil {
		return &CheckResponse{
			ExecutionError: true,
			Detail:         fmt.Sprintf("invalid regex pattern: %v", err),
		}
	}

	passed := re.MatchString(out.String())
	detail := ""
	if !passed {
		detail = req.Check.FailureMessage
		if detail == "" {
			detail = fmt.Sprintf("output did not match pattern: %s", req.Check.Pattern)
		}
	}
	return &CheckResponse{
		Passed: passed,
		Output: truncate(out.String(), 500),
		Detail: detail,
	}
}

func buildShellCmd(command, learnerHome string) *exec.Cmd {
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = learnerHome
	cmd.Env = append(os.Environ(), "HOME="+learnerHome)
	return cmd
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...[truncated]"
}
