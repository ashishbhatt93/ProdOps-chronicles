package handler

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/prodops-chronicles/prodops/internal/domain"
	"github.com/prodops-chronicles/prodops/internal/service"
)

type SyncHandler struct {
	svc    *service.SyncService
	runSvc *service.RunService
	players interface {
		GetProfile(ctx interface{}, playerID interface{}) (*domain.PlayerProfile, error)
	}
	syncDir string // /opt/prodops/sync or similar
}

func NewSyncHandler(svc *service.SyncService, runSvc *service.RunService, syncDir string) *SyncHandler {
	return &SyncHandler{svc: svc, runSvc: runSvc, syncDir: syncDir}
}

// POST /api/v1/sync
func (h *SyncHandler) Sync(w http.ResponseWriter, r *http.Request) {
	run, err := h.runSvc.GetActiveRun(r.Context())
	if err != nil {
		handleErr(w, err)
		return
	}

	runs, err := h.runSvc.ListRuns(r.Context())
	if err != nil {
		handleErr(w, err)
		return
	}
	runNumber := len(runs)

	export, err := h.svc.ExportProgressSnapshot(r.Context(), run, runNumber)
	if err != nil {
		handleErr(w, err)
		return
	}

	// Write progress.json
	jsonBytes, err := service.SerialiseToJSON(export)
	if err != nil {
		handleErr(w, err)
		return
	}
	if err := os.MkdirAll(h.syncDir, 0755); err != nil {
		handleErr(w, err)
		return
	}
	if err := os.WriteFile(filepath.Join(h.syncDir, "progress.json"), jsonBytes, 0644); err != nil {
		handleErr(w, err)
		return
	}

	// Write PROGRESS.md
	md := service.SerialiseToMarkdown(export)
	if err := os.WriteFile(filepath.Join(h.syncDir, "PROGRESS.md"), []byte(md), 0644); err != nil {
		handleErr(w, err)
		return
	}

	respond(w, http.StatusOK, map[string]any{
		"status":     "exported",
		"exported_at": time.Now().UTC(),
		"sync_dir":   h.syncDir,
		"note":       fmt.Sprintf("Run 'git push' in %s to sync to remote", h.syncDir),
	})
}
