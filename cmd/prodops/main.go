package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const defaultBackendURL = "http://localhost:7741"

var (
	backendURL string
	token      string
)

func main() {
	root := &cobra.Command{
		Use:   "prodops",
		Short: "ProdOps Chronicles — learn DevOps by doing",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if token == "" {
				token = os.Getenv("BACKEND_TOKEN")
			}
		},
	}

	root.PersistentFlags().StringVar(&backendURL, "backend", defaultBackendURL, "Backend URL")
	root.PersistentFlags().StringVar(&token, "token", "", "Bearer token (or set BACKEND_TOKEN)")

	root.AddCommand(
		cmdStart(),
		cmdStop(),
		cmdStatus(),
		cmdModuleList(),
		cmdModuleEnable(),
		cmdModuleDisable(),
		cmdUnlock(),
		cmdLessonNext(),
		cmdVerify(),
		cmdDecision(),
		cmdProgress(),
		cmdSync(),
		cmdConfig(),
		cmdSetup(),
		cmdVersion(),
	)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

// ── prodops start ─────────────────────────────────────────────────────────────

func cmdStart() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the ProdOps stack",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("▶ Starting ProdOps stack...")
			if err := dockerCompose("up", "-d"); err != nil {
				return err
			}
			if err := waitHealthy(30 * time.Second); err != nil {
				return err
			}
			fmt.Println("✅ ProdOps is running. Type 'prodops lesson next' to begin.")
			return nil
		},
	}
}

// ── prodops stop ──────────────────────────────────────────────────────────────

func cmdStop() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the ProdOps stack",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("■ Stopping ProdOps stack...")
			return dockerCompose("down")
		},
	}
}

// ── prodops status ────────────────────────────────────────────────────────────

func cmdStatus() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show running containers and health",
		RunE: func(cmd *cobra.Command, args []string) error {
			return dockerCompose("ps")
		},
	}
}

// ── prodops module ────────────────────────────────────────────────────────────

func cmdModuleList() *cobra.Command {
	return &cobra.Command{
		Use:   "module list",
		Short: "List available and locked modules",
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := apiGet("/api/v1/modules")
			if err != nil {
				return err
			}
			var resp struct {
				Modules []struct {
					Module struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					} `json:"Module"`
					DisplayStatus string `json:"DisplayStatus"`
					LockReason    string `json:"LockReason"`
				} `json:"modules"`
			}
			if err := json.Unmarshal(body, &resp); err != nil {
				return err
			}
			fmt.Printf("%-20s %-15s %s\n", "MODULE", "STATUS", "NOTES")
			fmt.Println(strings.Repeat("─", 55))
			for _, m := range resp.Modules {
				icon := statusIcon(m.DisplayStatus)
				notes := ""
				if m.LockReason != "" {
					notes = m.LockReason
				}
				fmt.Printf("%-20s %s %-12s %s\n",
					m.Module.ID, icon, m.DisplayStatus, notes)
			}
			return nil
		},
	}
}

func cmdModuleEnable() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "module enable [module-id]",
		Short: "Enable and start a module",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			moduleID := args[0]
			_, err := apiPost("/api/v1/modules/"+moduleID+"/enable", nil)
			if err != nil {
				return err
			}
			fmt.Printf("✅ Module '%s' enabled. Run 'prodops lesson next' to start.\n", moduleID)
			return nil
		},
	}
	// attach to module parent command
	parent := &cobra.Command{Use: "module", Short: "Manage modules"}
	parent.AddCommand(cmd)
	parent.AddCommand(cmdModuleDisable())
	parent.AddCommand(cmdModuleListSub())
	return parent
}

func cmdModuleListSub() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List modules",
		RunE:  cmdModuleList().RunE,
	}
}

func cmdModuleDisable() *cobra.Command {
	return &cobra.Command{
		Use:   "disable [module-id]",
		Short: "Stop and disable a module",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			moduleID := args[0]
			_, err := apiPost("/api/v1/modules/"+moduleID+"/disable", nil)
			if err != nil {
				return err
			}
			fmt.Printf("■ Module '%s' disabled.\n", moduleID)
			return nil
		},
	}
}

// ── prodops unlock ────────────────────────────────────────────────────────────

func cmdUnlock() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "unlock [module-id]",
		Short: "Force-unlock a module (skips prerequisite check)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !force {
				return fmt.Errorf("use --force to confirm force-unlock")
			}
			moduleID := args[0]
			_, err := apiPost("/api/v1/modules/"+moduleID+"/unlock",
				map[string]bool{"force": true})
			if err != nil {
				return err
			}
			fmt.Printf("⚠️  Module '%s' force-unlocked. A yearly review flag has been recorded.\n", moduleID)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Confirm force-unlock")
	return cmd
}

