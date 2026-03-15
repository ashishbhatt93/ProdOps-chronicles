package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/prodops-chronicles/prodops/internal/domain"
)

// respond writes a JSON body with the given status code.
func respond(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if body != nil {
		json.NewEncoder(w).Encode(body)
	}
}

// decode deserialises the request body into v.
func decode(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// handleErr maps domain errors to HTTP status codes and writes the error envelope.
func handleErr(w http.ResponseWriter, err error) {
	code, status := mapError(err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": err.Error(),
		},
	})
}

func mapError(err error) (string, int) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return "NOT_FOUND", http.StatusNotFound
	case errors.Is(err, domain.ErrConflict):
		return "CONFLICT", http.StatusConflict
	case errors.Is(err, domain.ErrLocked):
		return "LOCKED", http.StatusForbidden
	case errors.Is(err, domain.ErrInvalidInput):
		return "INVALID_INPUT", http.StatusBadRequest
	case errors.Is(err, domain.ErrUnauthorized):
		return "UNAUTHORIZED", http.StatusUnauthorized
	default:
		return "INTERNAL_ERROR", http.StatusInternalServerError
	}
}
