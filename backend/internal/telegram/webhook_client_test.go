package telegram_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/enricojoe/dailychecker/internal/telegram"
)

// TestSetWebhook_CorrectPathAndBody verifies that SetWebhook POSTs to the
// expected path and includes the webhook URL and secret_token in the request
// body. No real network connection is made — the call targets an httptest.Server.
func TestSetWebhook_CorrectPathAndBody(t *testing.T) {
	const token = "my-bot-token"
	const webhookURL = "https://example.com/api/telegram/webhook"
	const secret = "mysecret"

	var gotPath string
	var gotBody map[string]interface{}

	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
	})

	client := telegram.NewClient(token, srv.URL, srv.Client())
	if err := client.SetWebhook(context.Background(), webhookURL, secret); err != nil {
		t.Fatalf("SetWebhook: %v", err)
	}

	wantPath := "/bot" + token + "/setWebhook"
	if gotPath != wantPath {
		t.Errorf("path: want %q, got %q", wantPath, gotPath)
	}

	if v, _ := gotBody["url"].(string); v != webhookURL {
		t.Errorf("body url: want %q, got %q", webhookURL, v)
	}
	if v, _ := gotBody["secret_token"].(string); v != secret {
		t.Errorf("body secret_token: want %q, got %q", secret, v)
	}
}

// TestSetWebhook_OmitsSecretTokenWhenEmpty verifies that an empty secret is
// not serialised as "secret_token" in the body (omitempty).
func TestSetWebhook_OmitsSecretTokenWhenEmpty(t *testing.T) {
	const token = "my-bot-token"
	const webhookURL = "https://example.com/api/telegram/webhook"

	var gotBody map[string]interface{}

	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
	})

	client := telegram.NewClient(token, srv.URL, srv.Client())
	if err := client.SetWebhook(context.Background(), webhookURL, ""); err != nil {
		t.Fatalf("SetWebhook (no secret): %v", err)
	}

	if _, ok := gotBody["secret_token"]; ok {
		t.Errorf("body should not contain secret_token when secret is empty, got %v", gotBody["secret_token"])
	}
}

// TestSetWebhook_APIError verifies that a non-OK Telegram response is returned
// as an error.
func TestSetWebhook_APIError(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":          false,
			"error_code":  401,
			"description": "Unauthorized",
		})
	})

	client := telegram.NewClient("bad-token", srv.URL, srv.Client())
	err := client.SetWebhook(context.Background(), "https://example.com/webhook", "")
	if err == nil {
		t.Fatal("SetWebhook API error: want error, got nil")
	}
	if !strings.Contains(err.Error(), "Unauthorized") {
		t.Errorf("error: want 'Unauthorized' in message, got %q", err.Error())
	}
}

// TestDeleteWebhook_CorrectPath verifies that DeleteWebhook POSTs to
// /bot<token>/deleteWebhook.
func TestDeleteWebhook_CorrectPath(t *testing.T) {
	const token = "del-bot-token"
	var gotPath string

	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
	})

	client := telegram.NewClient(token, srv.URL, srv.Client())
	if err := client.DeleteWebhook(context.Background()); err != nil {
		t.Fatalf("DeleteWebhook: %v", err)
	}

	wantPath := "/bot" + token + "/deleteWebhook"
	if gotPath != wantPath {
		t.Errorf("path: want %q, got %q", wantPath, gotPath)
	}
}
