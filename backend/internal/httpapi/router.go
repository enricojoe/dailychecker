// Package httpapi wires the Gin router, middleware stack, and all HTTP handlers.
package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// NewRouter constructs and returns the configured Gin engine with all routes
// and middleware registered. Feature routes are added as the milestones land.
func NewRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	r.GET("/healthz", healthz)

	return r
}

// healthz returns a 200 JSON response indicating the server is alive.
func healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
