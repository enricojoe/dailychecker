package telegram

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/enricojoe/dailychecker/internal/config"
	"github.com/enricojoe/dailychecker/internal/users"
)

// IssueLinkResult is the value returned by Service.IssueLink.
type IssueLinkResult struct {
	// URL is the Telegram deep-link the user must click to start the bot and
	// trigger account linking. Format: https://t.me/<botUsername>?start=<token>
	URL string `json:"url"`
	// Token is the one-time token embedded in URL. Returned so the caller can
	// display or store it if needed, but must never be logged.
	Token string `json:"token"`
}

// Service provides the Telegram integration business logic: deep-link issuance
// and update handling. It depends only on interfaces defined in consumer packages.
type Service struct {
	users  users.Repository
	cfg    *config.Config
	client Client
}

// NewService constructs a Service. client may be nil when the bot token is
// absent (server boots without telegram configured); in that case SendMessage
// calls will panic — the poller is never started in that scenario, so this
// should not happen in practice.
func NewService(userRepo users.Repository, cfg *config.Config, client Client) *Service {
	return &Service{users: userRepo, cfg: cfg, client: client}
}

// IssueLink generates a cryptographically random one-time link token for the
// user identified by userID, persists it (replacing any existing token), and
// returns the Telegram deep-link URL plus the raw token.
//
// Re-calling for the same user simply replaces the old token.
func (s *Service) IssueLink(ctx context.Context, userID string) (*IssueLinkResult, error) {
	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("telegram: issue link: %w", err)
	}

	token, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("telegram: issue link: generate token: %w", err)
	}

	u.TelegramLinkToken = &token
	if err := s.users.Update(ctx, u); err != nil {
		return nil, fmt.Errorf("telegram: issue link: save token: %w", err)
	}

	url := fmt.Sprintf("https://t.me/%s?start=%s", s.cfg.TelegramBotUsername, token)
	return &IssueLinkResult{URL: url, Token: token}, nil
}

// HandleStart processes a /start <token> message received from Telegram.
// If token is empty or matches no user, it is a silent no-op (unknown tokens
// from unrelated /start commands must not crash the poll loop).
// On success it sets TelegramChatID, TelegramLinkedAt, clears TelegramLinkToken
// (single-use), saves the user, and sends a confirmation DM.
func (s *Service) HandleStart(ctx context.Context, token string, chatID int64) error {
	if token == "" {
		return nil
	}

	u, err := s.users.GetByLinkToken(ctx, token)
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			// Unknown or already-consumed token — ignore silently.
			return nil
		}
		return fmt.Errorf("telegram: handle start: lookup: %w", err)
	}

	now := time.Now().UTC()
	u.TelegramChatID = &chatID
	u.TelegramLinkedAt = &now
	u.TelegramLinkToken = nil // consume the token (single-use)

	if err := s.users.Update(ctx, u); err != nil {
		return fmt.Errorf("telegram: handle start: update user: %w", err)
	}

	// Best-effort confirmation DM; don't fail the link if the send fails.
	msg := fmt.Sprintf(
		"Your DailyChecker account is now linked to this Telegram chat.\n\nOpen the app: %s",
		s.cfg.AppPublicURL,
	)
	if s.client != nil {
		if sendErr := s.client.SendMessage(ctx, chatID, msg); sendErr != nil {
			// Log-worthy but not fatal — the account is already linked.
			// Returning the error here would leave the account linked but the
			// caller thinking it failed. We prefer returning success.
			_ = sendErr
		}
	}

	return nil
}

// generateToken returns a 32-byte cryptographically random hex string (64 chars).
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(b), nil
}
