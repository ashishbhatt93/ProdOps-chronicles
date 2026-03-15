package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/prodops-chronicles/prodops/internal/service"
)

type ProgressHandler struct {
	svc    *service.ProgressService
	runSvc *service.RunService
	yrSvc  *service.YearlyReviewService
}

func NewProgressHandler(
	svc *service.ProgressService,
	runSvc *service.RunService,
	yrSvc *service.YearlyReviewService,
) *ProgressHandler {
	return &ProgressHandler{svc: svc, runSvc: runSvc, yrSvc: yrSvc}
}

// GET /api/v1/progress
func (h *ProgressHandler) Summary(w http.ResponseWriter, r *http.Request) {
	run, err := h.runSvc.GetActiveRun(r.Context())
	if err != nil {
		handleErr(w, err)
		return
	}
	summary, err := h.svc.GetProgressSummary(r.Context(), run.ID)
	if err != nil {
		handleErr(w, err)
		return
	}
	respond(w, http.StatusOK, summary)
}

// GET /api/v1/progress/modules/:module_id
func (h *ProgressHandler) ModuleDetail(w http.ResponseWriter, r *http.Request) {
	moduleID := chi.URLParam(r, "module_id")
	run, err := h.runSvc.GetActiveRun(r.Context())
	if err != nil {
		handleErr(w, err)
		return
	}
	detail, err := h.svc.GetModuleDetail(r.Context(), run.ID, moduleID)
	if err != nil {
		handleErr(w, err)
		return
	}
	respond(w, http.StatusOK, detail)
}

// GET /api/v1/progress/yearly-review
func (h *ProgressHandler) YearlyReview(w http.ResponseWriter, r *http.Request) {
	run, err := h.runSvc.GetActiveRun(r.Context())
	if err != nil {
		handleErr(w, err)
		return
	}
	review, err := h.yrSvc.GetYearlyReview(r.Context(), run.ID)
	if err != nil {
		handleErr(w, err)
		return
	}
	respond(w, http.StatusOK, review)
}
