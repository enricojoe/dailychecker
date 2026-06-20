package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/enricojoe/dailychecker/internal/activities"
	"github.com/enricojoe/dailychecker/internal/auth"
	"github.com/enricojoe/dailychecker/internal/config"
	"github.com/enricojoe/dailychecker/internal/httpapi"
	"github.com/enricojoe/dailychecker/internal/occurrences"
	"github.com/enricojoe/dailychecker/internal/telegram"
	"github.com/enricojoe/dailychecker/internal/users"
	"github.com/gin-gonic/gin"
)

// ── webhook router helper ────────────────────────────────────────────────────

const testWebhookSecret = "super-secret-token-for-tests"

// newWebhookRouter builds a fresh router in webhook mode backed by the shared
// testDB. It is intentionally separate from testRouter so tests can vary the
// RouterConfig without polluting the shared suite.
func newWebhookRouter(t *testing.T) http.Handler {
	t.Helper()
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		JWTSecret:       testJWTSecret,
		AccessTokenTTL:  time.Minute,
		RefreshTokenTTL: 24 * time.Hour,
		Timezone:        "Asia/Jakarta",
	}

	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		t.Fatalf("newWebhookRouter: load timezone: %v", err)
	}

	userRepo := users.NewRepository(testDB)
	tokenRepo := auth.NewTokenRepository(testDB)
	authSvc := auth.NewService(userRepo, tokenRepo, cfg)

	actRepo := activities.NewRepository(testDB)
	actSvc := activities.NewService(actRepo)

	occRepo := occurrences.NewRepository(testDB)
	occSvc := occurrences.NewService(occRepo, actRepo, loc)

	tgCfg := &config.Config{
		TelegramBotUsername: "TestBot",
		AppPublicURL:        "http://localhost:5173",
	}
	tgSvc := telegram.NewService(userRepo, tgCfg, httpTestMockTgClient{})

	return httpapi.NewRouter(authSvc, actSvc, occSvc, tgSvc, cfg.JWTSecret, httpapi.RouterConfig{
		CORSAllowedOrigins:    []string{"http://localhost:5173"},
		TelegramWebhookMode:   true,
		TelegramWebhookSecret: testWebhookSecret,
	})
}

// webhookReq fires a POST to /api/telegram/webhook against a fresh webhook
// router and returns the recorder.
func webhookReq(t *testing.T, secret string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	router := newWebhookRouter(t)

	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("webhookReq: marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/telegram/webhook", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if secret != "" {
		req.Header.Set("X-Telegram-Bot-Api-Secret-Token", secret)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// makeUpdate constructs a telegram.Update carrying a /start message.
func makeUpdate(updateID int64, text string) telegram.Update {
	return telegram.Update{
		UpdateID: updateID,
		Message: &telegram.Message{
			Text: text,
			Chat: telegram.Chat{ID: 123456},
		},
	}
}

// ── tests ────────────────────────────────────────────────────────────────────

// TestWebhook_MissingSecret verifies that a request without the secret header
// is rejected with 401 and no update is dispatched.
func TestWebhook_MissingSecret(t *testing.T) {
	w := webhookReq(t, "", makeUpdate(1, "/start sometoken"))
	assertStatus(t, w, http.StatusUnauthorized)
}

// TestWebhook_WrongSecret verifies that an incorrect secret header value is
// rejected with 401.
func TestWebhook_WrongSecret(t *testing.T) {
	w := webhookReq(t, "wrong-secret", makeUpdate(1, "/start sometoken"))
	assertStatus(t, w, http.StatusUnauthorized)
}

// TestWebhook_NonStartUpdate verifies that a non-/start message with the
// correct secret returns 200 and is silently ignored (no DB writes needed —
// the service's no-op path covers it).
func TestWebhook_NonStartUpdate(t *testing.T) {
	u := telegram.Update{
		UpdateID: 2,
		Message: &telegram.Message{
			Text: "hello world",
			Chat: telegram.Chat{ID: 789},
		},
	}
	w := webhookReq(t, testWebhookSecret, u)
	assertStatus(t, w, http.StatusOK)
}

// TestWebhook_ValidStartToken verifies the happy path end-to-end:
// a /start <token> update with the correct secret dispatches through
// HandleUpdate → HandleStart which links the user's Telegram chat.
func TestWebhook_ValidStartToken(t *testing.T) {
	// Set up a real user and a real link token so HandleStart can consume it.
	repo := users.NewRepository(testDB)
	ctx := context.Background()

	phone := fmt.Sprintf("+1777%09d", time.Now().UnixNano()%1_000_000_000)
	u := &users.User{
		Name:         "WebhookTestUser",
		Phone:        phone,
		PasswordHash: "$2a$12$placeholder",
	}
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() {
		testDB.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, u.ID)
	})

	// Issue a link token (via the service so it persists to DB).
	tgCfg := &config.Config{
		TelegramBotUsername: "TestBot",
		AppPublicURL:        "http://localhost:5173",
	}
	tgSvc := telegram.NewService(repo, tgCfg, httpTestMockTgClient{})
	result, err := tgSvc.IssueLink(ctx, u.ID)
	if err != nil {
		t.Fatalf("IssueLink: %v", err)
	}

	update := makeUpdate(10, "/start "+result.Token)
	w := webhookReq(t, testWebhookSecret, update)
	assertStatus(t, w, http.StatusOK)

	// The user should now be linked.
	fresh, err := repo.GetByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetByID after webhook: %v", err)
	}
	if fresh.TelegramChatID == nil {
		t.Fatal("telegram_chat_id: want non-nil after webhook dispatch")
	}
	if *fresh.TelegramChatID != 123456 {
		t.Errorf("telegram_chat_id: want 123456, got %d", *fresh.TelegramChatID)
	}
	if fresh.TelegramLinkToken != nil {
		t.Error("telegram_link_token: want nil (consumed), got non-nil")
	}
}

// TestWebhook_NotRegisteredInPollingMode verifies that when the router is in
// polling mode (the default testRouter), the /api/telegram/webhook route does
// not exist and returns 404.
func TestWebhook_NotRegisteredInPollingMode(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/telegram/webhook", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", testWebhookSecret)
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	assertStatus(t, w, http.StatusNotFound)
}
