package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/prodops-chronicles/prodops/internal/service"
)

type RunHandler struct {
	svc *service.RunService
}

func NewRunHandler(svc *service.RunService) *RunHandler {
	return &RunHandler{svc: svc}
}

// POST /api/v1/runs
func (h *RunHandler) Start(w http.ResponseWriter, r *http.Request) {
	run, err := h.svc.StartRun(r.Context())
	if err != nil {
		handleErr(w, err)
		return
	}
	respond(w, http.StatusCreated, run)
}

// GET /api/v1/runs/active
func (h *RunHandler) GetActive(w http.ResponseWriter, r *http.Request) {
	run, err := h.svc.GetActiveRun(r.Context())
	if err != nil {
		handleErr(w, err)
		return
	}
	respond(w, http.StatusOK, run)
}

// GET /api/v1/runs
func (h *RunHandler) List(w http.ResponseWriter, r *http.Request) {
	runs, err := h.svc.ListRuns(r.Context())
	if err != nil {
		handleErr(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]any{"runs": runs})
}

// POST /api/v1/runs/:run_id/complete
func (h *RunHandler) Complete(w http.ResponseWriter, r *http.Request) {
	runID, err := uuid.Parse(chi.URLParam(r, "run_id"))
	if err != nil {
		handleErr(w, errBadParam("run_id"))
		return
	}
	if err := h.svc.CompleteRun(r.Context(), runID); err != nil {
		handleErr(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]string{"status": "completed"})
}

func errBadParam(name string) error {
	// wrap as invalid input
	return wrapInvalidInput("invalid " + name)
}
