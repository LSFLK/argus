package middleware

import (
	"net/http"
	"os"
	"strings"
)

// AuthMiddleware validates the Authorization header for a Bearer token
func AuthMiddleware(next http.Handler) http.Handler {
	// For production, this would ideally be loaded from a secret manager or OIDC provider.
	// For this refactor, we'll check for an ARGUS_AUTH_TOKEN environment variable.
	authToken := os.Getenv("ARGUS_AUTH_TOKEN")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for health and version endpoints if needed, 
		// but here we apply it to the mux it's wrapped around.
		
		// If no token is configured, we might want to warn or allow (depending on policy).
		// For hardening, we should require it if configured.
		if authToken == "" {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Unauthorized: Missing Authorization header", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			http.Error(w, "Unauthorized: Invalid Authorization header format", http.StatusUnauthorized)
			return
		}

		if parts[1] != authToken {
			http.Error(w, "Unauthorized: Invalid token", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}
