package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// ContextKeyUserID is the Gin context key under which the authenticated user's
// UUID is stored after a successful JWT validation.
const ContextKeyUserID = "userID"

// RequireAuth returns a Gin middleware that validates an HTTP Bearer access token.
// On success it sets ContextKeyUserID in the Gin context and calls Next().
// On failure it aborts with 401 and a JSON error envelope.
func RequireAuth(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			abortUnauthorized(c, "authorization header required")
			return
		}
		const prefix = "bearer "
		if len(header) <= len(prefix) || strings.ToLower(header[:len(prefix)]) != prefix {
			abortUnauthorized(c, "authorization header must be: Bearer <token>")
			return
		}
		tokenStr := strings.TrimSpace(header[len(prefix):])
		if tokenStr == "" {
			abortUnauthorized(c, "authorization header must be: Bearer <token>")
			return
		}
		userID, err := ParseAccessToken(tokenStr, jwtSecret)
		if err != nil {
			abortUnauthorized(c, "invalid or expired access token")
			return
		}
		c.Set(ContextKeyUserID, userID)
		c.Next()
	}
}

func abortUnauthorized(c *gin.Context, msg string) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": msg})
}
