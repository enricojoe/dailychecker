// Package httpapi wires the Gin router, middleware stack, and all HTTP handlers.
package httpapi

import (
	"net/http"

	"github.com/enricojoe/dailychecker/internal/activities"
	"github.com/enricojoe/dailychecker/internal/auth"
	"github.com/gin-gonic/gin"
)

// NewRouter constructs and returns the configured Gin engine with all routes
// and middleware registered. It accepts concrete service pointers for each
// feature domain and the JWT secret for the RequireAuth middleware.
func NewRouter(authSvc *auth.Service, actSvc *activities.Service, jwtSecret string) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	r.GET("/healthz", healthz)

	ah := &authHandler{svc: authSvc}
	acth := &activitiesHandler{svc: actSvc}

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

	return r
}

// healthz returns 200 JSON indicating the server is alive.
func healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
