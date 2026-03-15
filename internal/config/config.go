// Package config loads base_configs.yaml at startup and provides a typed
// Config struct to every component that needs it.
//
// Priority (highest first):
//  1. Environment variable (always wins — useful in CI/containers)
//  2. base_configs.yaml value
//  3. Compiled default (only for non-path values)
//
// Paths are NEVER hardcoded anywhere else in the codebase.
// All path constants live here.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ── Difficulty levels ─────────────────────────────────────────────────────────

// Difficulty is the internal representation of the learner's experience level.
type Difficulty string

const (
	// D1 targets practicing DevOps Engineers — standard module difficulty,
	// full hints available, shorter time pressure windows.
	D1 Difficulty = "d1"

	// D2 targets Senior DevOps Engineers — reduced hints, tighter incident
	// windows, additional complexity in task verification.
	D2 Difficulty = "d2"

	// D3 targets DevOps Team Leads — minimal hints, architectural decisions
	// surface earlier, yearly review scoring is stricter.
	D3 Difficulty = "d3"
)

// DifficultyLabel returns the human-readable label for a difficulty level.
func DifficultyLabel(d Difficulty) string {
	switch d {
	case D1:
		return "DevOps Engineer"
	case D2:
		return "Senior DevOps Engineer"
	case D3:
		return "DevOps Team Lead"
	default:
		return string(d)
	}
}

// ParseDifficulty converts a string (from YAML or env var) to a Difficulty.
// Accepts both internal codes ("d1") and friendly labels ("devops engineer").
func ParseDifficulty(s string) (Difficulty, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "d1", "devops engineer", "devops_engineer":
		return D1, nil
	case "d2", "senior devops engineer", "senior_devops_engineer":
		return D2, nil
	case "d3", "devops team lead", "devops_team_lead":
		return D3, nil
	default:
		return D1, fmt.Errorf("unknown difficulty %q: valid values are d1, d2, d3", s)
	}
}

// ── Config structs (map directly to base_configs.yaml) ───────────────────────

// Config is the fully parsed, validated configuration for a ProdOps installation.
type Config struct {
	System     SystemConfig             `yaml:"system"`
	Storage    StorageConfig            `yaml:"storage"`
	Versions   VersionsConfig           `yaml:"versions"`
	Network    NetworkConfig            `yaml:"network"`
	Runtime    string                   `yaml:"runtime"`    // compose | k3s
	Difficulty Difficulty               `yaml:"difficulty"` // d1 | d2 | d3
	Modules    map[string]ModuleConfig  `yaml:"modules"`
	AI         AIConfig                 `yaml:"ai"`
	Telemetry  TelemetryConfig          `yaml:"telemetry"`
}

type SystemConfig struct {
	PostgresUID  int    `yaml:"postgres_uid"`
	PostgresGID  int    `yaml:"postgres_gid"`
	ProdopsUser  string `yaml:"prodops_user"`
	ProdopsUID   int    `yaml:"prodops_uid"`
	ProdopsGID   int    `yaml:"prodops_gid"`
}

type StorageConfig struct {
	BasePath    string `yaml:"base_path"`
	PgdataPath  string `yaml:"pgdata_path"`
	ModulesPath string `yaml:"modules_path"`
	SyncPath    string `yaml:"sync_path"`
	LogsPath    string `yaml:"logs_path"`
	BackendPath string `yaml:"backend_path"`
}

type VersionsConfig struct {
	Postgres   string `yaml:"postgres"`
	Backend    string `yaml:"backend"`
	Gitea      string `yaml:"gitea"`
	Woodpecker string `yaml:"woodpecker"`
}

type NetworkConfig struct {
	ProdopsSubnet string `yaml:"prodops_subnet"`
	DNSDomain     string `yaml:"dns_domain"`
}

// ModuleConfig controls whether a module is available on this installation.
// A module with enabled: false is never written to disk and never starts.
// Difficulty gates which modules appear based on the player's level.
type ModuleConfig struct {
	Enabled           bool       `yaml:"enabled"`
	MinDifficulty     Difficulty `yaml:"min_difficulty"`     // default: d1 (all levels)
	Port              *int       `yaml:"port"`               // nil = no exposed port
	ScoreThresholdOverride *int  `yaml:"score_threshold"`    // nil = use module YAML default
}

