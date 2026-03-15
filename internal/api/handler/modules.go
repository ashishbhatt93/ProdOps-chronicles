package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/prodops-chronicles/prodops/internal/service"
)

type ModuleHandler struct {
	svc    *service.ModuleService
	runSvc *service.RunService
}

func NewModuleHandler(svc *service.ModuleService, runSvc *service.RunService) *ModuleHandler {
	return &ModuleHandler{svc: svc, runSvc: runSvc}
}

// GET /api/v1/modules
func (h *ModuleHandler) List(w http.ResponseWriter, r *http.Request) {
	run, err := h.runSvc.GetActiveRun(r.Context())
	if err != nil {
		handleErr(w, err)
		return
	}
	entries, err := h.svc.ListModules(r.Context(), run.ID)
	if err != nil {
		handleErr(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]any{"modules": entries})
}

// GET /api/v1/modules/:module_id
func (h *ModuleHandler) Get(w http.ResponseWriter, r *http.Request) {
	moduleID := chi.URLParam(r, "module_id")
	run, err := h.runSvc.GetActiveRun(r.Context())
	if err != nil {
		handleErr(w, err)
		return
	}
	entry, err := h.svc.GetModuleStatus(r.Context(), run.ID, moduleID)
	if err != nil {
		handleErr(w, err)
		return
	}
	respond(w, http.StatusOK, entry)
}

// POST /api/v1/modules/:module_id/enable
func (h *ModuleHandler) Enable(w http.ResponseWriter, r *http.Request) {
	moduleID := chi.URLParam(r, "module_id")
	run, err := h.runSvc.GetActiveRun(r.Context())
	if err != nil {
		handleErr(w, err)
		return
	}
	if err := h.svc.EnableModule(r.Context(), run.ID, moduleID); err != nil {
		handleErr(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]string{"status": "enabled", "module_id": moduleID})
}

// POST /api/v1/modules/:module_id/disable
func (h *ModuleHandler) Disable(w http.ResponseWriter, r *http.Request) {
	moduleID := chi.URLParam(r, "module_id")
	if err := h.svc.DisableModule(r.Context(), moduleID); err != nil {
		handleErr(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]string{"status": "disabled", "module_id": moduleID})
}

// POST /api/v1/modules/:module_id/unlock
func (h *ModuleHandler) ForceUnlock(w http.ResponseWriter, r *http.Request) {
	moduleID := chi.URLParam(r, "module_id")
	var req struct {
		Force bool `json:"force"`
	}
	if err := decode(r, &req); err != nil || !req.Force {
		handleErr(w, wrapInvalidInput("body must contain {\"force\": true}"))
		return
	}
	run, err := h.runSvc.GetActiveRun(r.Context())
	if err != nil {
		handleErr(w, err)
		return
	}
	if err := h.svc.ForceUnlockModule(r.Context(), run.ID, moduleID); err != nil {
		handleErr(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]string{"status": "unlocked", "source": "forced"})
}

// internal helper used by multiple handlers
func parseRunID(r *http.Request) (uuid.UUID, error) {
	raw := chi.URLParam(r, "run_id")
	return uuid.Parse(raw)
}
