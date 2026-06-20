package httpapi

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/enricojoe/dailychecker/internal/activities"
	"github.com/enricojoe/dailychecker/internal/auth"
	"github.com/gin-gonic/gin"
)

// activitiesHandler groups the Gin handlers for the activities resource.
// It depends only on *activities.Service; no business logic or SQL lives here.
type activitiesHandler struct {
	svc *activities.Service
}

// list handles GET /api/activities.
// Returns the authenticated user's activities as a JSON tree.
func (h *activitiesHandler) list(c *gin.Context) {
	userID := c.GetString(auth.ContextKeyUserID)
	tree, err := h.svc.List(c.Request.Context(), userID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, tree)
}

// create handles POST /api/activities.
// Creates a new activity owned by the authenticated user.
func (h *activitiesHandler) create(c *gin.Context) {
	userID := c.GetString(auth.ContextKeyUserID)

	var req CreateActivityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, errResponse{Error: err.Error()})
		return
	}

	// ── Format & cross-field validation ─────────────────────────────────────

	if msg := validateFreqAndDays(req.Freq, req.DaysOfWeek); msg != "" {
		c.JSON(http.StatusUnprocessableEntity, errResponse{Error: msg})
		return
	}
	if err := validateDaysValues(req.DaysOfWeek); err != nil {
		c.JSON(http.StatusUnprocessableEntity, errResponse{Error: err.Error()})
		return
	}
	tod, err := parseTimeOfDay(req.TimeOfDay)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, errResponse{Error: err.Error()})
		return
	}

	// Resolve IsActive default: omitted → true.
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	in := activities.CreateInput{
		ParentID:   req.ParentID,
		Title:      req.Title,
		Notes:      req.Notes,
		Freq:       req.Freq,
		DaysOfWeek: req.DaysOfWeek,
		TimeOfDay:  tod,
		SortOrder:  req.SortOrder,
		IsActive:   isActive,
	}

	a, err := h.svc.Create(c.Request.Context(), userID, in)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, a)
}

// getByID handles GET /api/activities/:id.
func (h *activitiesHandler) getByID(c *gin.Context) {
	userID := c.GetString(auth.ContextKeyUserID)
	id := c.Param("id")

	a, err := h.svc.GetByID(c.Request.Context(), userID, id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, a)
}

// patch handles PATCH /api/activities/:id.
// Applies a partial update; only the fields present in the request body change.
func (h *activitiesHandler) patch(c *gin.Context) {
	userID := c.GetString(auth.ContextKeyUserID)
	id := c.Param("id")

	var req PatchActivityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, errResponse{Error: err.Error()})
		return
	}

	// ── Per-field format validation for provided fields ───────────────────────
	// Cross-field freq/days_of_week validation is deferred to the service, which
	// merges the request with the current activity state before validating.

	if req.Title != nil && *req.Title == "" {
		c.JSON(http.StatusUnprocessableEntity, errResponse{Error: "title must not be empty"})
		return
	}
	if req.Freq != nil && *req.Freq != "daily" && *req.Freq != "weekly" {
		c.JSON(http.StatusUnprocessableEntity, errResponse{Error: "freq must be 'daily' or 'weekly'"})
		return
	}
	if req.DaysOfWeek != nil {
		if err := validateDaysValues(*req.DaysOfWeek); err != nil {
			c.JSON(http.StatusUnprocessableEntity, errResponse{Error: err.Error()})
			return
		}
	}
	if req.TimeOfDay != nil {
		tod, err := parseTimeOfDay(*req.TimeOfDay)
		if err != nil {
			c.JSON(http.StatusUnprocessableEntity, errResponse{Error: err.Error()})
			return
		}
		req.TimeOfDay = &tod
	}
	if req.ParentID != nil && *req.ParentID == id {
		c.JSON(http.StatusUnprocessableEntity, errResponse{Error: "parent_id cannot equal the activity's own id"})
		return
	}

	in := activities.UpdateInput{
		Title:      req.Title,
		Notes:      req.Notes,
		Freq:       req.Freq,
		DaysOfWeek: req.DaysOfWeek,
		TimeOfDay:  req.TimeOfDay,
		SortOrder:  req.SortOrder,
		IsActive:   req.IsActive,
		ParentID:   req.ParentID,
	}

	a, err := h.svc.Update(c.Request.Context(), userID, id, in)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, a)
}

// del handles DELETE /api/activities/:id.
// Returns 204 No Content on success.
func (h *activitiesHandler) del(c *gin.Context) {
	userID := c.GetString(auth.ContextKeyUserID)
	id := c.Param("id")

	if err := h.svc.Delete(c.Request.Context(), userID, id); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ── Validation helpers ────────────────────────────────────────────────────────

// validateFreqAndDays returns a non-empty error message when the freq/days_of_week
// cross-field rule is violated. An empty return means the combination is valid.
// Used only on Create where both values are present in the request.
func validateFreqAndDays(freq string, days []int64) string {
	switch freq {
	case "daily":
		if len(days) > 0 {
			return "daily frequency must have an empty days_of_week"
		}
	case "weekly":
		if len(days) == 0 {
			return "weekly frequency requires at least one value in days_of_week"
		}
	default:
		return "freq must be 'daily' or 'weekly'"
	}
	return ""
}

// validateDaysValues checks that every element of days is in [0, 6] with no
// duplicates. It does NOT check whether the slice is empty — that is a
// freq-specific cross-field check handled by validateFreqAndDays.
func validateDaysValues(days []int64) error {
	seen := make(map[int64]struct{}, len(days))
	for _, d := range days {
		if d < 0 || d > 6 {
			return fmt.Errorf("days_of_week values must be between 0 (Sunday) and 6 (Saturday), got %d", d)
		}
		if _, dup := seen[d]; dup {
			return fmt.Errorf("days_of_week contains duplicate value %d", d)
		}
		seen[d] = struct{}{}
	}
	return nil
}

// parseTimeOfDay accepts "HH:MM" or "HH:MM:SS", validates the ranges, and
// normalises to "HH:MM:SS" for consistent Postgres storage.
func parseTimeOfDay(s string) (string, error) {
	var h, m, sec int
	switch len(s) {
	case 5: // HH:MM
		if n, err := fmt.Sscanf(s, "%d:%d", &h, &m); err != nil || n != 2 {
			return "", errors.New("time_of_day must be in HH:MM or HH:MM:SS format")
		}
	case 8: // HH:MM:SS
		if n, err := fmt.Sscanf(s, "%d:%d:%d", &h, &m, &sec); err != nil || n != 3 {
			return "", errors.New("time_of_day must be in HH:MM or HH:MM:SS format")
		}
	default:
		return "", errors.New("time_of_day must be in HH:MM or HH:MM:SS format")
	}
	if h < 0 || h > 23 || m < 0 || m > 59 || sec < 0 || sec > 59 {
		return "", errors.New("time_of_day has an out-of-range hour, minute, or second")
	}
	return fmt.Sprintf("%02d:%02d:%02d", h, m, sec), nil
}
