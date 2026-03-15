package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/prodops-chronicles/prodops/internal/api/handler"
	"github.com/prodops-chronicles/prodops/internal/api/middleware"
)

// Handlers groups all route handlers for dependency injection at startup.
type Handlers struct {
	System   *handler.SystemHandler
	Player   *handler.PlayerHandler
	Runs     *handler.RunHandler
	Modules  *handler.ModuleHandler
	Gameplay *handler.GameplayHandler
	Progress *handler.ProgressHandler
	Config   *handler.ConfigHandler
	Sync     *handler.SyncHandler
	Internal *handler.InternalHandler
}

// NewRouter builds and returns the fully configured chi router.
func NewRouter(h *Handlers) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(middleware.Recovery)
	r.Use(middleware.RequestLogger)

	// Health (no auth)
	r.Get("/health", h.System.Health)

	// All other routes require Bearer auth
	r.Group(func(r chi.Router) {
		r.Use(middleware.BearerAuth)

		r.Route("/api/v1", func(r chi.Router) {

			// ── System ────────────────────────────────────────────────────
			r.Post("/system/shutdown", h.System.Shutdown)

			// ── Player ────────────────────────────────────────────────────
			r.Post("/player", h.Player.Create)
			r.Get("/player", h.Player.Get)
			r.Put("/player/git-identity", h.Player.SetGitIdentity)
			r.Put("/player/ssh-key", h.Player.SetSSHKey)
			r.Put("/player/telemetry", h.Player.SetTelemetry)

			// ── Runs ──────────────────────────────────────────────────────
			r.Post("/runs", h.Runs.Start)
			r.Get("/runs/active", h.Runs.GetActive)
			r.Get("/runs", h.Runs.List)
			r.Post("/runs/{run_id}/complete", h.Runs.Complete)

			// ── Modules ───────────────────────────────────────────────────
			r.Get("/modules", h.Modules.List)
			r.Get("/modules/{module_id}", h.Modules.Get)
			r.Post("/modules/{module_id}/enable", h.Modules.Enable)
			r.Post("/modules/{module_id}/disable", h.Modules.Disable)
			r.Post("/modules/{module_id}/unlock", h.Modules.ForceUnlock)

			// ── Gameplay ──────────────────────────────────────────────────
			r.Get("/modules/{module_id}/acts/current", h.Gameplay.GetCurrentAct)
			r.Get("/modules/{module_id}/acts", h.Gameplay.ListActs)
			r.Post("/modules/{module_id}/acts/{act_id}/tasks/current/verify", h.Gameplay.VerifyCurrentTask)
			r.Post("/modules/{module_id}/acts/{act_id}/decision", h.Gameplay.MakeDecision)
			r.Post("/modules/{module_id}/replay", h.Gameplay.Replay)

			// ── Progress ──────────────────────────────────────────────────
			r.Get("/progress", h.Progress.Summary)
			r.Get("/progress/modules/{module_id}", h.Progress.ModuleDetail)
			r.Get("/progress/yearly-review", h.Progress.YearlyReview)

			// ── Config ────────────────────────────────────────────────────
			r.Get("/config", h.Config.List)
			r.Get("/config/{key}", h.Config.Get)
			r.Put("/config/{key}", h.Config.Set)

			// ── Sync ──────────────────────────────────────────────────────
			r.Post("/sync", h.Sync.Sync)

			// ── Internal (pod → backend) ──────────────────────────────────
			r.Post("/internal/modules/seed", h.Internal.SeedModule)
		})
	})

	return r
}
