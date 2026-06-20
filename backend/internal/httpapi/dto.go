// Package httpapi contains Gin route registration, middleware wiring,
// request/response DTOs, and HTTP handlers. No business logic lives here.
package httpapi

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
