package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// corsMiddleware returns a Gin middleware that adds the appropriate
// Access-Control-* response headers and handles preflight OPTIONS requests.
//
// allowedOrigins is the list of origins that may make cross-origin requests
// (e.g. ["http://localhost:5173"]).  A wildcard "*" is intentionally NOT used
// together with credentials, per the CORS specification.
//
// Preflight OPTIONS requests receive a 204 No Content response with the
// required headers and do not continue down the handler chain.
func corsMiddleware(allowedOrigins []string) gin.HandlerFunc {
	// Build a set for O(1) origin lookup.
	originSet := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		originSet[o] = struct{}{}
	}

	const (
		allowMethods = "GET, POST, PATCH, DELETE, OPTIONS"
		allowHeaders = "Authorization, Content-Type, X-Telegram-Bot-Api-Secret-Token"
		maxAge       = "86400" // 24 h preflight cache
	)

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		if _, ok := originSet[origin]; ok {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Vary", "Origin")
		}

		// Always set these so clients know what is allowed.
		c.Header("Access-Control-Allow-Methods", allowMethods)
		c.Header("Access-Control-Allow-Headers", allowHeaders)

		if c.Request.Method == http.MethodOptions {
			c.Header("Access-Control-Max-Age", maxAge)
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
