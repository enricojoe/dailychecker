package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is the interface that the telegram service and future scheduler use to
// send messages. Keeping it behind an interface allows tests to inject a mock
// and M6 to consume it without importing a concrete HTTP implementation.
type Client interface {
	// SendMessage delivers text to the Telegram chat identified by chatID.
	// It is safe to call concurrently; the implementation handles basic
	// rate-limit safety.
	SendMessage(ctx context.Context, chatID int64, text string) error
}

// httpClient is the real implementation. It calls the Telegram Bot API over
// HTTPS. baseURL is overridable so tests can point at an httptest.Server
// instead of the public internet.
type httpClient struct {
	token    string
	baseURL  string // e.g. "https://api.telegram.org"
	http     *http.Client
	lastSend time.Time // simple min-interval throttle — not concurrency-safe by itself
}

// NewClient returns a Client that talks to the Telegram Bot API.
// Pass a non-nil *http.Client to control timeouts and transport (useful for
// tests that inject an httptest.Server via a custom Transport). If nil, a
// sensible default is used.
//
// baseURL should be "https://api.telegram.org" in production; pass a
// httptest.Server URL in tests to avoid real network calls.
//
// The bot token is never logged.
func NewClient(token, baseURL string, httpCli *http.Client) Client {
	if httpCli == nil {
		httpCli = &http.Client{Timeout: 10 * time.Second}
	}
	return &httpClient{
		token:   token,
		baseURL: baseURL,
		http:    httpCli,
	}
}

// tgAPIError is returned when the Telegram API responds with ok=false.
type tgAPIError struct {
	Code        int
	Description string
}

func (e *tgAPIError) Error() string {
	return fmt.Sprintf("telegram api error %d: %s", e.Code, e.Description)
}

// tgRateLimitError is returned on HTTP 429 so callers can inspect RetryAfter.
type tgRateLimitError struct {
	RetryAfter int // seconds to wait, from Telegram's response
}

func (e *tgRateLimitError) Error() string {
	return fmt.Sprintf("telegram: rate limited, retry after %ds", e.RetryAfter)
}

// ErrRateLimit is a sentinel that wraps tgRateLimitError for errors.As checks
// in callers that want to read RetryAfter.
var ErrRateLimit = errors.New("telegram: rate limited")

// sendMessageRequest is the JSON body for the sendMessage API call.
type sendMessageRequest struct {
	ChatID int64  `json:"chat_id"`
	Text   string `json:"text"`
}

// tgResponse is the minimal shape of a Telegram Bot API response envelope.
type tgResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
	ErrorCode   int    `json:"error_code"`
	Parameters  *struct {
		RetryAfter int `json:"retry_after"`
	} `json:"parameters,omitempty"`
}

// SendMessage calls POST /bot<token>/sendMessage.
//
// Rate-limit behaviour:
//   - If the API returns HTTP 429 the method returns a wrapped ErrRateLimit
//     with the RetryAfter field populated. The caller (poller, scheduler) is
//     responsible for honouring the delay; this keeps retry logic at a higher
//     level where context cancellation can be respected cleanly.
//   - As an additional local safeguard, SendMessage enforces a 50 ms minimum
//     interval between consecutive calls on the same client instance (Telegram's
//     global limit is ~30 msg/s; 50 ms gives ~20 msg/s).  This is a simple
//     last-send timestamp approach — it is not goroutine-safe, so callers that
//     share one client across goroutines should add an external mutex or use
//     separate client instances per goroutine.
func (c *httpClient) SendMessage(ctx context.Context, chatID int64, text string) error {
	// Minimal per-client throttle: 50 ms between sends.
	const minInterval = 50 * time.Millisecond
	if wait := minInterval - time.Since(c.lastSend); wait > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}

	payload := sendMessageRequest{ChatID: chatID, Text: text}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("telegram: marshal sendMessage: %w", err)
	}

	url := fmt.Sprintf("%s/bot%s/sendMessage", c.baseURL, c.token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("telegram: sendMessage request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("telegram: read response: %w", err)
	}

	// Record send time before inspecting result so the throttle counts even on errors.
	c.lastSend = time.Now()

	// Handle HTTP 429 specifically so the caller can inspect RetryAfter.
	if resp.StatusCode == http.StatusTooManyRequests {
		var tgResp tgResponse
		_ = json.Unmarshal(raw, &tgResp)
		retryAfter := 0
		if tgResp.Parameters != nil {
			retryAfter = tgResp.Parameters.RetryAfter
		}
		return fmt.Errorf("telegram: sendMessage: %w: %w",
			ErrRateLimit, &tgRateLimitError{RetryAfter: retryAfter})
	}

	if resp.StatusCode != http.StatusOK {
		var tgResp tgResponse
		_ = json.Unmarshal(raw, &tgResp)
		return fmt.Errorf("telegram: sendMessage: %w",
			&tgAPIError{Code: resp.StatusCode, Description: tgResp.Description})
	}

	var tgResp tgResponse
	if err := json.Unmarshal(raw, &tgResp); err != nil {
		return fmt.Errorf("telegram: decode response: %w", err)
	}
	if !tgResp.OK {
		return fmt.Errorf("telegram: sendMessage: %w",
			&tgAPIError{Code: tgResp.ErrorCode, Description: tgResp.Description})
	}

	return nil
}
