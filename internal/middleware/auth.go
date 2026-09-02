package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"os"
)

// AuthMiddleware validates a static API key on write endpoints.
// The key is read from ARGUS_API_KEY, with ARGUS_AUTH_TOKEN as a fallback
// for older Helm/env deployments. Clients must send X-API-Key.
func AuthMiddleware(next http.Handler) http.Handler {
	apiKey := os.Getenv("ARGUS_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("ARGUS_AUTH_TOKEN")
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow public access to health, metrics, and version endpoints
		if r.URL.Path == "/health" || r.URL.Path == "/metrics" || r.URL.Path == "/version" {
			next.ServeHTTP(w, r)
			return
		}

		// Fail closed if no API key is configured in the environment
		if apiKey == "" {
			http.Error(w, "Unauthorized: Server authentication is not configured", http.StatusUnauthorized)
			return
		}

		presented := r.Header.Get("X-API-Key")
		if presented == "" {
			http.Error(w, "Unauthorized: Missing API key", http.StatusUnauthorized)
			return
		}

		// Use constant-time comparison on hashes to prevent length-based timing attacks
		expectedHash := sha256.Sum256([]byte(apiKey))
		actualHash := sha256.Sum256([]byte(presented))
		if subtle.ConstantTimeCompare(actualHash[:], expectedHash[:]) != 1 {
			http.Error(w, "Unauthorized: Invalid API key", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}
