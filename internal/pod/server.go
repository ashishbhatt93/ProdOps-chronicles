package pod

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/prodops-chronicles/prodops/internal/content"
)

// Server is the HTTP server that runs inside each module pod.
type Server struct {
	moduleContent *content.ModuleContent
	router        http.Handler
}

func NewServer(mc *content.ModuleContent) *Server {
	s := &Server{moduleContent: mc}
	s.router = s.buildRouter()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) buildRouter() http.Handler {
	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)

	r.Get("/health", s.health)
	r.Get("/content/module", s.getModule)
	r.Get("/content/acts/{act_id}", s.getAct)
	r.Post("/verify", s.verify)

	return r
}

// GET /health
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GET /content/module
func (s *Server) getModule(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.moduleContent)
}

// GET /content/acts/:act_id
func (s *Server) getAct(w http.ResponseWriter, r *http.Request) {
	actID := chi.URLParam(r, "act_id")
	for _, act := range s.moduleContent.Acts {
		if act.ID == actID {
			writeJSON(w, http.StatusOK, act)
			return
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]string{
		"error": "act not found: " + actID,
	})
}

// POST /verify
func (s *Server) verify(w http.ResponseWriter, r *http.Request) {
	var req CheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	resp := RunCheck(&req)
	if resp.ExecutionError {
		slog.Error("check execution error",
			"type", req.Check.Type,
			"detail", resp.Detail)
		writeJSON(w, http.StatusInternalServerError, resp)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}
