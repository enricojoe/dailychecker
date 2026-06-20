package telegram_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/enricojoe/dailychecker/internal/telegram"
)

// ── real Client against httptest.Server (no real network) ───────────────────

// newTestServer starts an httptest.Server that records the last request and
// returns a pre-configured response. The caller provides a handler; the server
// is closed automatically at test cleanup.
func newTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func TestSendMessage_CorrectPathAndBody(t *testing.T) {
	const token = "test-bot-token"
	const chatID = int64(42)
	const text = "hello from test"

	var gotPath string
	var gotBody map[string]interface{}

	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
	})

	client := telegram.NewClient(token, srv.URL, srv.Client())

	if err := client.SendMessage(context.Background(), chatID, text); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	// Path must be /bot<token>/sendMessage.
	wantPath := "/bot" + token + "/sendMessage"
	if gotPath != wantPath {
		t.Errorf("path: want %q, got %q", wantPath, gotPath)
	}

	// Body must carry chat_id and text.
	if v, ok := gotBody["chat_id"].(float64); !ok || int64(v) != chatID {
		t.Errorf("body chat_id: want %d, got %v", chatID, gotBody["chat_id"])
	}
	if v, _ := gotBody["text"].(string); v != text {
		t.Errorf("body text: want %q, got %q", text, v)
	}
}

func TestSendMessage_Handles429(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":          false,
			"description": "Too Many Requests: retry after 5",
			"parameters":  map[string]interface{}{"retry_after": 5},
		})
	})

	client := telegram.NewClient("tok", srv.URL, srv.Client())

	err := client.SendMessage(context.Background(), 1, "hi")
	if err == nil {
		t.Fatal("SendMessage 429: want error, got nil")
	}
	if !errors.Is(err, telegram.ErrRateLimit) {
		t.Errorf("SendMessage 429: want errors.Is(err, ErrRateLimit), got %v", err)
	}
	// The error message should mention the retry_after.
	if !strings.Contains(err.Error(), "5") {
		t.Errorf("error message: want retry_after=5 mentioned, got %q", err.Error())
	}
}

func TestSendMessage_HandlesNon200(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":          false,
			"description": "Internal Server Error",
		})
	})

	client := telegram.NewClient("tok", srv.URL, srv.Client())

	err := client.SendMessage(context.Background(), 1, "hi")
	if err == nil {
		t.Fatal("SendMessage 500: want error, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error: want status code 500 in message, got %q", err.Error())
	}
}
