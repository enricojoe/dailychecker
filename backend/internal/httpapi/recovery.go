package httpapi

import (
	"log"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
)

// jsonRecovery is a Gin middleware that catches panics, logs the stack trace,
// and writes a JSON 500 error envelope ({ "error": "internal server error" })
// instead of gin.Recovery()'s default plain-text response. This ensures
// panicked handlers comply with the same error-response contract as all other
// routes.
//
// The panic value and stack trace are written to the standard logger; they are
// NEVER forwarded to the client.
func jsonRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("httpapi: panic recovered: %v\n%s", r, debug.Stack())
				c.AbortWithStatusJSON(http.StatusInternalServerError, errResponse{
					Error: "internal server error",
				})
			}
		}()
		c.Next()
	}
}
