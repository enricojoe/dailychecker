// Package httpapi wires the Gin router, middleware stack, and all HTTP handlers.
package httpapi

import (
	"net/http"

	"github.com/enricojoe/dailychecker/internal/activities"
	"github.com/enricojoe/dailychecker/internal/auth"
	"github.com/enricojoe/dailychecker/internal/occurrences"
	"github.com/enricojoe/dailychecker/internal/telegram"
	"github.com/gin-gonic/gin"
)

// RouterConfig groups optional settings for the router that come from config
// but vary between normal operation and tests.  Zero values are safe: CORS
// defaults to no allowed origins (preflight replies omit the header) and the
// webhook is not registered.
type RouterConfig struct {
	// CORSAllowedOrigins is the list of origins permitted for CORS.
	CORSAllowedOrigins []string

	// TelegramWebhookMode signals that the webhook route should be registered
	// instead of (and in addition to) relying on the long-poll poller.
	// Only meaningful when tgSvc != nil.
	TelegramWebhookMode bool

	// TelegramWebhookSecret is validated in every incoming webhook request via
	// the X-Telegram-Bot-Api-Secret-Token header. Never logged.
	TelegramWebhookSecret string
}

// NewRouter constructs and returns the configured Gin engine with all routes
// and middleware registered. It accepts concrete service pointers for each
// feature domain and the JWT secret for the RequireAuth middleware.
//
// tgSvc may be nil when the Telegram bot token is absent (the server boots
// fine without it; the telegram route group is simply not registered).
func NewRouter(
	authSvc *auth.Service,
	actSvc *activities.Service,
	occSvc *occurrences.Service,
	tgSvc *telegram.Service,
	jwtSecret string,
	routerCfg RouterConfig,
) *gin.Engine {
	r := gin.New()

	// jsonRecovery replaces gin.Recovery() to ensure panics produce the same
	// {"error": "..."} JSON envelope as all other error paths. gin.Logger()
	// provides structured request logging (method, path, status, latency)
	// without leaking Authorization headers or tokens.
	r.Use(gin.Logger(), jsonRecovery())

	// CORS must run before auth middleware so preflight OPTIONS requests are
	// handled without requiring a Bearer token.
	r.Use(corsMiddleware(routerCfg.CORSAllowedOrigins))

	r.GET("/healthz", healthz)

	ah := &authHandler{svc: authSvc}
	acth := &activitiesHandler{svc: actSvc}
	occh := &occurrencesHandler{svc: occSvc}

	api := r.Group("/api")

	// Public auth routes.
	authGroup := api.Group("/auth")
	authGroup.POST("/register", ah.register)
	authGroup.POST("/login", ah.login)
	authGroup.POST("/refresh", ah.refreshToken)
	authGroup.POST("/logout", ah.logout)

	// Protected routes — all require a valid JWT Bearer access token.
	protected := api.Group("")
	protected.Use(auth.RequireAuth(jwtSecret))
	protected.GET("/me", ah.me)

	// Activities CRUD — all protected.
	actGroup := protected.Group("/activities")
	actGroup.GET("", acth.list)
	actGroup.POST("", acth.create)
	actGroup.GET("/:id", acth.getByID)
	actGroup.PATCH("/:id", acth.patch)
	actGroup.DELETE("/:id", acth.del)

	// Occurrences — all protected.
	protected.GET("/today", occh.today)
	protected.PATCH("/occurrences/:id", occh.patchOccurrence)

	// History — all protected.
	historyGroup := protected.Group("/history")
	historyGroup.GET("/calendar", occh.calendarSummary)
	historyGroup.GET("/calendar/:date", occh.calendarDay)
	historyGroup.GET("/activities/:id", occh.activityHistory)

	// Telegram — only registered when a service is wired (token present).
	if tgSvc != nil {
		tgh := &telegramHandler{
			svc:           tgSvc,
			webhookSecret: routerCfg.TelegramWebhookSecret,
		}
		// Protected link route is always present when telegram is enabled.
		protected.POST("/telegram/link", tgh.link)

		// Public webhook route — registered only in webhook mode.
		// The handler validates the shared secret on every request.
		if routerCfg.TelegramWebhookMode {
			api.POST("/telegram/webhook", tgh.webhook)
		}
	}

	return r
}

// healthz returns 200 JSON indicating the server is alive.
func healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
