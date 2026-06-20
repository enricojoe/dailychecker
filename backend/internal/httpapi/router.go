// Package httpapi wires the Gin router, middleware stack, and all HTTP handlers.
package httpapi

import (
	"net/http"

	"github.com/enricojoe/dailychecker/internal/auth"
	"github.com/gin-gonic/gin"
)

// NewRouter constructs and returns the configured Gin engine with all routes
// and middleware registered. It accepts an auth.Service for the auth endpoints
// and the JWT secret for the RequireAuth middleware. Feature routes from later
// milestones are added here as they land.
func NewRouter(authSvc *auth.Service, jwtSecret string) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	r.GET("/healthz", healthz)

	ah := &authHandler{svc: authSvc}

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

	return r
}

// healthz returns 200 JSON indicating the server is alive.
func healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
