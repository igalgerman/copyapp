package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDiagnosticsCSPAllowsOnlyConfiguredIngestionOrigin(t *testing.T) {
	for _, origin := range []string{"", "https://o123.ingest.us.sentry.io"} {
		w := httptest.NewRecorder()
		securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }), origin).ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
		csp := w.Header().Get("Content-Security-Policy")
		if !strings.Contains(csp, "script-src 'self'") || strings.Contains(csp, "https:") && origin == "" {
			t.Fatalf("unexpected CSP: %s", csp)
		}
		if origin != "" && !strings.Contains(csp, "connect-src 'self' "+origin+";") {
			t.Fatalf("missing narrow ingestion allowance: %s", csp)
		}
	}
}
