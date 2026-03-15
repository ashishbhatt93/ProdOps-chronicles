package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	appconfig "github.com/prodops-chronicles/prodops/internal/config"
	"github.com/prodops-chronicles/prodops/internal/content"
	"github.com/prodops-chronicles/prodops/internal/domain"
	"github.com/prodops-chronicles/prodops/internal/repository"
)

type VerificationService struct {
	progress     repository.ProgressRepository
	verification repository.VerificationRepository
	scores       repository.ScoreRepository
	httpClient   *http.Client
	learnerHome  string
	cfg          *appconfig.Config
}

func NewVerificationService(
	progress repository.ProgressRepository,
	verification repository.VerificationRepository,
	scores repository.ScoreRepository,
	learnerHome string,
	cfg *appconfig.Config,
) *VerificationService {
	return &VerificationService{
		progress:     progress,
		verification: verification,
		scores:       scores,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		learnerHome:  learnerHome,
		cfg:          cfg,
	}
}

type VerifyResult struct {
	TaskID        string
	Passed        bool
	AlreadyLocked bool
	AttemptCount  int
	XPAwarded     int
	CheckResults  []domain.CheckResult
	AllTasksDone  bool // true when last task in act passes
}

// VerifyTask runs the current task's checks via the module pod.
// Full flow as designed: fast-path lock check → run all checks → record → lock if passed.
func (s *VerificationService) VerifyTask(ctx context.Context, runID uuid.UUID, moduleID string, contentCache map[string]*content.ModuleContent) (*VerifyResult, error) {
	mp, err := s.progress.GetModuleProgress(ctx, runID, moduleID)
	if err != nil {
		return nil, err
	}
	if mp.CurrentActID == nil {
		return nil, fmt.Errorf("%w: module not started", domain.ErrInvalidInput)
	}

	mc, ok := contentCache[moduleID]
	if !ok {
		return nil, fmt.Errorf("content not loaded for module %s", moduleID)
	}

	// Find current act
	var currentAct *content.ActContent
	for _, a := range mc.Acts {
		if a.ID == *mp.CurrentActID {
			currentAct = a
			break
		}
	}
	if currentAct == nil {
		return nil, fmt.Errorf("act %s not found", *mp.CurrentActID)
	}

	// Find current task: first task NOT in completed_tasks[]
	completedSet := make(map[string]bool, len(mp.CompletedTasks))
	for _, id := range mp.CompletedTasks {
		completedSet[id] = true
	}
	var currentTask *content.TaskContent
	for _, t := range currentAct.Tasks {
		if !completedSet[t.ID] {
			currentTask = t
			break
		}
	}
	if currentTask == nil {
		// All tasks done — decision is ready
		return &VerifyResult{AllTasksDone: true}, nil
	}

	// Fast path: already locked
	locked, err := s.verification.IsLocked(ctx, runID, currentTask.ID)
	if err != nil {
		return nil, err
	}
	if locked {
		return &VerifyResult{
			TaskID:        currentTask.ID,
			Passed:        true,
			AlreadyLocked: true,
		}, nil
	}

	// Run all checks via pod — AND logic, all checks always run
	podURL := fmt.Sprintf("http://%s:8080/verify", "prodops-"+moduleID)
	var results []domain.CheckResult
	allPassed := true

	for _, check := range currentTask.Checks {
		result, err := s.executeCheck(ctx, podURL, check)
		if err != nil {
			return nil, fmt.Errorf("execution failed for task %s: %w", currentTask.ID, err)
		}
		results = append(results, *result)
		if !result.Passed {
			allPassed = false
		}
	}

	// Record attempt
	if err := s.verification.RecordAttempt(ctx, runID, moduleID,
		currentAct.ID, currentTask.ID, allPassed, results); err != nil {
		return nil, err
	}

	tc, _ := s.verification.GetTaskCompletion(ctx, runID, currentTask.ID)
	attemptCount := 1
	if tc != nil {
		attemptCount = tc.AttemptCount
	}

	result := &VerifyResult{
		TaskID:       currentTask.ID,
		Passed:       allPassed,
		AttemptCount: attemptCount,
		CheckResults: results,
	}

	if allPassed {
		if err := s.verification.LockTask(ctx, runID, currentTask.ID, currentTask.XP); err != nil {
			return nil, err
		}
		if err := s.progress.AppendCompletedTask(ctx, nil, runID, moduleID, currentTask.ID); err != nil {
			return nil, err
		}
		if err := s.scores.AppendScore(ctx, nil, runID, moduleID, currentTask.XP,
			fmt.Sprintf("task %s completed", currentTask.ID)); err != nil {
			return nil, err
		}
		result.XPAwarded = currentTask.XP

		// Check if all tasks in act are now done
		result.AllTasksDone = areAllTasksDone(currentAct, append(mp.CompletedTasks, currentTask.ID))
	}

	return result, nil
}

type podVerifyRequest struct {
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

type podVerifyResponse struct {
	Passed         bool   `json:"passed"`
	ExecutionError bool   `json:"execution_error"`
	Output         string `json:"output"`
	Detail         string `json:"detail"`
}

func (s *VerificationService) executeCheck(ctx context.Context, podURL string, check *content.CheckContent) (*domain.CheckResult, error) {
	reqBody := podVerifyRequest{LearnerHome: s.learnerHome}
	reqBody.Check.Type = check.Type
	reqBody.Check.Command = check.Command
	reqBody.Check.ExpectedExit = check.ExpectedExit
	reqBody.Check.Path = check.Path
	reqBody.Check.Contains = check.Contains
	reqBody.Check.Pattern = check.Pattern
	reqBody.Check.FailureMessage = check.FailureMessage

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, podURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: pod unreachable: %v", domain.ErrInvalidInput, err)
	}
	defer resp.Body.Close()

	var podResp podVerifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&podResp); err != nil {
		return nil, err
	}

	if podResp.ExecutionError {
		return nil, fmt.Errorf("check execution error: %s", podResp.Detail)
	}

	return &domain.CheckResult{
		Type:           check.Type,
		Passed:         podResp.Passed,
		ExecutionError: podResp.ExecutionError,
		Output:         podResp.Output,
		Detail:         podResp.Detail,
	}, nil
}

func areAllTasksDone(act *content.ActContent, completedTasks []string) bool {
	done := make(map[string]bool, len(completedTasks))
	for _, id := range completedTasks {
		done[id] = true
	}
	for _, t := range act.Tasks {
		if !done[t.ID] {
			return false
		}
	}
	return true
}
