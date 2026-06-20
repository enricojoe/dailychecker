package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// Poller calls getUpdates in a long-poll loop, dispatches /start <token>
// messages to svc.HandleStart, and stops cleanly when ctx is cancelled.
// It is intended for development; webhook mode is deferred to M9.
type Poller struct {
	token   string
	baseURL string
	http    *http.Client
	svc     *Service
}

// NewPoller constructs a Poller.
// baseURL is "https://api.telegram.org" in production; tests override it to
// point at an httptest.Server (same pattern as the message client).
func NewPoller(token, baseURL string, httpCli *http.Client, svc *Service) *Poller {
	if httpCli == nil {
		httpCli = &http.Client{Timeout: 35 * time.Second} // long-poll needs a longer timeout
	}
	return &Poller{token: token, baseURL: baseURL, http: httpCli, svc: svc}
}

// Run starts the long-poll loop and blocks until ctx is cancelled. Call as a
// goroutine. Errors from individual update cycles are logged but do not stop
// the loop so transient network errors self-heal on the next tick.
func (p *Poller) Run(ctx context.Context) {
	var offset int64
	log.Println("telegram poller: starting")

	for {
		select {
		case <-ctx.Done():
			log.Println("telegram poller: stopped")
			return
		default:
		}

		updates, err := p.getUpdates(ctx, offset, 30)
		if err != nil {
			if ctx.Err() != nil {
				// Context was cancelled while waiting — clean exit.
				log.Println("telegram poller: stopped")
				return
			}
			log.Printf("telegram poller: getUpdates error: %v", err)
			// Back off briefly to avoid hammering on persistent errors.
			select {
			case <-ctx.Done():
				log.Println("telegram poller: stopped")
				return
			case <-time.After(5 * time.Second):
			}
			continue
		}

		for _, u := range updates {
			if u.UpdateID >= offset {
				offset = u.UpdateID + 1
			}
			if err := p.svc.HandleUpdate(ctx, u); err != nil {
				log.Printf("telegram poller: HandleUpdate(update=%d): %v", u.UpdateID, err)
			}
		}
	}
}

// getUpdates calls POST /bot<token>/getUpdates with long-polling timeout.
func (p *Poller) getUpdates(ctx context.Context, offset int64, timeoutSec int) ([]Update, error) {
	body, err := json.Marshal(map[string]interface{}{
		"offset":          offset,
		"timeout":         timeoutSec,
		"allowed_updates": []string{"message"},
	})
	if err != nil {
		return nil, fmt.Errorf("telegram poller: marshal getUpdates: %w", err)
	}

	url := fmt.Sprintf("%s/bot%s/getUpdates", p.baseURL, p.token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("telegram poller: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("telegram poller: do request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("telegram poller: read body: %w", err)
	}

	var result updatesResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("telegram poller: unmarshal: %w", err)
	}
	if !result.OK {
		return nil, fmt.Errorf("telegram poller: api error %d: %s", result.ErrorCode, result.Description)
	}
	return result.Result, nil
}

// ParseStartToken extracts the payload token from a /start <token> message.
// Returns "" for any message that is not a /start command with a non-empty payload.
// This function is exported so tests can cover the parsing logic independently
// of networking.
func ParseStartToken(u Update) string {
	if u.Message == nil {
		return ""
	}
	text := strings.TrimSpace(u.Message.Text)
	// Telegram sends "/start TOKEN" when the user clicks a deep-link.
	// A bare "/start" (no payload) is ignored.
	if !strings.HasPrefix(text, "/start ") {
		return ""
	}
	token := strings.TrimPrefix(text, "/start ")
	token = strings.TrimSpace(token)
	return token
}

// ── Telegram API shapes ──────────────────────────────────────────────────────

// Update mirrors the Telegram Update object (only the fields we use).
type Update struct {
	UpdateID int64    `json:"update_id"`
	Message  *Message `json:"message,omitempty"`
}

// Message mirrors the Telegram Message object (minimal fields).
type Message struct {
	Text string `json:"text"`
	Chat Chat   `json:"chat"`
}

// Chat carries the chat_id.
type Chat struct {
	ID int64 `json:"id"`
}

// updatesResponse is the envelope returned by getUpdates.
type updatesResponse struct {
	OK          bool     `json:"ok"`
	Result      []Update `json:"result"`
	Description string   `json:"description"`
	ErrorCode   int      `json:"error_code"`
}
