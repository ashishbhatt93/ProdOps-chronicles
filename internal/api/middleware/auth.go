package middleware

import (
	"net/http"
	"os"
	"strings"
)

// BearerAuth validates the Authorization: Bearer <token> header.
// The token is read from the BACKEND_TOKEN environment variable at startup.
func BearerAuth(next http.Handler) http.Handler {
	token := os.Getenv("BACKEND_TOKEN")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Bearer token required")
			return
		}
		if strings.TrimPrefix(auth, "Bearer ") != token {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid token")
			return
		}
		next.ServeHTTP(w, r)
	})
}
