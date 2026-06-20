// Package users provides the user domain type, repository interface, and
// sqlx-backed repository implementation. Business logic belongs in a separate
// service type (added in Milestone 2); this file is pure data access.
package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// Sentinel errors returned by the repository. Callers compare with errors.Is.
var (
	// ErrNotFound is returned when a query matches no rows.
	ErrNotFound = errors.New("users: not found")
	// ErrConflict is returned when a unique constraint is violated (duplicate username).
	ErrConflict = errors.New("users: username already registered")
)

// User represents a row in the users table.
type User struct {
	ID                string     `db:"id"                   json:"id"`
	Name              string     `db:"name"                 json:"name"`
	Username          string     `db:"username"             json:"username"`
	PasswordHash      string     `db:"password_hash"        json:"-"`
	TelegramChatID    *int64     `db:"telegram_chat_id"     json:"telegram_chat_id,omitempty"`
	TelegramLinkToken *string    `db:"telegram_link_token"  json:"-"`
	TelegramLinkedAt  *time.Time `db:"telegram_linked_at"   json:"telegram_linked_at,omitempty"`
	CreatedAt         time.Time  `db:"created_at"           json:"created_at"`
	UpdatedAt         time.Time  `db:"updated_at"           json:"updated_at"`
}

// Repository is the data-access contract consumed by the service layer.
// Defined here (in the consumer package) so dependencies point inward.
type Repository interface {
	// Create inserts a new user. The Name, Username, and PasswordHash fields
	// must be populated; ID, CreatedAt, and UpdatedAt are set by the database
	// and written back into u on success.
	Create(ctx context.Context, u *User) error

	// GetByID returns the user with the given UUID, or ErrNotFound.
	GetByID(ctx context.Context, id string) (*User, error)

	// GetByUsername returns the user with the given username, or ErrNotFound.
	GetByUsername(ctx context.Context, username string) (*User, error)

	// GetByLinkToken returns the user whose telegram_link_token matches token,
	// or ErrNotFound when no row matches. Used during the Telegram deep-link
	// linking flow to consume a one-time token.
	GetByLinkToken(ctx context.Context, token string) (*User, error)

	// Update persists changes to Name, Username, PasswordHash, and the Telegram
	// fields (chat ID, link token, linked-at). UpdatedAt is refreshed by the
	// database and written back into u. Returns ErrConflict when the username
	// collides with another user.
	Update(ctx context.Context, u *User) error
}

type sqlxRepository struct {
	db *sqlx.DB
}

// NewRepository returns a Repository backed by the provided *sqlx.DB.
func NewRepository(db *sqlx.DB) Repository {
	return &sqlxRepository{db: db}
}

func (r *sqlxRepository) Create(ctx context.Context, u *User) error {
	const q = `
		INSERT INTO users (name, username, password_hash)
		VALUES (:name, :username, :password_hash)
		RETURNING id, created_at, updated_at`

	stmt, args, err := sqlx.Named(q, u)
	if err != nil {
		return fmt.Errorf("users: create named: %w", err)
	}
	stmt = r.db.Rebind(stmt)

	row := r.db.QueryRowContext(ctx, stmt, args...)
	if err := row.Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return fmt.Errorf("users: create: %w", err)
	}
	return nil
}

func (r *sqlxRepository) GetByID(ctx context.Context, id string) (*User, error) {
	const q = `
		SELECT id, name, username, password_hash,
		       telegram_chat_id, telegram_link_token, telegram_linked_at,
		       created_at, updated_at
		FROM users
		WHERE id = $1`
	var u User
	if err := r.db.GetContext(ctx, &u, q, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("users: get by id: %w", err)
	}
	return &u, nil
}

func (r *sqlxRepository) GetByUsername(ctx context.Context, username string) (*User, error) {
	const q = `
		SELECT id, name, username, password_hash,
		       telegram_chat_id, telegram_link_token, telegram_linked_at,
		       created_at, updated_at
		FROM users
		WHERE username = $1`
	var u User
	if err := r.db.GetContext(ctx, &u, q, username); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("users: get by username: %w", err)
	}
	return &u, nil
}

func (r *sqlxRepository) GetByLinkToken(ctx context.Context, token string) (*User, error) {
	const q = `
		SELECT id, name, username, password_hash,
		       telegram_chat_id, telegram_link_token, telegram_linked_at,
		       created_at, updated_at
		FROM users
		WHERE telegram_link_token = $1`

	var u User
	if err := r.db.GetContext(ctx, &u, q, token); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("users: get by link token: %w", err)
	}
	return &u, nil
}

func (r *sqlxRepository) Update(ctx context.Context, u *User) error {
	const q = `
		UPDATE users SET
			name                = :name,
			username            = :username,
			password_hash       = :password_hash,
			telegram_chat_id    = :telegram_chat_id,
			telegram_link_token = :telegram_link_token,
			telegram_linked_at  = :telegram_linked_at,
			updated_at          = NOW()
		WHERE id = :id
		RETURNING updated_at`

	stmt, args, err := sqlx.Named(q, u)
	if err != nil {
		return fmt.Errorf("users: update named: %w", err)
	}
	stmt = r.db.Rebind(stmt)

	row := r.db.QueryRowContext(ctx, stmt, args...)
	if err := row.Scan(&u.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return fmt.Errorf("users: update: %w", err)
	}
	return nil
}

// isUniqueViolation reports whether err is a PostgreSQL unique-constraint
// violation (SQLSTATE 23505).
func isUniqueViolation(err error) bool {
	var pgErr *pq.Error
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
