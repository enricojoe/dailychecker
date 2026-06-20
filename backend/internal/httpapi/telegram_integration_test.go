package httpapi_test

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestTelegramLink_Unauthed verifies that POST /api/telegram/link without a
// Bearer token returns 401.
func TestTelegramLink_Unauthed(t *testing.T) {
	w := post(t, "/api/telegram/link", nil, nil)
	assertStatus(t, w, http.StatusUnauthorized)
}

// TestTelegramLink_Authed verifies that an authenticated user receives a 200
// response containing a non-empty URL and token.
func TestTelegramLink_Authed(t *testing.T) {
	// Register + login to get an access token.
	username := uniqueUsername()
	post(t, "/api/auth/register", map[string]interface{}{
		"name": "TG Link User", "username": username, "password": "password123",
	}, nil)
	w := post(t, "/api/auth/login", map[string]interface{}{
		"username": username, "password": "password123",
	}, nil)
	assertStatus(t, w, http.StatusOK)
	access, _ := tokenPairFrom(t, w)

	// POST /api/telegram/link — must return 200 with URL + token.
	w = post(t, "/api/telegram/link", nil, bearer(access))
	assertStatus(t, w, http.StatusOK)

	var body struct {
		URL   string `json:"url"`
		Token string `json:"token"`
	}
	decodeJSON(t, w, &body)

	if body.URL == "" {
		t.Fatal("telegram/link response: url is empty")
	}
	if body.Token == "" {
		t.Fatal("telegram/link response: token is empty")
	}
	if !strings.HasPrefix(body.URL, "https://t.me/TestBot?start=") {
		t.Errorf("telegram/link url: want prefix 'https://t.me/TestBot?start=', got %q", body.URL)
	}
	if !strings.Contains(body.URL, body.Token) {
		t.Errorf("telegram/link url %q does not contain token %q", body.URL, body.Token)
	}
}

// TestTelegramLink_ReissueReplacesToken verifies that calling the endpoint
// twice returns different tokens (re-issue replaces the old one).
func TestTelegramLink_ReissueReplacesToken(t *testing.T) {
	username := uniqueUsername()
	post(t, "/api/auth/register", map[string]interface{}{
		"name": "TG Reissue", "username": username, "password": "password123",
	}, nil)
	w := post(t, "/api/auth/login", map[string]interface{}{
		"username": username, "password": "password123",
	}, nil)
	access, _ := tokenPairFrom(t, w)

	w1 := post(t, "/api/telegram/link", nil, bearer(access))
	assertStatus(t, w1, http.StatusOK)

	// Small sleep to avoid identical UnixNano-based usernames on fast hardware.
	time.Sleep(time.Millisecond)

	w2 := post(t, "/api/telegram/link", nil, bearer(access))
	assertStatus(t, w2, http.StatusOK)

	var r1, r2 struct {
		Token string `json:"token"`
	}
	decodeJSON(t, w1, &r1)
	decodeJSON(t, w2, &r2)

	if r1.Token == r2.Token {
		t.Error("re-issue: want different tokens, got same")
	}
}
