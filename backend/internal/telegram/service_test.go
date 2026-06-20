package telegram_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/enricojoe/dailychecker/internal/config"
	"github.com/enricojoe/dailychecker/internal/telegram"
	"github.com/enricojoe/dailychecker/internal/testhelper"
	"github.com/enricojoe/dailychecker/internal/users"
	"github.com/jmoiron/sqlx"
)

// ── shared test setup ────────────────────────────────────────────────────────

var testDB *sqlx.DB

func TestMain(m *testing.M) {
	db, skip := testhelper.OpenTestDB()
	if skip {
		fmt.Println("DATABASE_URL not set — skipping telegram integration tests")
		os.Exit(0)
	}
	testDB = db
	code := m.Run()
	testDB.Close()
	os.Exit(code)
}

// ── mock telegram client ─────────────────────────────────────────────────────

// mockClient records SendMessage calls so tests can assert on them.
type mockClient struct {
	calls []mockCall
}

type mockCall struct {
	ChatID int64
	Text   string
}

func (m *mockClient) SendMessage(_ context.Context, chatID int64, text string) error {
	m.calls = append(m.calls, mockCall{ChatID: chatID, Text: text})
	return nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

func newTestConfig() *config.Config {
	return &config.Config{
		TelegramBotUsername: "TestBot",
		AppPublicURL:        "http://localhost:5173",
	}
}

// createUser inserts a unique user in the test DB and returns it.
func createUser(t *testing.T, repo users.Repository) *users.User {
	t.Helper()
	ctx := context.Background()
	u := &users.User{
		Name:         "TelegramTestUser",
		Username:     fmt.Sprintf("tguser_%d", time.Now().UnixNano()),
		PasswordHash: "$2a$12$placeholder",
	}
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("createUser: %v", err)
	}
	t.Cleanup(func() {
		testDB.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, u.ID)
	})
	return u
}

// ── IssueLink tests ──────────────────────────────────────────────────────────

func TestIssueLink_StoresTokenAndReturnsURL(t *testing.T) {
	repo := users.NewRepository(testDB)
	svc := telegram.NewService(repo, newTestConfig(), &mockClient{})
	ctx := context.Background()

	u := createUser(t, repo)

	result, err := svc.IssueLink(ctx, u.ID)
	if err != nil {
		t.Fatalf("IssueLink: %v", err)
	}

	// URL must have the expected shape.
	wantPrefix := "https://t.me/TestBot?start="
	if !strings.HasPrefix(result.URL, wantPrefix) {
		t.Errorf("URL: want prefix %q, got %q", wantPrefix, result.URL)
	}
	if result.Token == "" {
		t.Error("Token: want non-empty")
	}
	// Token should appear in the URL.
	if !strings.Contains(result.URL, result.Token) {
		t.Errorf("URL %q does not contain token %q", result.URL, result.Token)
	}

	// DB must have the token stored.
	fresh, err := repo.GetByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if fresh.TelegramLinkToken == nil {
		t.Fatal("telegram_link_token: want non-nil in DB")
	}
	if *fresh.TelegramLinkToken != result.Token {
		t.Errorf("telegram_link_token: want %q, got %q", result.Token, *fresh.TelegramLinkToken)
	}
}

func TestIssueLink_ReplacesExistingToken(t *testing.T) {
	repo := users.NewRepository(testDB)
	svc := telegram.NewService(repo, newTestConfig(), &mockClient{})
	ctx := context.Background()

	u := createUser(t, repo)

	r1, err := svc.IssueLink(ctx, u.ID)
	if err != nil {
		t.Fatalf("IssueLink #1: %v", err)
	}
	r2, err := svc.IssueLink(ctx, u.ID)
	if err != nil {
		t.Fatalf("IssueLink #2: %v", err)
	}

	if r1.Token == r2.Token {
		t.Error("re-issue should generate a different token")
	}

	fresh, err := repo.GetByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if fresh.TelegramLinkToken == nil || *fresh.TelegramLinkToken != r2.Token {
		t.Errorf("stored token: want %q, got %v", r2.Token, fresh.TelegramLinkToken)
	}
}

// ── HandleStart tests ────────────────────────────────────────────────────────

