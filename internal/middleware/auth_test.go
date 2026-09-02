package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthMiddleware(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("health is public without a configured key", func(t *testing.T) {
		t.Setenv("ARGUS_API_KEY", "")
		t.Setenv("ARGUS_AUTH_TOKEN", "")
		h := AuthMiddleware(ok)

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("fails closed when no key is configured", func(t *testing.T) {
		t.Setenv("ARGUS_API_KEY", "")
		t.Setenv("ARGUS_AUTH_TOKEN", "")
		h := AuthMiddleware(ok)

		req := httptest.NewRequest(http.MethodPost, "/api/audit-logs", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "not configured")
	})

	t.Run("accepts X-API-Key", func(t *testing.T) {
		t.Setenv("ARGUS_API_KEY", "secret-key")
		h := AuthMiddleware(ok)

		req := httptest.NewRequest(http.MethodPost, "/api/audit-logs", nil)
		req.Header.Set("X-API-Key", "secret-key")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("rejects Authorization Bearer", func(t *testing.T) {
		t.Setenv("ARGUS_API_KEY", "secret-key")
		h := AuthMiddleware(ok)

		req := httptest.NewRequest(http.MethodPost, "/api/audit-logs", nil)
		req.Header.Set("Authorization", "Bearer secret-key")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "Missing API key")
	})

	t.Run("falls back to ARGUS_AUTH_TOKEN env", func(t *testing.T) {
		t.Setenv("ARGUS_API_KEY", "")
		t.Setenv("ARGUS_AUTH_TOKEN", "legacy-key")
		h := AuthMiddleware(ok)

		req := httptest.NewRequest(http.MethodPost, "/api/audit-logs", nil)
		req.Header.Set("X-API-Key", "legacy-key")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("rejects a missing key", func(t *testing.T) {
		t.Setenv("ARGUS_API_KEY", "secret-key")
		h := AuthMiddleware(ok)

		req := httptest.NewRequest(http.MethodPost, "/api/audit-logs", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "Missing API key")
	})

	t.Run("rejects an invalid key", func(t *testing.T) {
		t.Setenv("ARGUS_API_KEY", "secret-key")
		h := AuthMiddleware(ok)

		req := httptest.NewRequest(http.MethodPost, "/api/audit-logs", nil)
		req.Header.Set("X-API-Key", "wrong")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "Invalid API key")
	})

	t.Run("ARGUS_API_KEY takes precedence over ARGUS_AUTH_TOKEN", func(t *testing.T) {
		t.Setenv("ARGUS_API_KEY", "new-key")
		t.Setenv("ARGUS_AUTH_TOKEN", "legacy-key")
		h := AuthMiddleware(ok)

		req := httptest.NewRequest(http.MethodPost, "/api/audit-logs", nil)
		req.Header.Set("X-API-Key", "legacy-key")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)

		req = httptest.NewRequest(http.MethodPost, "/api/audit-logs", nil)
		req.Header.Set("X-API-Key", "new-key")
		w = httptest.NewRecorder()
		h.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
	})
}
