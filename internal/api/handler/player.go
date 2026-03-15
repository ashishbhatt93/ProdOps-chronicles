package handler

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/prodops-chronicles/prodops/internal/domain"
	"github.com/prodops-chronicles/prodops/internal/service"
)

type PlayerHandler struct {
	svc *service.PlayerService
}

func NewPlayerHandler(svc *service.PlayerService) *PlayerHandler {
	return &PlayerHandler{svc: svc}
}

// POST /api/v1/player
func (h *PlayerHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DisplayName string `json:"display_name"`
	}
	if err := decode(r, &req); err != nil {
		handleErr(w, domain.ErrInvalidInput)
		return
	}
	player, err := h.svc.SetupPlayer(r.Context(), req.DisplayName)
	if err != nil {
		handleErr(w, err)
		return
	}
	respond(w, http.StatusCreated, player)
}

// GET /api/v1/player
func (h *PlayerHandler) Get(w http.ResponseWriter, r *http.Request) {
	identities, err := h.svc.DetectGitIdentities(r.Context())
	if err != nil {
		handleErr(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]interface{}{"git_identities": identities})
}

// PUT /api/v1/player/git-identity
func (h *PlayerHandler) SetGitIdentity(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		PlayerID string `json:"player_id"`
		RunID    string `json:"run_id"`
	}
	if err := decode(r, &req); err != nil {
		handleErr(w, domain.ErrInvalidInput)
		return
	}
	playerID, err := uuid.Parse(req.PlayerID)
	if err != nil {
		handleErr(w, wrapInvalidInput("invalid player_id"))
		return
	}
	runID, err := uuid.Parse(req.RunID)
	if err != nil {
		handleErr(w, wrapInvalidInput("invalid run_id"))
		return
	}
	identity := domain.GitIdentity{Username: req.Username, Email: req.Email}
	if err := h.svc.ConfirmGitIdentity(r.Context(), playerID, runID, identity); err != nil {
		handleErr(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]string{"status": "ok"})
}

// PUT /api/v1/player/ssh-key
func (h *PlayerHandler) SetSSHKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PlayerID string `json:"player_id"`
		Path     string `json:"path"`
	}
	if err := decode(r, &req); err != nil {
		handleErr(w, domain.ErrInvalidInput)
		return
	}
	playerID, err := uuid.Parse(req.PlayerID)
	if err != nil {
		handleErr(w, wrapInvalidInput("invalid player_id"))
		return
	}
	if err := h.svc.SetSSHKeyPath(r.Context(), playerID, req.Path); err != nil {
		handleErr(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]string{"status": "ok"})
}

// PUT /api/v1/player/telemetry
func (h *PlayerHandler) SetTelemetry(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PlayerID string `json:"player_id"`
		RunID    string `json:"run_id"`
		Consent  string `json:"consent"` // tier1 | tier2 | tier3
	}
	if err := decode(r, &req); err != nil {
		handleErr(w, domain.ErrInvalidInput)
		return
	}
	playerID, err := uuid.Parse(req.PlayerID)
	if err != nil {
		handleErr(w, wrapInvalidInput("invalid player_id"))
		return
	}
	runID, err := uuid.Parse(req.RunID)
	if err != nil {
		handleErr(w, wrapInvalidInput("invalid run_id"))
		return
	}
	consent := domain.TelemetryConsent(req.Consent)
	if consent != domain.TelemetryTier1 && consent != domain.TelemetryTier2 && consent != domain.TelemetryTier3 {
		handleErr(w, wrapInvalidInput("consent must be tier1, tier2, or tier3"))
		return
	}
	if err := h.svc.SetTelemetryConsent(r.Context(), playerID, runID, consent); err != nil {
		handleErr(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]string{"status": "ok"})
}