func TestHandleStart_ValidToken_LinksChatAndSendsDM(t *testing.T) {
	repo := users.NewRepository(testDB)
	mc := &mockClient{}
	cfg := newTestConfig()
	svc := telegram.NewService(repo, cfg, mc)
	ctx := context.Background()

	u := createUser(t, repo)

	// Issue a link token so there's something to consume.
	result, err := svc.IssueLink(ctx, u.ID)
	if err != nil {
		t.Fatalf("IssueLink: %v", err)
	}

	chatID := int64(123456789)
	if err := svc.HandleStart(ctx, result.Token, chatID); err != nil {
		t.Fatalf("HandleStart: %v", err)
	}

	// User must be updated in the DB.
	fresh, err := repo.GetByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if fresh.TelegramChatID == nil || *fresh.TelegramChatID != chatID {
		t.Errorf("telegram_chat_id: want %d, got %v", chatID, fresh.TelegramChatID)
	}
	if fresh.TelegramLinkedAt == nil {
		t.Error("telegram_linked_at: want non-nil")
	}
	// Token must be consumed (cleared to NULL).
	if fresh.TelegramLinkToken != nil {
		t.Errorf("telegram_link_token: want nil (consumed), got %v", fresh.TelegramLinkToken)
	}

	// A confirmation DM must have been sent to the correct chatID.
	if len(mc.calls) != 1 {
		t.Fatalf("SendMessage calls: want 1, got %d", len(mc.calls))
	}
	if mc.calls[0].ChatID != chatID {
		t.Errorf("SendMessage chatID: want %d, got %d", chatID, mc.calls[0].ChatID)
	}
	if !strings.Contains(mc.calls[0].Text, cfg.AppPublicURL) {
		t.Errorf("SendMessage text: want to contain AppPublicURL %q, got %q", cfg.AppPublicURL, mc.calls[0].Text)
	}
}

func TestHandleStart_ConsumedToken_IsNoOp(t *testing.T) {
	repo := users.NewRepository(testDB)
	mc := &mockClient{}
	svc := telegram.NewService(repo, newTestConfig(), mc)
	ctx := context.Background()

	u := createUser(t, repo)

	result, err := svc.IssueLink(ctx, u.ID)
	if err != nil {
		t.Fatalf("IssueLink: %v", err)
	}

	// First call links the account.
	if err := svc.HandleStart(ctx, result.Token, 111); err != nil {
		t.Fatalf("HandleStart (first): %v", err)
	}
	// Second call with the same (now-cleared) token must be a no-op.
	if err := svc.HandleStart(ctx, result.Token, 222); err != nil {
		t.Fatalf("HandleStart (reuse): want no error, got %v", err)
	}

	// The chatID must still be from the first call.
	fresh, err := repo.GetByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if fresh.TelegramChatID == nil || *fresh.TelegramChatID != 111 {
		t.Errorf("telegram_chat_id: want 111, got %v", fresh.TelegramChatID)
	}

	// Only one SendMessage should have been fired (the first HandleStart).
	if len(mc.calls) != 1 {
		t.Errorf("SendMessage calls: want 1, got %d", len(mc.calls))
	}
}

func TestHandleStart_UnknownToken_IsNoOp(t *testing.T) {
	repo := users.NewRepository(testDB)
	mc := &mockClient{}
	svc := telegram.NewService(repo, newTestConfig(), mc)
	ctx := context.Background()

	if err := svc.HandleStart(ctx, "not-a-real-token", 999); err != nil {
		t.Fatalf("HandleStart unknown token: want no error, got %v", err)
	}
	if len(mc.calls) != 0 {
		t.Errorf("SendMessage calls: want 0, got %d", len(mc.calls))
	}
}

func TestHandleStart_EmptyToken_IsNoOp(t *testing.T) {
	repo := users.NewRepository(testDB)
	mc := &mockClient{}
	svc := telegram.NewService(repo, newTestConfig(), mc)
	ctx := context.Background()

	if err := svc.HandleStart(ctx, "", 999); err != nil {
		t.Fatalf("HandleStart empty token: want no error, got %v", err)
	}
	if len(mc.calls) != 0 {
		t.Errorf("SendMessage calls: want 0, got %d", len(mc.calls))
	}
}

// ── GetByLinkToken repo test ─────────────────────────────────────────────────

func TestGetByLinkToken(t *testing.T) {
	repo := users.NewRepository(testDB)
	ctx := context.Background()

	u := createUser(t, repo)
	token := "unique-link-token-" + fmt.Sprint(time.Now().UnixNano())
	u.TelegramLinkToken = &token
	if err := repo.Update(ctx, u); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := repo.GetByLinkToken(ctx, token)
	if err != nil {
		t.Fatalf("GetByLinkToken: %v", err)
	}
	if got.ID != u.ID {
		t.Errorf("ID: want %q, got %q", u.ID, got.ID)
	}

	_, err = repo.GetByLinkToken(ctx, "does-not-exist")
	if !errors.Is(err, users.ErrNotFound) {
		t.Errorf("GetByLinkToken unknown: want ErrNotFound, got %v", err)
	}
}