// ── prodops lesson next ───────────────────────────────────────────────────────

func cmdLessonNext() *cobra.Command {
	lesson := &cobra.Command{
		Use:   "lesson",
		Short: "Lesson commands",
	}
	lesson.AddCommand(&cobra.Command{
		Use:   "next",
		Short: "Show the current act and next task",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get active run to determine current module
			runBody, err := apiGet("/api/v1/runs/active")
			if err != nil {
				return fmt.Errorf("no active run — start one with 'prodops start'")
			}
			var run struct {
				ID string `json:"id"`
			}
			json.Unmarshal(runBody, &run)

			// Get module list to find in-progress module
			body, err := apiGet("/api/v1/modules")
			if err != nil {
				return err
			}
			var resp struct {
				Modules []struct {
					Module        struct{ ID string `json:"id"` } `json:"Module"`
					DisplayStatus string `json:"DisplayStatus"`
				} `json:"modules"`
			}
			json.Unmarshal(body, &resp)

			moduleID := ""
			for _, m := range resp.Modules {
				if m.DisplayStatus == "in_progress" {
					moduleID = m.Module.ID
					break
				}
			}
			if moduleID == "" {
				// Find first available (unlocked, not started)
				for _, m := range resp.Modules {
					if m.DisplayStatus == "available" {
						moduleID = m.Module.ID
						break
					}
				}
			}
			if moduleID == "" {
				fmt.Println("🎉 All modules complete! Run 'prodops progress' to see your yearly review.")
				return nil
			}

			actBody, err := apiGet("/api/v1/modules/" + moduleID + "/acts/current")
			if err != nil {
				return err
			}
			printAct(actBody, moduleID)
			return nil
		},
	})
	return lesson
}

// ── prodops verify ────────────────────────────────────────────────────────────

func cmdVerify() *cobra.Command {
	var moduleID string
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify your current task",
		RunE: func(cmd *cobra.Command, args []string) error {
			if moduleID == "" {
				return fmt.Errorf("--module is required")
			}
			body, err := apiPost(
				fmt.Sprintf("/api/v1/modules/%s/acts/current/tasks/current/verify", moduleID),
				nil)
			if err != nil {
				return err
			}
			printVerifyResult(body)
			return nil
		},
	}
	cmd.Flags().StringVar(&moduleID, "module", "", "Module ID to verify")
	return cmd
}

// ── prodops decision ──────────────────────────────────────────────────────────

func cmdDecision() *cobra.Command {
	var moduleID, actID, optionID string
	cmd := &cobra.Command{
		Use:   "decision",
		Short: "Make a decision in the current act",
		RunE: func(cmd *cobra.Command, args []string) error {
			if moduleID == "" || actID == "" || optionID == "" {
				return fmt.Errorf("--module, --act, and --option are required")
			}
			body, err := apiPost(
				fmt.Sprintf("/api/v1/modules/%s/acts/%s/decision", moduleID, actID),
				map[string]string{"option_id": optionID})
			if err != nil {
				return err
			}
			printDecisionResult(body)
			return nil
		},
	}
	cmd.Flags().StringVar(&moduleID, "module", "", "Module ID")
	cmd.Flags().StringVar(&actID, "act", "", "Act ID")
	cmd.Flags().StringVar(&optionID, "option", "", "Option ID")
	return cmd
}

// ── prodops progress ──────────────────────────────────────────────────────────

