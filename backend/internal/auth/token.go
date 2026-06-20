package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// claims holds the registered JWT claims used for access tokens.
type claims struct {
	jwt.RegisteredClaims
}

// HashPassword returns a bcrypt hash of the given plaintext password using the
// default cost factor.
func HashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("auth: hash password: %w", err)
	}
	return string(h), nil
}

// CheckPassword compares a bcrypt hash with a plaintext password. Returns nil
// when the password matches the hash; returns a non-nil error on mismatch.
func CheckPassword(hash, password string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return fmt.Errorf("auth: check password: %w", err)
	}
	return nil
}

// IssueAccessToken creates and signs an HS256 JWT whose subject is userID and
// whose expiry is now + ttl. Returns the compact serialised token string.
func IssueAccessToken(userID, secret string, ttl time.Duration) (string, error) {
	now := time.Now()
	c := claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	signed, err := tok.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("auth: sign access token: %w", err)
	}
	return signed, nil
}

// ParseAccessToken validates tokenStr and returns the subject (user ID).
// Returns an error for expired, malformed, or wrong-secret tokens.
func ParseAccessToken(tokenStr, secret string) (string, error) {
	tok, err := jwt.ParseWithClaims(tokenStr, &claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("auth: unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return "", fmt.Errorf("auth: parse access token: %w", err)
	}
	c, ok := tok.Claims.(*claims)
	if !ok || !tok.Valid {
		return "", fmt.Errorf("auth: invalid token claims")
	}
	return c.Subject, nil
}

// GenerateRefreshToken creates a cryptographically random 32-byte token.
// It returns both the base64url-encoded raw token (returned to the client)
// and its SHA-256 hex hash (the only value stored in the database).
func GenerateRefreshToken() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", fmt.Errorf("auth: generate refresh token: %w", err)
	}
	raw = base64.URLEncoding.EncodeToString(b)
	hash = hashRefreshToken(raw)
	return raw, hash, nil
}

// hashRefreshToken returns the SHA-256 hex digest of raw.
// This is the value stored in refresh_tokens.token_hash.
func hashRefreshToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
