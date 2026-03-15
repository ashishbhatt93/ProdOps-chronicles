package handler

import (
	"fmt"
	"net/http"

	"github.com/prodops-chronicles/prodops/internal/content"
	"github.com/prodops-chronicles/prodops/internal/domain"
	"github.com/prodops-chronicles/prodops/internal/repository"
	"github.com/prodops-chronicles/prodops/internal/service"
)

// InternalHandler handles pod-to-backend internal routes.
// These are not part of the public CLI API.
type InternalHandler struct {
	gameSvc      *service.GameService
	moduleRepo   repository.ModuleRepository
	contentCache map[string]*content.ModuleContent
}

func NewInternalHandler(
	gameSvc *service.GameService,
	moduleRepo repository.ModuleRepository,
	contentCache map[string]*content.ModuleContent,
) *InternalHandler {
	return &InternalHandler{
		gameSvc:      gameSvc,
		moduleRepo:   moduleRepo,
		contentCache: contentCache,
	}
}

// POST /api/v1/internal/modules/seed
// Called by a module pod at startup to register content and seed the DB catalog.
func (h *InternalHandler) SeedModule(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ModuleID string                `json:"module_id"`
		Content  *content.ModuleContent `json:"content"`
	}
	if err := decode(r, &req); err != nil || req.ModuleID == "" || req.Content == nil {
		handleErr(w, wrapInvalidInput("module_id and content are required"))
		return
	}

	// Version check: if DB already has this version, skip seeding
	existing, err := h.moduleRepo.GetModule(r.Context(), req.ModuleID)
	if err == nil && existing.Version == req.Content.Version {
		// Register in-memory but skip DB write
		h.gameSvc.RegisterContent(req.ModuleID, req.Content)
		h.contentCache[req.ModuleID] = req.Content
		respond(w, http.StatusOK, map[string]string{
			"status":  "already_seeded",
			"version": req.Content.Version,
		})
		return
	}

	// Build domain.Module from content
	mod := &domain.Module{
		ID:             req.Content.ID,
		Name:           req.Content.Name,
		Version:        req.Content.Version,
		PodName:        fmt.Sprintf("prodops-%s", req.Content.ID),
		OrderIndex:     req.Content.OrderIndex,
		Mode:           req.Content.Mode,
		Runtime:        req.Content.Runtime,
		Description:    req.Content.Description,
		ScoreThreshold: req.Content.ScoreThreshold,
	}

	if err := h.moduleRepo.SeedModules(r.Context(), []*domain.Module{mod}); err != nil {
		handleErr(w, err)
		return
	}

	// Register content in memory
	h.gameSvc.RegisterContent(req.ModuleID, req.Content)
	h.contentCache[req.ModuleID] = req.Content

	respond(w, http.StatusOK, map[string]string{
		"status":  "seeded",
		"version": req.Content.Version,
	})
}

// ── handler-level error constructors ─────────────────────────────────────────

func wrapInvalidInput(msg string) error {
	return fmt.Errorf("%w: %s", domain.ErrInvalidInput, msg)
}

func wrapNotFound(msg string) error {
	return fmt.Errorf("%w: %s", domain.ErrNotFound, msg)
}