type AIConfig struct {
	Provider string `yaml:"provider"` // ollama | claude | openai | gemini
	APIKey   string `yaml:"api_key"`
}

type TelemetryConfig struct {
	Enabled bool   `yaml:"enabled"`
	Level   string `yaml:"level"` // tier1 | tier2 | tier3
}

// ── Computed paths (derived from StorageConfig) ───────────────────────────────

// ComposeFile returns the absolute path to docker-compose.yml.
func (c *Config) ComposeFile() string {
	return filepath.Join(c.Storage.BasePath, "docker-compose.yml")
}

// DatabaseURL constructs the postgres connection string.
// The password is expected via the DATABASE_PASSWORD env var.
func (c *Config) DatabaseURL() string {
	if url := os.Getenv("DATABASE_URL"); url != "" {
		return url
	}
	pass := os.Getenv("DB_PASSWORD")
	return fmt.Sprintf("postgres://prodops_app:%s@localhost:5432/prodops?sslmode=disable", pass)
}

// BackendToken returns the bearer token for backend auth.
func (c *Config) BackendToken() string {
	return os.Getenv("BACKEND_TOKEN")
}

// ── Loader ────────────────────────────────────────────────────────────────────

// Load reads base_configs.yaml from path, applies environment variable overrides,
// validates required fields, and returns the parsed Config.
func Load(path string) (*Config, error) {
	// Resolve path — default to the value of CONFIG_PATH env var if not provided
	if path == "" {
		path = os.Getenv("CONFIG_PATH")
	}
	if path == "" {
		return nil, fmt.Errorf("config path not provided and CONFIG_PATH env var not set")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	applyDefaults(&cfg)
	applyEnvOverrides(&cfg)

	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

// ── Defaults ──────────────────────────────────────────────────────────────────

func applyDefaults(cfg *Config) {
	if cfg.Runtime == "" {
		cfg.Runtime = "compose"
	}
	if cfg.Difficulty == "" {
		cfg.Difficulty = D1
	}
	if cfg.System.PostgresUID == 0 {
		cfg.System.PostgresUID = 999
	}
	if cfg.System.PostgresGID == 0 {
		cfg.System.PostgresGID = 999
	}
	if cfg.System.ProdopsUID == 0 {
		cfg.System.ProdopsUID = 1000
	}
	if cfg.System.ProdopsUser == "" {
		cfg.System.ProdopsUser = "prodops"
	}
	if cfg.Versions.Postgres == "" {
		cfg.Versions.Postgres = "15.4"
	}
	if cfg.AI.Provider == "" {
		cfg.AI.Provider = "ollama"
	}
	if cfg.Telemetry.Level == "" {
		cfg.Telemetry.Level = "tier1"
	}

	// Derive sub-paths from base_path if not explicitly set
	base := cfg.Storage.BasePath
	if base != "" {
		if cfg.Storage.PgdataPath == "" {
			cfg.Storage.PgdataPath = filepath.Join(base, "pgdata")
		}
		if cfg.Storage.ModulesPath == "" {
			cfg.Storage.ModulesPath = filepath.Join(base, "modules")
		}
		if cfg.Storage.SyncPath == "" {
			cfg.Storage.SyncPath = filepath.Join(base, "sync")
		}
		if cfg.Storage.LogsPath == "" {
			cfg.Storage.LogsPath = filepath.Join(base, "logs")
		}
		if cfg.Storage.BackendPath == "" {
			cfg.Storage.BackendPath = filepath.Join(base, "backend")
		}
	}

	// Default module min_difficulty to d1 (available to everyone)
	for id, m := range cfg.Modules {
		if m.MinDifficulty == "" {
			m.MinDifficulty = D1
		}
		cfg.Modules[id] = m
	}
}

// ── Environment overrides ─────────────────────────────────────────────────────
// Env vars let operators override specific values without editing the file.
// Useful for Docker/k3s deployments where the config is mounted read-only.

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("PRODOPS_BASE_PATH"); v != "" {
		cfg.Storage.BasePath = v
	}
	if v := os.Getenv("PRODOPS_PGDATA_PATH"); v != "" {
		cfg.Storage.PgdataPath = v
	}
	if v := os.Getenv("PRODOPS_MODULES_PATH"); v != "" {
		cfg.Storage.ModulesPath = v
	}
	if v := os.Getenv("PRODOPS_SYNC_PATH"); v != "" {
		cfg.Storage.SyncPath = v
	}
	if v := os.Getenv("RUNTIME"); v != "" {
		cfg.Runtime = v
	}
	if v := os.Getenv("PRODOPS_DIFFICULTY"); v != "" {
		if d, err := ParseDifficulty(v); err == nil {
			cfg.Difficulty = d
		}
	}
	if v := os.Getenv("AI_PROVIDER"); v != "" {
		cfg.AI.Provider = v
	}
	if v := os.Getenv("AI_API_KEY"); v != "" {
		cfg.AI.APIKey = v
	}
	if v := os.Getenv("TELEMETRY_ENABLED"); v != "" {
		cfg.Telemetry.Enabled = v == "true" || v == "1"
	}
}

// ── Validation ────────────────────────────────────────────────────────────────

func validate(cfg *Config) error {
	if cfg.Storage.BasePath == "" {
		return fmt.Errorf("storage.base_path is required")
	}
	if cfg.Runtime != "compose" && cfg.Runtime != "k3s" {
		return fmt.Errorf("runtime must be 'compose' or 'k3s', got %q", cfg.Runtime)
	}
	if cfg.Difficulty != D1 && cfg.Difficulty != D2 && cfg.Difficulty != D3 {
		return fmt.Errorf("difficulty must be d1, d2, or d3, got %q", cfg.Difficulty)
	}
	return nil
}

// ── Module availability helpers ───────────────────────────────────────────────

// IsModuleEnabled returns true if the module is both enabled in base_configs
// and accessible at the current difficulty level.
func (c *Config) IsModuleEnabled(moduleID string) bool {
	m, ok := c.Modules[moduleID]
	if !ok {
		return false
	}
	if !m.Enabled {
		return false
	}
	return difficultyAtLeast(c.Difficulty, m.MinDifficulty)
}

// EnabledModuleIDs returns all module IDs that are enabled and accessible
// at the current difficulty level, in no particular order.
func (c *Config) EnabledModuleIDs() []string {
	var ids []string
	for id := range c.Modules {
		if c.IsModuleEnabled(id) {
			ids = append(ids, id)
		}
	}
	return ids
}

// difficultyAtLeast returns true if current >= required.
func difficultyAtLeast(current, required Difficulty) bool {
	return difficultyRank(current) >= difficultyRank(required)
}

func difficultyRank(d Difficulty) int {
	switch d {
	case D1:
		return 1
	case D2:
		return 2
	case D3:
		return 3
	default:
		return 1
	}
}

// ── Port helpers ──────────────────────────────────────────────────────────────

// ModulePort returns the host port for a module, or 0 if none.
func (c *Config) ModulePort(moduleID string) int {
	m, ok := c.Modules[moduleID]
	if !ok || m.Port == nil {
		return 0
	}
	return *m.Port
}

// ── Score threshold helpers ───────────────────────────────────────────────────

// ScoreThreshold returns the score threshold for a module, checking for an
// override in base_configs before falling back to the provided default.
func (c *Config) ScoreThreshold(moduleID string, defaultThreshold int) int {
	m, ok := c.Modules[moduleID]
	if !ok || m.ScoreThresholdOverride == nil {
		return defaultThreshold
	}
	return *m.ScoreThresholdOverride
}

// ── Difficulty-aware hint level ───────────────────────────────────────────────

// MaxHintLevel returns the maximum hint level the player can access at their
// difficulty. D1 = all 3 hints, D2 = first 2 hints, D3 = first hint only.
func (c *Config) MaxHintLevel() int {
	switch c.Difficulty {
	case D3:
		return 1
	case D2:
		return 2
	default:
		return 3
	}
}

// ── String/debug helpers ──────────────────────────────────────────────────────

func (c *Config) DifficultyLabel() string {
	return DifficultyLabel(c.Difficulty)
}

// ParseInt is a small utility used by env override parsing.
func ParseInt(s string) (int, error) {
	return strconv.Atoi(s)
}
