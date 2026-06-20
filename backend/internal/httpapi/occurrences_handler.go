package httpapi

import (
	"fmt"
	"net/http"
	"time"

	"github.com/enricojoe/dailychecker/internal/auth"
	"github.com/enricojoe/dailychecker/internal/occurrences"
	"github.com/gin-gonic/gin"
)

const (
	// maxCalendarRange is the maximum number of days allowed for calendar/history
	// range queries.  366 days covers a full leap year.
	maxCalendarRange = 366
)

// occurrencesHandler holds the occurrence service and handles all occurrence
// and history HTTP endpoints.
type occurrencesHandler struct {
	svc *occurrences.Service
}

// today handles GET /api/today.
// Idempotently generates today's occurrences then returns the occurrence tree.
func (h *occurrencesHandler) today(c *gin.Context) {
	userID := c.GetString(auth.ContextKeyUserID)

	tree, err := h.svc.Today(c.Request.Context(), userID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, tree)
}

// patchOccurrence handles PATCH /api/occurrences/:id.
// Applies a state change with parent/child rollup and returns the updated group.
func (h *occurrencesHandler) patchOccurrence(c *gin.Context) {
	userID := c.GetString(auth.ContextKeyUserID)
	id := c.Param("id")

	var req PatchOccurrenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, errResponse{Error: "state is required"})
		return
	}

	group, err := h.svc.SetState(c.Request.Context(), userID, id, req.State)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, group)
}

// calendarSummary handles GET /api/history/calendar?from=&to=.
// Returns per-day occurrence state counts for the given date range.
func (h *occurrencesHandler) calendarSummary(c *gin.Context) {
	userID := c.GetString(auth.ContextKeyUserID)

	var q CalendarQueryRequest
	if err := c.ShouldBindQuery(&q); err != nil {
		c.JSON(http.StatusUnprocessableEntity, errResponse{Error: "from and to query params are required (YYYY-MM-DD)"})
		return
	}

	from, to, err := parseDateRange(q.From, q.To)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, errResponse{Error: err.Error()})
		return
	}

	summary, err := h.svc.CalendarSummary(c.Request.Context(), userID, from, to)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, summary)
}

// calendarDay handles GET /api/history/calendar/:date.
// Returns the occurrence tree for the given Jakarta calendar date.
func (h *occurrencesHandler) calendarDay(c *gin.Context) {
	userID := c.GetString(auth.ContextKeyUserID)

	date, err := parseDate(c.Param("date"))
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, errResponse{Error: "date must be in YYYY-MM-DD format"})
		return
	}

	tree, err := h.svc.CalendarDay(c.Request.Context(), userID, date)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, tree)
}

// activityHistory handles GET /api/history/activities/:id?from=&to=.
// Returns the state timeline for a single activity.
func (h *occurrencesHandler) activityHistory(c *gin.Context) {
	userID := c.GetString(auth.ContextKeyUserID)
	activityID := c.Param("id")

	var q ActivityHistoryQueryRequest
	if err := c.ShouldBindQuery(&q); err != nil {
		c.JSON(http.StatusUnprocessableEntity, errResponse{Error: "from and to query params are required (YYYY-MM-DD)"})
		return
	}

	from, to, err := parseDateRange(q.From, q.To)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, errResponse{Error: err.Error()})
		return
	}

	history, err := h.svc.ActivityHistory(c.Request.Context(), userID, activityID, from, to)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, history)
}

// ── date parsing helpers ──────────────────────────────────────────────────────

// parseDate parses a YYYY-MM-DD string into a UTC midnight time.Time.
func parseDate(s string) (time.Time, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q: must be YYYY-MM-DD", s)
	}
	return t, nil
}

// parseDateRange parses from/to YYYY-MM-DD strings, validates from<=to, and
// rejects ranges wider than maxCalendarRange days.
func parseDateRange(fromStr, toStr string) (from, to time.Time, err error) {
	from, err = parseDate(fromStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("from: invalid date: must be YYYY-MM-DD")
	}
	to, err = parseDate(toStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("to: invalid date: must be YYYY-MM-DD")
	}
	if from.After(to) {
		return time.Time{}, time.Time{}, fmt.Errorf("from must not be after to")
	}
	if int(to.Sub(from).Hours()/24) >= maxCalendarRange {
		return time.Time{}, time.Time{}, fmt.Errorf("date range must not exceed %d days", maxCalendarRange)
	}
	return from, to, nil
}
