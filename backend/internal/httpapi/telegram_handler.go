package httpapi

import (
	"net/http"

	"github.com/enricojoe/dailychecker/internal/auth"
	"github.com/enricojoe/dailychecker/internal/telegram"
	"github.com/gin-gonic/gin"
)

// telegramHandler groups Telegram-related Gin handlers.
type telegramHandler struct {
	svc *telegram.Service
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