func cmdProgress() *cobra.Command {
	return &cobra.Command{
		Use:   "progress",
		Short: "Show scores, streaks, unlocks, and yearly review flags",
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := apiGet("/api/v1/progress")
			if err != nil {
				return err
			}
			var summary struct {
				PerformScore int `json:"PerformScore"`
				XPTotal      int `json:"XPTotal"`
				Flags        []struct {
					FlagID   string `json:"FlagID"`
					Severity string `json:"Severity"`
					Note     string `json:"Note"`
					IsOffset bool   `json:"IsOffset"`
				} `json:"Flags"`
				ModuleProgress []struct {
					Module        struct{ Name string `json:"name"` } `json:"Module"`
					DisplayStatus string `json:"DisplayStatus"`
					Progress      *struct {
						FinalScore *int `json:"FinalScore"`
					} `json:"Progress"`
				} `json:"ModuleProgress"`
			}
			json.Unmarshal(body, &summary)

			fmt.Printf("\n📊 Performance Score: %+d   XP: %d\n\n", summary.PerformScore, summary.XPTotal)
			fmt.Printf("%-22s %s\n", "MODULE", "STATUS")
			fmt.Println(strings.Repeat("─", 40))
			for _, m := range summary.ModuleProgress {
				icon := statusIcon(m.DisplayStatus)
				score := ""
				if m.Progress != nil && m.Progress.FinalScore != nil {
					score = fmt.Sprintf("  (%d%%)", *m.Progress.FinalScore)
				}
				fmt.Printf("%-22s %s %s%s\n", m.Module.Name, icon, m.DisplayStatus, score)
			}

			if len(summary.Flags) > 0 {
				fmt.Println("\n⚠️  Yearly Review Flags:")
				for _, f := range summary.Flags {
					offset := ""
					if f.IsOffset {
						offset = " [offset]"
					}
					fmt.Printf("  [%s] %s — %s%s\n", f.Severity, f.FlagID, f.Note, offset)
				}
			}
			return nil
		},
	}
}

// ── prodops sync ──────────────────────────────────────────────────────────────

func cmdSync() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Export progress to git-tracked directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := apiPost("/api/v1/sync", nil)
			if err != nil {
				return err
			}
			var resp struct {
				SyncDir string `json:"sync_dir"`
				Note    string `json:"note"`
			}
			json.Unmarshal(body, &resp)
			fmt.Printf("✅ Progress exported to %s\n", resp.SyncDir)
			if resp.Note != "" {
				fmt.Println("  " + resp.Note)
			}
			return nil
		},
	}
}

// ── prodops config ────────────────────────────────────────────────────────────

func cmdConfig() *cobra.Command {
	cfg := &cobra.Command{Use: "config", Short: "Get or set configuration values"}

	cfg.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all config keys",
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := apiGet("/api/v1/config")
			if err != nil {
				return err
			}
			fmt.Println(string(body))
			return nil
		},
	})

	cfg.AddCommand(&cobra.Command{
		Use:   "get [key]",
		Short: "Get a config value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := apiGet("/api/v1/config/" + args[0])
			if err != nil {
				return err
			}
			var resp struct {
				Key   string `json:"key"`
				Value any    `json:"value"`
			}
			json.Unmarshal(body, &resp)
			fmt.Printf("%s = %v\n", resp.Key, resp.Value)
			return nil
		},
	})

	cfg.AddCommand(&cobra.Command{
		Use:   "set [key] [value]",
		Short: "Set a config value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := apiPut("/api/v1/config/"+args[0],
				map[string]any{"value": args[1]})
			if err != nil {
				return err
			}
			fmt.Printf("✅ %s = %s\n", args[0], args[1])
			return nil
		},
	})

	return cfg
}

// ── prodops setup ─────────────────────────────────────────────────────────────

func cmdSetup() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Run the first-time setup wizard",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Welcome to ProdOps Chronicles!")
			fmt.Print("Enter your display name: ")
			var name string
			fmt.Scanln(&name)
			_, err := apiPost("/api/v1/player", map[string]string{"display_name": name})
			if err != nil {
				return err
			}
			_, err = apiPost("/api/v1/runs", nil)
			if err != nil {
				return err
			}
			fmt.Println("✅ Setup complete. Run 'prodops lesson next' to begin.")
			return nil
		},
	}
}

// ── prodops version ───────────────────────────────────────────────────────────

func cmdVersion() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the ProdOps version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("prodops v1.0.0")
		},
	}
}

// ── HTTP helpers ──────────────────────────────────────────────────────────────

func apiGet(path string) ([]byte, error) {
	return apiRequest(http.MethodGet, path, nil)
}

func apiPost(path string, body any) ([]byte, error) {
	return apiRequest(http.MethodPost, path, body)
}

func apiPut(path string, body any) ([]byte, error) {
	return apiRequest(http.MethodPut, path, body)
}

