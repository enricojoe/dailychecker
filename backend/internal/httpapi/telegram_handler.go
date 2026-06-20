package httpapi

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/enricojoe/dailychecker/internal/auth"
	"github.com/enricojoe/dailychecker/internal/telegram"
	"github.com/gin-gonic/gin"
)

// telegramHandler groups Telegram-related Gin handlers.
type telegramHandler struct {
	svc           *telegram.Service
	webhookSecret string
}

// link handles POST /api/telegram/link.
// Issues a one-time deep-link token for the authenticated user and returns the
// Telegram bot URL the user must click to link their account.
//
// Response 200: { "url": "https://t.me/<bot>?start=<token>", "token": "<token>" }
// Response 401: missing or invalid access token (handled by RequireAuth middleware).
// Response 500: unexpected internal error.
func (h *telegramHandler) link(c *gin.Context) {
	userID := c.GetString(auth.ContextKeyUserID)

	result, err := h.svc.IssueLink(c.Request.Context(), userID)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, TelegramLinkResponse{
		URL:   result.URL,
		Token: result.Token,
	})
}

// webhook handles POST /api/telegram/webhook.
// This endpoint is public (no JWT) but is protected by a shared secret
// validated in the X-Telegram-Bot-Api-Secret-Token header.
//
// Per the Telegram Bot API contract the handler ALWAYS responds 200 quickly
// so Telegram does not retry-storm even when dispatch fails (errors are
// logged but not surfaced to Telegram).
//
// Responses:
//
//	401 — missing or wrong secret; update not dispatched.
//	200 — all other cases (including dispatch errors, which are logged).
func (h *telegramHandler) webhook(c *gin.Context) {
	// Validate shared secret. Never log the secret itself.
	got := c.GetHeader("X-Telegram-Bot-Api-Secret-Token")
	if got != h.webhookSecret {
		c.JSON(http.StatusUnauthorized, errResponse{Error: "unauthorized"})
		return
	}

	var u telegram.Update
	if err := json.NewDecoder(c.Request.Body).Decode(&u); err != nil {
		// Telegram sent a body we cannot parse. Respond 200 so it doesn't
		// retry; log for our own observability.
		log.Printf("telegram webhook: decode update: %v", err)
		c.Status(http.StatusOK)
		return
	}

	if err := h.svc.HandleUpdate(c.Request.Context(), u); err != nil {
		// Dispatch errors are logged but must not cause a non-200 response.
		log.Printf("telegram webhook: HandleUpdate(update=%d): %v", u.UpdateID, err)
	}

	c.Status(http.StatusOK)
}
