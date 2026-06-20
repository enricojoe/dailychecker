package httpapi

import (
	"errors"
	"net/http"

	"github.com/enricojoe/dailychecker/internal/auth"
	"github.com/enricojoe/dailychecker/internal/users"
	"github.com/gin-gonic/gin"
)

// errResponse is the JSON envelope for all error responses.
type errResponse struct {
	Error string `json:"error"`
}

// respondError maps a service-layer error to the appropriate HTTP status code
// and writes a JSON error envelope. Unrecognised errors yield 500 without
// leaking internal details to the caller.
func respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, users.ErrConflict):
		c.JSON(http.StatusConflict, errResponse{Error: "phone already registered"})
	case errors.Is(err, auth.ErrInvalidCredentials):
		c.JSON(http.StatusUnauthorized, errResponse{Error: "invalid credentials"})
	case errors.Is(err, auth.ErrTokenInvalid):
		c.JSON(http.StatusUnauthorized, errResponse{Error: "invalid or expired refresh token"})
	case errors.Is(err, users.ErrNotFound):
		c.JSON(http.StatusNotFound, errResponse{Error: "user not found"})
	default:
		c.JSON(http.StatusInternalServerError, errResponse{Error: "internal server error"})
	}
}
