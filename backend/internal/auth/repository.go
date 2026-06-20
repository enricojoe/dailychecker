// Package auth provides JWT utilities, password hashing, refresh-token
// persistence, and the Gin auth middleware. The refresh-token repository lives
// here because the full token lifecycle (issue, rotate, revoke) is an auth
// concern, not a user-domain concern.
package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// Sentinel errors returned by the refresh-token repository.
var (
	// ErrTokenNotFound is returned when no matching token hash exists in the DB.
	ErrTokenNotFound = errors.New("auth: refresh token not found")
)

// RefreshToken represents a row in the refresh_tokens table.
// The raw token value is never stored; only its bcrypt/SHA-256 hash is persisted.
type RefreshToken struct {
	ID        string     `db:"id"         json:"id"`
	UserID    string     `db:"user_id"    json:"user_id"`
	TokenHash string     `db:"token_hash" json:"-"`
	ExpiresAt time.Time  `db:"expires_at" json:"expires_at"`
	RevokedAt *time.Time `db:"revoked_at" json:"revoked_at,omitempty"`
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
}

// IsValid reports whether the token has not expired and has not been revoked.
func (t *RefreshToken) IsValid() bool {
	return t.RevokedAt == nil && time.Now().Before(t.ExpiresAt)
}

// TokenRepository is the data-access contract for refresh tokens.
type TokenRepository interface {
	// Create inserts a new refresh token. ID and CreatedAt are set by the
	// database and written back into t on success.
	Create(ctx context.Context, t *RefreshToken) error

	// GetByHash returns the token whose hash matches, or ErrTokenNotFound.
	GetByHash(ctx context.Context, hash string) (*RefreshToken, error)

	// Revoke sets revoked_at = NOW() for the token with the given ID.
	// Returns ErrTokenNotFound if the ID does not exist.
	Revoke(ctx context.Context, id string) error
}

type sqlxTokenRepository struct {
	db *sqlx.DB
}

// NewTokenRepository returns a TokenRepository backed by the provided *sqlx.DB.
func NewTokenRepository(db *sqlx.DB) TokenRepository {
	return &sqlxTokenRepository{db: db}
}

func (r *sqlxTokenRepository) Create(ctx context.Context, t *RefreshToken) error {
	const q = `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES (:user_id, :token_hash, :expires_at)
		RETURNING id, created_at`

	stmt, args, err := sqlx.Named(q, t)
	if err != nil {
		return fmt.Errorf("auth: create token named: %w", err)
	}
	stmt = r.db.Rebind(stmt)

	row := r.db.QueryRowContext(ctx, stmt, args...)
	if err := row.Scan(&t.ID, &t.CreatedAt); err != nil {
		if isUniqueViolation(err) {
			// Extremely unlikely (hash collision), but handle gracefully.
			return fmt.Errorf("auth: create token: hash collision: %w", err)
		}
		return fmt.Errorf("auth: create token: %w", err)
	}
	return nil
}

func (r *sqlxTokenRepository) GetByHash(ctx context.Context, hash string) (*RefreshToken, error) {
	var t RefreshToken
	err := r.db.GetContext(ctx, &t,
		`SELECT * FROM refresh_tokens WHERE token_hash = $1`, hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTokenNotFound
		}
		return nil, fmt.Errorf("auth: get token by hash: %w", err)
	}
	return &t, nil
}

func (r *sqlxTokenRepository) Revoke(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE refresh_tokens SET revoked_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("auth: revoke token: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("auth: revoke token rows affected: %w", err)
	}
	if n == 0 {
		return ErrTokenNotFound
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pq.Error
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
