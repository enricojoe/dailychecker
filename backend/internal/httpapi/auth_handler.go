package httpapi

import (
	"net/http"

	"github.com/enricojoe/dailychecker/internal/auth"
	"github.com/gin-gonic/gin"
)

// authHandler groups the auth-related Gin handlers. It depends only on
// *auth.Service; no business logic lives here.
type authHandler struct {
	svc *auth.Service
}

// register handles POST /api/auth/register.
// On success: 201 with the created user object.
// On validation failure: 422 with an error envelope.
// On duplicate phone: 409.
func (h *authHandler) register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, errResponse{Error: err.Error()})
		return
	}
	u, err := h.svc.Register(c.Request.Context(), req.Name, req.Phone, req.Password)
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
	access, refresh, err := h.svc.Login(c.Request.Context(), req.Phone, req.Password)
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
