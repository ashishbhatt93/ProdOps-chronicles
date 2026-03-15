package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/prodops-chronicles/prodops/internal/content"
	"github.com/prodops-chronicles/prodops/internal/service"
)

type GameplayHandler struct {
	gameSvc  *service.GameService
	verifySvc *service.VerificationService
	runSvc   *service.RunService
	// contentCache shared between GameService and VerificationService
	contentCache map[string]*content.ModuleContent
}

func NewGameplayHandler(
	gameSvc *service.GameService,
	verifySvc *service.VerificationService,
	runSvc *service.RunService,
	contentCache map[string]*content.ModuleContent,
) *GameplayHandler {
	return &GameplayHandler{
		gameSvc:      gameSvc,
		verifySvc:    verifySvc,
		runSvc:       runSvc,
		contentCache: contentCache,
	}
}

// GET /api/v1/modules/:module_id/acts/current
func (h *GameplayHandler) GetCurrentAct(w http.ResponseWriter, r *http.Request) {
	moduleID := chi.URLParam(r, "module_id")
	run, err := h.runSvc.GetActiveRun(r.Context())
	if err != nil {
		handleErr(w, err)
		return
	}
	act, mp, err := h.gameSvc.GetCurrentAct(r.Context(), run.ID, moduleID)
	if err != nil {
		handleErr(w, err)
		return
	}
	// Filter hints to the player's difficulty level before sending
	filteredAct := *act
	for i := range filteredAct.Tasks {
		t := *filteredAct.Tasks[i]
		t.Hints = h.gameSvc.FilteredHints(t.Hints)
		filteredAct.Tasks[i] = &t
	}
	respond(w, http.StatusOK, map[string]any{
		"act":      filteredAct,
		"progress": mp,
	})
}

// GET /api/v1/modules/:module_id/acts
func (h *GameplayHandler) ListActs(w http.ResponseWriter, r *http.Request) {
	moduleID := chi.URLParam(r, "module_id")
	mc, ok := h.contentCache[moduleID]
	if !ok {
		handleErr(w, wrapNotFound("module content not loaded for "+moduleID))
		return
	}
	respond(w, http.StatusOK, map[string]any{"acts": mc.Acts})
}

// POST /api/v1/modules/:module_id/acts/:act_id/tasks/current/verify
// Returns 200 even when verification fails — execution errors return 500.
func (h *GameplayHandler) VerifyCurrentTask(w http.ResponseWriter, r *http.Request) {
	moduleID := chi.URLParam(r, "module_id")
	run, err := h.runSvc.GetActiveRun(r.Context())
	if err != nil {
		handleErr(w, err)
		return
	}
	result, err := h.verifySvc.VerifyTask(r.Context(), run.ID, moduleID, h.contentCache)
	if err != nil {
		// execution_error — 500
		handleErr(w, err)
		return
	}
	// 200 regardless of passed/failed
	respond(w, http.StatusOK, result)
}

// POST /api/v1/modules/:module_id/acts/:act_id/decision
func (h *GameplayHandler) MakeDecision(w http.ResponseWriter, r *http.Request) {
	moduleID := chi.URLParam(r, "module_id")
	actID := chi.URLParam(r, "act_id")
	var req struct {
		OptionID string `json:"option_id"`
	}
	if err := decode(r, &req); err != nil || req.OptionID == "" {
		handleErr(w, wrapInvalidInput("option_id is required"))
		return
	}
	run, err := h.runSvc.GetActiveRun(r.Context())
	if err != nil {
		handleErr(w, err)
		return
	}
	result, err := h.gameSvc.MakeDecision(r.Context(), run.ID, moduleID, actID, req.OptionID)
	if err != nil {
		handleErr(w, err)
		return
	}
	respond(w, http.StatusOK, result)
}

// POST /api/v1/modules/:module_id/replay
func (h *GameplayHandler) Replay(w http.ResponseWriter, r *http.Request) {
	moduleID := chi.URLParam(r, "module_id")
	run, err := h.runSvc.GetActiveRun(r.Context())
	if err != nil {
		handleErr(w, err)
		return
	}
	if err := h.gameSvc.ReplayModule(r.Context(), run.ID, moduleID); err != nil {
		handleErr(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]string{"status": "reset", "module_id": moduleID})
}
