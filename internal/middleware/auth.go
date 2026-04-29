package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"os"
	"strings"
)

// AuthMiddleware validates the Authorization header for a Bearer token
func AuthMiddleware(next http.Handler) http.Handler {
	// For production, we require a token. Fail-closed if missing.
	authToken := os.Getenv("ARGUS_AUTH_TOKEN")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for public endpoints (health, metrics, version)
		if r.URL.Path == "/health" || r.URL.Path == "/metrics" || r.URL.Path == "/version" {
			next.ServeHTTP(w, r)
			return
		}

		// Fail closed if no token is configured in the environment
		if authToken == "" {
			http.Error(w, "Unauthorized: Server authentication is not configured", http.StatusUnauthorized)
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

		// Use constant-time comparison on hashes to prevent length-based timing attacks
		expectedHash := sha256.Sum256([]byte(authToken))
		actualHash := sha256.Sum256([]byte(parts[1]))
		if subtle.ConstantTimeCompare(actualHash[:], expectedHash[:]) != 1 {
			http.Error(w, "Unauthorized: Invalid token", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}