func apiRequest(method, path string, body any) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, backendURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("backend unreachable at %s — is the stack running?", backendURL)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		var errResp struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Error.Message != "" {
			return nil, fmt.Errorf("[%s] %s", errResp.Error.Code, errResp.Error.Message)
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// ── Docker Compose helpers ────────────────────────────────────────────────────

func dockerCompose(args ...string) error {
	cmdArgs := append([]string{"compose"}, args...)
	cmd := exec.Command("docker", cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func waitHealthy(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(backendURL + "/health")
		if err == nil && resp.StatusCode == 200 {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("backend did not become healthy within %s", timeout)
}

// ── Print helpers ─────────────────────────────────────────────────────────────

func printAct(body []byte, moduleID string) {
	var resp struct {
		Act struct {
			ID        string `json:"ID"`
			Title     string `json:"Title"`
			Narrative string `json:"Narrative"`
			Tasks     []struct {
				ID    string `json:"ID"`
				Title string `json:"Title"`
				Hints []string `json:"Hints"`
			} `json:"Tasks"`
			Decision *struct {
				Prompt  string `json:"Prompt"`
				Options []struct {
					ID    string `json:"ID"`
					Label string `json:"Label"`
				} `json:"Options"`
			} `json:"Decision"`
		} `json:"act"`
	}
	json.Unmarshal(body, &resp)
	act := resp.Act

	fmt.Printf("\n📖 %s\n\n", act.Title)
	fmt.Println(wordWrap(act.Narrative, 80))
	fmt.Println()

	if len(act.Tasks) > 0 {
		fmt.Println("🔧 Tasks:")
		for i, t := range act.Tasks {
			fmt.Printf("  %d. %s\n", i+1, t.Title)
		}
		fmt.Printf("\nRun: prodops verify --module %s\n", moduleID)
	}

	if act.Decision != nil {
		fmt.Printf("\n🤔 Decision: %s\n", act.Decision.Prompt)
		for _, o := range act.Decision.Options {
			fmt.Printf("  [%s] %s\n", o.ID, o.Label)
		}
		fmt.Printf("\nRun: prodops decision --module %s --act %s --option <id>\n", moduleID, act.ID)
	}
}

func printVerifyResult(body []byte) {
	var result struct {
		Passed        bool   `json:"Passed"`
		AlreadyLocked bool   `json:"AlreadyLocked"`
		AttemptCount  int    `json:"AttemptCount"`
		XPAwarded     int    `json:"XPAwarded"`
		AllTasksDone  bool   `json:"AllTasksDone"`
		CheckResults  []struct {
			Type   string `json:"Type"`
			Passed bool   `json:"Passed"`
			Detail string `json:"Detail"`
		} `json:"CheckResults"`
	}
	json.Unmarshal(body, &result)

	if result.AlreadyLocked {
		fmt.Println("✅ Task already verified.")
		return
	}
	if result.Passed {
		fmt.Printf("✅ Task passed! +%d XP\n", result.XPAwarded)
		if result.AllTasksDone {
			fmt.Println("🎯 All tasks complete. Make your decision with 'prodops decision'.")
		}
	} else {
		fmt.Printf("❌ Not yet. (attempt %d)\n", result.AttemptCount)
		for _, c := range result.CheckResults {
			icon := "✅"
			if !c.Passed {
				icon = "❌"
			}
			fmt.Printf("  %s [%s]", icon, c.Type)
			if !c.Passed && c.Detail != "" {
				fmt.Printf(" — %s", c.Detail)
			}
			fmt.Println()
		}
	}
}

func printDecisionResult(body []byte) {
	var result struct {
		Narrative  string `json:"Narrative"`
		NewTracker struct {
			Morale           int    `json:"Morale"`
			IncidentSeverity string `json:"IncidentSeverity"`
			TechnicalDebt    int    `json:"TechnicalDebt"`
		} `json:"NewTracker"`
		NextActID string `json:"NextActID"`
		Ending    *struct {
			Title     string `json:"Title"`
			Narrative string `json:"Narrative"`
		} `json:"Ending"`
	}
	json.Unmarshal(body, &result)

	fmt.Println()
	fmt.Println(wordWrap(result.Narrative, 80))
	fmt.Printf("\n📊 Morale: %d | Severity: %s | Debt: %d\n",
		result.NewTracker.Morale,
		result.NewTracker.IncidentSeverity,
		result.NewTracker.TechnicalDebt)

	if result.Ending != nil {
		fmt.Printf("\n🏁 %s\n\n%s\n", result.Ending.Title,
			wordWrap(result.Ending.Narrative, 80))
	} else if result.NextActID != "" {
		fmt.Println("\n▶ Run 'prodops lesson next' to continue.")
	}
}

func statusIcon(s string) string {
	switch s {
	case "completed":
		return "✅"
	case "in_progress":
		return "🔄"
	case "available":
		return "🔓"
	default:
		return "🔒"
	}
}

func wordWrap(s string, width int) string {
	words := strings.Fields(s)
	var lines []string
	line := ""
	for _, w := range words {
		if len(line)+len(w)+1 > width {
			lines = append(lines, line)
			line = w
		} else {
			if line != "" {
				line += " "
			}
			line += w
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}
