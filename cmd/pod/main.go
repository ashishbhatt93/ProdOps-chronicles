package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/prodops-chronicles/prodops/internal/content"
	"github.com/prodops-chronicles/prodops/internal/pod"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	moduleID := os.Getenv("MODULE_ID")
	if moduleID == "" {
		slog.Error("MODULE_ID env var is required")
		os.Exit(1)
	}

	backendURL := os.Getenv("BACKEND_URL")
	if backendURL == "" {
		backendURL = "http://backend:7741"
	}

	token := os.Getenv("BACKEND_TOKEN")
	if token == "" {
		slog.Error("BACKEND_TOKEN env var is required")
		os.Exit(1)
	}

	contentRoot := os.Getenv("CONTENT_ROOT")
	if contentRoot == "" {
		contentRoot = "./content"
	}

	// ── Load + validate YAML content ─────────────────────────────────────────
	loader := content.NewLoader(contentRoot)
	mc, err := loader.Load()
	if err != nil {
		slog.Error("failed to load module content", "err", err)
		os.Exit(1)
	}
	slog.Info("content loaded", "module", mc.ID, "version", mc.Version, "acts", len(mc.Acts))

	// ── Seed backend (with retry) ─────────────────────────────────────────────
	seeder := pod.NewSeeder(backendURL, token)
	if err := seeder.Seed(moduleID, mc); err != nil {
		slog.Error("seed failed", "err", err)
		os.Exit(1)
	}
	slog.Info("content seeded to backend")

	// ── Start HTTP server ─────────────────────────────────────────────────────
	server := pod.NewServer(mc)
	addr := ":8080"
	slog.Info("pod listening", "module", moduleID, "addr", addr)
	if err := http.ListenAndServe(addr, server); err != nil {
		slog.Error("pod server error", "err", err)
		os.Exit(1)
	}
}
