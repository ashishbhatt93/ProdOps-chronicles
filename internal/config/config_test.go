package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/prodops-chronicles/prodops/internal/config"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "base_configs.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoad_Minimal(t *testing.T) {
	path := writeConfig(t, `
storage:
  base_path: /opt/prodops
difficulty: d1
runtime: compose
`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Storage.BasePath != "/opt/prodops" {
		t.Errorf("expected base_path /opt/prodops, got %s", cfg.Storage.BasePath)
	}
	if cfg.Difficulty != config.D1 {
		t.Errorf("expected D1, got %s", cfg.Difficulty)
	}
	// Sub-paths should be derived automatically
	if cfg.Storage.PgdataPath != "/opt/prodops/pgdata" {
		t.Errorf("expected derived pgdata_path, got %s", cfg.Storage.PgdataPath)
	}
}

func TestLoad_DifficultyParsing(t *testing.T) {
	cases := []struct {
		input    string
		expected config.Difficulty
	}{
		{"d1", config.D1},
		{"d2", config.D2},
		{"d3", config.D3},
	}
	for _, c := range cases {
		d, err := config.ParseDifficulty(c.input)
		if err != nil {
			t.Errorf("ParseDifficulty(%q): unexpected error: %v", c.input, err)
		}
		if d != c.expected {
			t.Errorf("ParseDifficulty(%q) = %v, want %v", c.input, d, c.expected)
		}
	}
}

func TestLoad_ModuleEnabled(t *testing.T) {
	path := writeConfig(t, `
storage:
  base_path: /opt/prodops
runtime: compose
difficulty: d1
modules:
  linux-cli:
    enabled: true
    min_difficulty: d1
  terraform:
    enabled: true
    min_difficulty: d2
  cicd:
    enabled: false
`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if !cfg.IsModuleEnabled("linux-cli") {
		t.Error("linux-cli should be enabled for d1")
	}
	if cfg.IsModuleEnabled("terraform") {
		t.Error("terraform requires d2, should not be enabled for d1 player")
	}
	if cfg.IsModuleEnabled("cicd") {
		t.Error("cicd is explicitly disabled")
	}
}

func TestLoad_ModuleEnabled_D2Player(t *testing.T) {
	path := writeConfig(t, `
storage:
  base_path: /opt/prodops
runtime: compose
difficulty: d2
modules:
  linux-cli:
    enabled: true
    min_difficulty: d1
  terraform:
    enabled: true
    min_difficulty: d2
  ansible:
    enabled: true
    min_difficulty: d3
`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if !cfg.IsModuleEnabled("linux-cli") {
		t.Error("d2 player should see d1 modules")
	}
	if !cfg.IsModuleEnabled("terraform") {
		t.Error("d2 player should see d2 modules")
	}
	if cfg.IsModuleEnabled("ansible") {
		t.Error("d2 player should not see d3 modules")
	}
}

func TestLoad_MaxHintLevel(t *testing.T) {
	cases := []struct {
		difficulty config.Difficulty
		want       int
	}{
		{config.D1, 3},
		{config.D2, 2},
		{config.D3, 1},
	}
	for _, c := range cases {
		path := writeConfig(t, "storage:\n  base_path: /opt/prodops\nruntime: compose\ndifficulty: "+string(c.difficulty)+"\n")
		cfg, err := config.Load(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := cfg.MaxHintLevel(); got != c.want {
			t.Errorf("MaxHintLevel for %s = %d, want %d", c.difficulty, got, c.want)
		}
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	path := writeConfig(t, `
storage:
  base_path: /opt/prodops
runtime: compose
difficulty: d1
`)
	t.Setenv("PRODOPS_BASE_PATH", "/custom/path")
	t.Setenv("PRODOPS_DIFFICULTY", "d3")

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Storage.BasePath != "/custom/path" {
		t.Errorf("env override for base_path failed, got %s", cfg.Storage.BasePath)
	}
	if cfg.Difficulty != config.D3 {
		t.Errorf("env override for difficulty failed, got %s", cfg.Difficulty)
	}
}

func TestLoad_SubPathsNotOverwrittenIfExplicit(t *testing.T) {
	path := writeConfig(t, `
storage:
  base_path: /opt/prodops
  pgdata_path: /data/custom/pgdata
runtime: compose
difficulty: d1
`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Storage.PgdataPath != "/data/custom/pgdata" {
		t.Errorf("explicit pgdata_path should not be overwritten by derivation, got %s", cfg.Storage.PgdataPath)
	}
}
