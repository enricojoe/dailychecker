// Package httpapi contains Gin route registration, middleware wiring,
// request/response DTOs, and HTTP handlers. No business logic lives here.
package httpapi

// ── Occurrence DTOs ──────────────────────────────────────────────────────────

// PatchOccurrenceRequest is the JSON body for PATCH /api/occurrences/:id.
// state must be one of "pending", "partial", or "done".
type PatchOccurrenceRequest struct {
	State string `json:"state" binding:"required"`
}

// CalendarQueryRequest carries validated query params for
// GET /api/history/calendar.
type CalendarQueryRequest struct {
	From string `form:"from" binding:"required"`
	To   string `form:"to"   binding:"required"`
}

// ActivityHistoryQueryRequest carries validated query params for
// GET /api/history/activities/:id.
type ActivityHistoryQueryRequest struct {
	From string `form:"from" binding:"required"`
	To   string `form:"to"   binding:"required"`
}

// ── Activity DTOs ────────────────────────────────────────────────────────────

// CreateActivityRequest is the JSON body for POST /api/activities.
// days_of_week is required (and must be non-empty) when freq is "weekly";
// it must be absent or empty when freq is "daily".
type CreateActivityRequest struct {
	ParentID   *string  `json:"parent_id"`
	Title      string   `json:"title"       binding:"required"`
	Notes      *string  `json:"notes"`
	Freq       string   `json:"freq"        binding:"required"`
	DaysOfWeek []int64  `json:"days_of_week"`
	TimeOfDay  string   `json:"time_of_day" binding:"required"`
	SortOrder  int      `json:"sort_order"`
	IsActive   *bool    `json:"is_active"` // defaults to true when omitted
}

// PatchActivityRequest is the JSON body for PATCH /api/activities/:id.
// Every field is optional; omit a field to leave its value unchanged.
//
// Limitation: nullable string fields (notes, parent_id) cannot be explicitly
// cleared to SQL NULL via this struct because encoding/json maps both "absent"
// and "null" JSON values to a nil *string. Clearing nullable fields is deferred
// to a later milestone.
type PatchActivityRequest struct {
	Title      *string  `json:"title"`
	Notes      *string  `json:"notes"`
	Freq       *string  `json:"freq"`
	DaysOfWeek *[]int64 `json:"days_of_week"`
	TimeOfDay  *string  `json:"time_of_day"`
	SortOrder  *int     `json:"sort_order"`
	IsActive   *bool    `json:"is_active"`
	ParentID   *string  `json:"parent_id"`
}

// RegisterRequest is the JSON body for POST /api/auth/register.
type RegisterRequest struct {
	Name     string `json:"name"     binding:"required,min=1,max=100"`
	Phone    string `json:"phone"    binding:"required,min=5,max=20"`
	Password string `json:"password" binding:"required,min=8"`
}

// LoginRequest is the JSON body for POST /api/auth/login.
type LoginRequest struct {
	Phone    string `json:"phone"    binding:"required"`
	Password string `json:"password" binding:"required"`
}

// RefreshRequest is the JSON body for POST /api/auth/refresh.
type RefreshRequest struct {
	Refresh string `json:"refresh" binding:"required"`
}

// LogoutRequest is the JSON body for POST /api/auth/logout.
type LogoutRequest struct {
	Refresh string `json:"refresh" binding:"required"`
}

// TokenPair is the response body returned by the login and refresh endpoints.
type TokenPair struct {
	Access  string `json:"access"`
	Refresh string `json:"refresh"`
}

// ── Telegram DTOs ─────────────────────────────────────────────────────────────

// TelegramLinkResponse is the JSON response for POST /api/telegram/link.
// It carries both the deep-link URL (for display/QR code) and the raw token
// (in case the client wants to construct its own URL).
type TelegramLinkResponse struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}
