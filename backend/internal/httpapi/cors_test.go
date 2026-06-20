package httpapi_test

import (
	"net/http"
	"testing"
)

// TestCORSPreflight verifies that an OPTIONS preflight to a protected route
// returns 204 with the required Access-Control-* headers when the Origin is
// in the allowed list. The request must NOT require a Bearer token.
func TestCORSPreflight(t *testing.T) {
	origin := "http://localhost:5173"
	w := doReq(t, http.MethodOptions, "/api/activities", nil, map[string]string{
		"Origin":                         origin,
		"Access-Control-Request-Method":  "GET",
		"Access-Control-Request-Headers": "Authorization",
	})

	assertStatus(t, w, http.StatusNoContent)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != origin {
		t.Errorf("Access-Control-Allow-Origin: want %q, got %q", origin, got)
	}
	if got := w.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Error("Access-Control-Allow-Methods: want non-empty")
	}
	if got := w.Header().Get("Access-Control-Allow-Headers"); got == "" {
		t.Error("Access-Control-Allow-Headers: want non-empty")
	}
}

// TestCORSPreflightUnknownOrigin verifies that a preflight from an origin NOT
// in the allowed list does NOT receive the Access-Control-Allow-Origin header
// (no wildcard credentials leak).
func TestCORSPreflightUnknownOrigin(t *testing.T) {
	w := doReq(t, http.MethodOptions, "/api/activities", nil, map[string]string{
		"Origin":                        "https://evil.example.com",
		"Access-Control-Request-Method": "GET",
	})
	// The request returns 204 (preflight short-circuits), but the header must
	// be absent because the origin is not allowed.
	assertStatus(t, w, http.StatusNoContent)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin: want empty for unknown origin, got %q", got)
	}
}

// TestCORSNormalRequestIncludesHeader verifies that an authenticated normal
// request from an allowed origin receives the Access-Control-Allow-Origin header.
func TestCORSNormalRequestIncludesHeader(t *testing.T) {
	origin := "http://localhost:5173"

	// Hit an unauthenticated public endpoint to avoid needing a token.
	w := doReq(t, http.MethodGet, "/healthz", nil, map[string]string{
		"Origin": origin,
	})

	assertStatus(t, w, http.StatusOK)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != origin {
		t.Errorf("Access-Control-Allow-Origin: want %q, got %q", origin, got)
	}
}
