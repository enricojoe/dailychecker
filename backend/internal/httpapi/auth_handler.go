package httpapi

import (
	"net/http"
	"strings"

	"github.com/enricojoe/dailychecker/internal/auth"
	"github.com/gin-gonic/gin"
)

// usernameRules mirrors the registration constraints (min=3, max=30).
const (
	usernameMinLen = 3
	usernameMaxLen = 30
	passwordMinLen = 8
)

// validateUsername returns a non-empty error message when username violates the
// length rules used at registration; an empty return means it is acceptable.
func validateUsername(username string) string {
	switch {
	case len(username) < usernameMinLen:
		return "username must be at least 3 characters"
	case len(username) > usernameMaxLen:
		return "username must be at most 30 characters"
	}
	return ""
}

// authHandler groups the auth-related Gin handlers. It depends only on
// *auth.Service; no business logic lives here.
type authHandler struct {
	svc *auth.Service
}

// register handles POST /api/auth/register.
// On success: 201 with the created user object.
// On validation failure: 422 with an error envelope.
// On duplicate username: 409.
func (h *authHandler) register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, errResponse{Error: err.Error()})
		return
	}
	u, err := h.svc.Register(c.Request.Context(), req.Name, req.Username, req.Password)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, u)
}

// login handles POST /api/auth/login.
// On success: 200 with {access, refresh}.
// On bad credentials: 401.
func (h *authHandler) login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, errResponse{Error: err.Error()})
		return
	}
	access, refresh, err := h.svc.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, TokenPair{Access: access, Refresh: refresh})
}

// refreshToken handles POST /api/auth/refresh.
// Rotates the refresh token: revokes the presented one and issues a new pair.
// On success: 200 with {access, refresh}.
// On invalid/expired/revoked refresh token: 401.
func (h *authHandler) refreshToken(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, errResponse{Error: err.Error()})
		return
	}
	access, refresh, err := h.svc.Refresh(c.Request.Context(), req.Refresh)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, TokenPair{Access: access, Refresh: refresh})
}

// logout handles POST /api/auth/logout.
// Revokes the refresh token so it can no longer be used.
// On success: 204 No Content.
// On unknown refresh token: 401.
func (h *authHandler) logout(c *gin.Context) {
	var req LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, errResponse{Error: err.Error()})
		return
	}
	if err := h.svc.Logout(c.Request.Context(), req.Refresh); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// me handles GET /api/me — requires a valid Bearer access token (see RequireAuth).
// Returns the authenticated user's profile.
func (h *authHandler) me(c *gin.Context) {
	userID := c.GetString(auth.ContextKeyUserID)
	u, err := h.svc.Me(c.Request.Context(), userID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, u)
}

// updateMe handles PATCH /api/me — partial update of the authenticated user's
// name, username, and/or password.
// On success: 200 with the updated user object.
// On taken username: 409. On wrong/missing current password: 401.
// On validation failure: 422.
func (h *authHandler) updateMe(c *gin.Context) {
	userID := c.GetString(auth.ContextKeyUserID)

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, errResponse{Error: err.Error()})
		return
	}

	// ── Per-field validation for provided fields ────────────────────────────
	if req.Name != nil {
		trimmed := strings.TrimSpace(*req.Name)
		if trimmed == "" {
			c.JSON(http.StatusUnprocessableEntity, errResponse{Error: "name must not be empty"})
			return
		}
		req.Name = &trimmed
	}
	if req.Username != nil {
		trimmed := strings.TrimSpace(*req.Username)
		if msg := validateUsername(trimmed); msg != "" {
			c.JSON(http.StatusUnprocessableEntity, errResponse{Error: msg})
			return
		}
		req.Username = &trimmed
	}
	if req.NewPassword != nil {
		if len(*req.NewPassword) < passwordMinLen {
			c.JSON(http.StatusUnprocessableEntity, errResponse{Error: "new_password must be at least 8 characters"})
			return
		}
		if req.CurrentPassword == nil || *req.CurrentPassword == "" {
			c.JSON(http.StatusUnprocessableEntity, errResponse{Error: "current_password is required to change the password"})
			return
		}
	}
	if req.Name == nil && req.Username == nil && req.NewPassword == nil {
		c.JSON(http.StatusUnprocessableEntity, errResponse{Error: "no updatable fields provided"})
		return
	}

	u, err := h.svc.UpdateProfile(c.Request.Context(), userID, auth.UpdateProfileInput{
		Name:            req.Name,
		Username:        req.Username,
		CurrentPassword: req.CurrentPassword,
		NewPassword:     req.NewPassword,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, u)
}

// checkUsername handles GET /api/auth/check-username?username=X — public.
// Returns {"available": bool}. Validates the username format first (422).
func (h *authHandler) checkUsername(c *gin.Context) {
	username := strings.TrimSpace(c.Query("username"))
	if msg := validateUsername(username); msg != "" {
		c.JSON(http.StatusUnprocessableEntity, errResponse{Error: msg})
		return
	}
	available, err := h.svc.UsernameAvailable(c.Request.Context(), username)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, CheckUsernameResponse{Available: available})
}
