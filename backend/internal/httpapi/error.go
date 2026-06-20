package httpapi

import (
	"errors"
	"net/http"

	"github.com/enricojoe/dailychecker/internal/activities"
	"github.com/enricojoe/dailychecker/internal/auth"
	"github.com/enricojoe/dailychecker/internal/occurrences"
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
	// Auth errors.
	case errors.Is(err, users.ErrConflict):
		c.JSON(http.StatusConflict, errResponse{Error: "username already registered"})
	case errors.Is(err, auth.ErrInvalidCredentials):
		c.JSON(http.StatusUnauthorized, errResponse{Error: "invalid credentials"})
	case errors.Is(err, auth.ErrTokenInvalid):
		c.JSON(http.StatusUnauthorized, errResponse{Error: "invalid or expired refresh token"})
	case errors.Is(err, users.ErrNotFound):
		c.JSON(http.StatusNotFound, errResponse{Error: "user not found"})

	// Activities errors.
	case errors.Is(err, activities.ErrNotFound):
		c.JSON(http.StatusNotFound, errResponse{Error: "activity not found"})
	case errors.Is(err, activities.ErrInvalidParent):
		c.JSON(http.StatusUnprocessableEntity, errResponse{Error: "parent_id must refer to an existing top-level activity belonging to you"})
	case errors.Is(err, activities.ErrHasChildren):
		c.JSON(http.StatusUnprocessableEntity, errResponse{Error: "cannot assign a parent to an activity that already has children"})
	case errors.Is(err, activities.ErrInvalidSchedule):
		c.JSON(http.StatusUnprocessableEntity, errResponse{Error: "invalid schedule: weekly frequency requires at least one day_of_week; daily frequency requires none"})

	// Occurrences errors.
	case errors.Is(err, occurrences.ErrNotFound):
		c.JSON(http.StatusNotFound, errResponse{Error: "occurrence not found"})
	case errors.Is(err, occurrences.ErrInvalidState):
		c.JSON(http.StatusUnprocessableEntity, errResponse{Error: "state must be one of: pending, partial, done"})

	default:
		c.JSON(http.StatusInternalServerError, errResponse{Error: "internal server error"})
	}
}
