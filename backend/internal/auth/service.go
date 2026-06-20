package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/enricojoe/dailychecker/internal/config"
	"github.com/enricojoe/dailychecker/internal/users"
)

// Service-level sentinel errors. Handlers use errors.Is to map these to HTTP
// status codes without leaking internal details to the caller.
var (
	// ErrInvalidCredentials is returned when a username/password pair does not match.
	ErrInvalidCredentials = errors.New("auth: invalid credentials")
	// ErrTokenInvalid is returned when a refresh token is not found, expired, or revoked.
	ErrTokenInvalid = errors.New("auth: refresh token is invalid or expired")
)

// Service orchestrates the full authentication lifecycle: registration, login,
// access-token refresh (with rotation), logout, and profile retrieval.
type Service struct {
	users  users.Repository
	tokens TokenRepository
	cfg    *config.Config
}

// NewService constructs a Service with the provided repositories and config.
func NewService(userRepo users.Repository, tokenRepo TokenRepository, cfg *config.Config) *Service {
	return &Service{users: userRepo, tokens: tokenRepo, cfg: cfg}
}

// Register creates a new user account. Returns a wrapped users.ErrConflict if
// the username is already taken — callers can detect it via errors.Is.
func (s *Service) Register(ctx context.Context, name, username, password string) (*users.User, error) {
	hash, err := HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("auth service: register: %w", err)
	}
	u := &users.User{Name: name, Username: username, PasswordHash: hash}
	if err := s.users.Create(ctx, u); err != nil {
		return nil, fmt.Errorf("auth service: register: %w", err)
	}
	return u, nil
}

// Login authenticates username + password and issues a fresh token pair.
// Returns ErrInvalidCredentials on bad username or password.
func (s *Service) Login(ctx context.Context, username, password string) (access, refresh string, err error) {
	u, err := s.users.GetByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			return "", "", ErrInvalidCredentials
		}
		return "", "", fmt.Errorf("auth service: login: %w", err)
	}
	if err := CheckPassword(u.PasswordHash, password); err != nil {
		return "", "", ErrInvalidCredentials
	}
	return s.issueTokenPair(ctx, u.ID)
}

// Refresh validates rawToken, immediately revokes it (rotation), and issues a
// new token pair. Returns ErrTokenInvalid if the token is unknown, expired, or
// already revoked.
func (s *Service) Refresh(ctx context.Context, rawToken string) (access, refresh string, err error) {
	hash := hashRefreshToken(rawToken)
	tok, err := s.tokens.GetByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, ErrTokenNotFound) {
			return "", "", ErrTokenInvalid
		}
		return "", "", fmt.Errorf("auth service: refresh lookup: %w", err)
	}
	if !tok.IsValid() {
		return "", "", ErrTokenInvalid
	}
	// Revoke the presented token before issuing a new pair (rotation).
	if err := s.tokens.Revoke(ctx, tok.ID); err != nil {
		return "", "", fmt.Errorf("auth service: refresh revoke: %w", err)
	}
	return s.issueTokenPair(ctx, tok.UserID)
}

// Logout revokes the given refresh token so it can no longer be used.
// Returns ErrTokenInvalid if the token is not found or has already been revoked.
func (s *Service) Logout(ctx context.Context, rawToken string) error {
	hash := hashRefreshToken(rawToken)
	tok, err := s.tokens.GetByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, ErrTokenNotFound) {
			return ErrTokenInvalid
		}
		return fmt.Errorf("auth service: logout lookup: %w", err)
	}
	// Reject tokens that have already been revoked so that replayed logout
	// requests do not silently succeed.
	if tok.RevokedAt != nil {
		return ErrTokenInvalid
	}
	if err := s.tokens.Revoke(ctx, tok.ID); err != nil {
		return fmt.Errorf("auth service: logout revoke: %w", err)
	}
	return nil
}

// Me returns the profile of the authenticated user.
func (s *Service) Me(ctx context.Context, userID string) (*users.User, error) {
	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("auth service: me: %w", err)
	}
	return u, nil
}

// UpdateProfileInput carries the optional fields of a profile update. A nil
// pointer means "leave unchanged". When NewPassword is non-nil, CurrentPassword
// must also be supplied and match the stored hash.
type UpdateProfileInput struct {
	Name            *string
	Username        *string
	CurrentPassword *string
	NewPassword     *string
}

// UpdateProfile applies a partial update to the authenticated user's name,
// username, and/or password. Returns:
//   - users.ErrConflict when the requested username is taken by another user,
//   - ErrInvalidCredentials when changing the password but CurrentPassword is
//     missing or does not match the stored hash.
//
// A username equal to the user's current username is a no-op for that field.
func (s *Service) UpdateProfile(ctx context.Context, userID string, in UpdateProfileInput) (*users.User, error) {
	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("auth service: update profile: %w", err)
	}

	if in.Name != nil {
		u.Name = *in.Name
	}

	if in.Username != nil && *in.Username != u.Username {
		// Pre-check for a friendlier 409; the DB unique constraint is the source
		// of truth (handled by the repository on Update).
		existing, err := s.users.GetByUsername(ctx, *in.Username)
		switch {
		case err == nil && existing.ID != u.ID:
			return nil, users.ErrConflict
		case err != nil && !errors.Is(err, users.ErrNotFound):
			return nil, fmt.Errorf("auth service: update profile: username check: %w", err)
		}
		u.Username = *in.Username
	}

	if in.NewPassword != nil {
		if in.CurrentPassword == nil {
			return nil, ErrInvalidCredentials
		}
		if err := CheckPassword(u.PasswordHash, *in.CurrentPassword); err != nil {
			return nil, ErrInvalidCredentials
		}
		hash, err := HashPassword(*in.NewPassword)
		if err != nil {
			return nil, fmt.Errorf("auth service: update profile: hash: %w", err)
		}
		u.PasswordHash = hash
	}

	if err := s.users.Update(ctx, u); err != nil {
		return nil, fmt.Errorf("auth service: update profile: %w", err)
	}
	return u, nil
}

// UsernameAvailable reports whether username is free to claim. It returns true
// when no user currently holds that username.
func (s *Service) UsernameAvailable(ctx context.Context, username string) (bool, error) {
	_, err := s.users.GetByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			return true, nil
		}
		return false, fmt.Errorf("auth service: username available: %w", err)
	}
	return false, nil
}

// issueTokenPair generates a new refresh token (persisted as its hash), signs a
// new access token, and returns both to the caller.
func (s *Service) issueTokenPair(ctx context.Context, userID string) (access, refresh string, err error) {
	rawRefresh, hashStr, err := GenerateRefreshToken()
	if err != nil {
		return "", "", fmt.Errorf("auth service: generate refresh token: %w", err)
	}
	tok := &RefreshToken{
		UserID:    userID,
		TokenHash: hashStr,
		ExpiresAt: time.Now().Add(s.cfg.RefreshTokenTTL),
	}
	if err := s.tokens.Create(ctx, tok); err != nil {
		return "", "", fmt.Errorf("auth service: store refresh token: %w", err)
	}
	accessStr, err := IssueAccessToken(userID, s.cfg.JWTSecret, s.cfg.AccessTokenTTL)
	if err != nil {
		return "", "", fmt.Errorf("auth service: issue access token: %w", err)
	}
	return accessStr, rawRefresh, nil
}
