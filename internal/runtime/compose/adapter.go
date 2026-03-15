package compose

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/prodops-chronicles/prodops/internal/runtime"
)

// Adapter implements runtime.RuntimeAdapter using Docker Compose.
type Adapter struct {
	composeFile string
	projectDir  string
	imagePrefix string // e.g. "prodops"
	imageTag    string // e.g. "1.0.0"
}

func New(composeFile, projectDir, imagePrefix, imageTag string) *Adapter {
	return &Adapter{
		composeFile: composeFile,
		projectDir:  projectDir,
		imagePrefix: imagePrefix,
		imageTag:    imageTag,
	}
}

func (a *Adapter) StartModule(ctx context.Context, moduleID string) error {
	return a.run(ctx, "up", "-d", "--no-deps", podName(moduleID))
}

func (a *Adapter) StopModule(ctx context.Context, moduleID string) error {
	return a.run(ctx, "stop", podName(moduleID))
}

func (a *Adapter) ModuleStatus(ctx context.Context, moduleID string) (runtime.PodStatus, error) {
	statuses, err := a.ListRunning(ctx)
	if err != nil {
		return runtime.PodStatus{}, err
	}
	for _, s := range statuses {
		if s.ModuleID == moduleID {
			return s, nil
		}
	}
	return runtime.PodStatus{ModuleID: moduleID, Running: false, Healthy: false}, nil
}

type composePS struct {
	Name    string `json:"Name"`
	State   string `json:"State"`
	Health  string `json:"Health"`
	Image   string `json:"Image"`
}

func (a *Adapter) ListRunning(ctx context.Context) ([]runtime.PodStatus, error) {
	cmd := exec.CommandContext(ctx, "docker", "compose",
		"-f", a.composeFile, "ps", "--format", "json")
	cmd.Dir = a.projectDir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("docker compose ps: %w", err)
	}

	var services []composePS
	// docker compose ps --format json outputs one JSON object per line
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		var s composePS
		if err := json.Unmarshal([]byte(line), &s); err != nil {
			continue
		}
		services = append(services, s)
	}

	var statuses []runtime.PodStatus
	for _, s := range services {
		if !strings.HasPrefix(s.Name, "prodops-") {
			continue
		}
		// strip "prodops-" prefix to get moduleID
		moduleID := strings.TrimPrefix(s.Name, "prodops-")
		// skip core services (backend, db)
		if moduleID == "backend" || moduleID == "db" {
			continue
		}
		statuses = append(statuses, runtime.PodStatus{
			ModuleID: moduleID,
			Running:  s.State == "running",
			Healthy:  s.Health == "healthy",
			ImageTag: s.Image,
		})
	}
	return statuses, nil
}

func (a *Adapter) WriteModuleDefinition(ctx context.Context, moduleID string) error {
	serviceBlock, err := a.renderServiceBlock(moduleID)
	if err != nil {
		return err
	}

	// Read current compose file
	current, err := os.ReadFile(a.composeFile)
	if err != nil {
		return err
	}

	// Idempotent: check if service already defined
	if strings.Contains(string(current), fmt.Sprintf("container_name: prodops-%s", moduleID)) {
		return nil
	}

	// Append service block
	updated := strings.TrimRight(string(current), "\n") + "\n" + serviceBlock + "\n"
	return os.WriteFile(a.composeFile, []byte(updated), 0644)
}

func (a *Adapter) RemoveModuleDefinition(ctx context.Context, moduleID string) error {
	current, err := os.ReadFile(a.composeFile)
	if err != nil {
		return err
	}

	// Remove lines belonging to this service block
	// Strategy: find "  prodops-<moduleID>:" block and remove until next top-level service
	lines := strings.Split(string(current), "\n")
	var result []string
	skip := false
	target := fmt.Sprintf("  %s:", podName(moduleID))

	for i, line := range lines {
		if line == target {
			skip = true
			// also remove trailing blank line before this block
			if len(result) > 0 && result[len(result)-1] == "" {
				result = result[:len(result)-1]
			}
			continue
		}
		if skip {
			// stop skipping at next top-level service (2-space indent + non-space)
			if i > 0 && len(line) > 2 && line[0] == ' ' && line[1] == ' ' && line[2] != ' ' {
				skip = false
			} else if len(line) > 0 && line[0] != ' ' {
				skip = false
			}
		}
		if !skip {
			result = append(result, line)
		}
	}

	return os.WriteFile(a.composeFile, []byte(strings.Join(result, "\n")), 0644)
}

// renderServiceBlock generates the YAML service block for a module pod.
func (a *Adapter) renderServiceBlock(moduleID string) (string, error) {
	tmpl := `
  {{.PodName}}:
    image: {{.ImagePrefix}}/module-{{.ModuleID}}:{{.ImageTag}}
    container_name: {{.PodName}}
    restart: unless-stopped
    environment:
      MODULE_ID:   "{{.ModuleID}}"
      BACKEND_URL: "http://backend:7741"
    volumes:
      - /opt/prodops/modules/{{.ModuleID}}:/module/state
      - /home/${USER}:/home/learner:ro
      {{- if eq .ModuleID "linux-cli"}}
      - /var/log:/var/log/host:ro
      {{- end}}
    networks:
      - prodops-net
    depends_on:
      - backend
    expose:
      - "8080"
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 10s
      timeout:  5s
      retries:  3`

	t, err := template.New("service").Parse(tmpl)
	if err != nil {
		return "", err
	}

	data := map[string]string{
		"PodName":     podName(moduleID),
		"ModuleID":    moduleID,
		"ImagePrefix": a.imagePrefix,
		"ImageTag":    a.imageTag,
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (a *Adapter) run(ctx context.Context, args ...string) error {
	base := []string{"compose", "-f", filepath.Base(a.composeFile)}
	cmd := exec.CommandContext(ctx, "docker", append(base, args...)...)
	cmd.Dir = a.projectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker compose %s: %w\n%s", strings.Join(args, " "), err, out)
	}
	return nil
}

func podName(moduleID string) string {
	return "prodops-" + moduleID
}
