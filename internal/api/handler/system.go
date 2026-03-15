package handler

import (
	"context"
	"net/http"
)

type SystemHandler struct {
	shutdown context.CancelFunc
}

func NewSystemHandler(shutdown context.CancelFunc) *SystemHandler {
	return &SystemHandler{shutdown: shutdown}
}

// GET /health
func (h *SystemHandler) Health(w http.ResponseWriter, r *http.Request) {
	respond(w, http.StatusOK, map[string]string{"status": "ok"})
}

// POST /api/v1/system/shutdown  (CLI only — not exposed to learner)
func (h *SystemHandler) Shutdown(w http.ResponseWriter, r *http.Request) {
	respond(w, http.StatusOK, map[string]string{"status": "shutting_down"})
	h.shutdown()
}
