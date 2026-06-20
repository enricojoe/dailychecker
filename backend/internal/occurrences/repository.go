// Package occurrences provides the occurrence domain type (one activity instance
// per date), repository interface, and sqlx-backed implementation.
package occurrences

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// Valid state values for Occurrence.State.
const (
	StatePending = "pending"
	StatePartial = "partial"
	StateDone    = "done"
)

// Sentinel errors returned by the repository.
var (
	// ErrNotFound is returned when a query matches no rows.
	ErrNotFound = errors.New("occurrences: not found")
)

// Occurrence represents a row in the occurrences table.
// Each occurrence is the single checkable instance of an activity on a
// particular Jakarta calendar date.
type Occurrence struct {
	ID                     string     `db:"id"                        json:"id"`
	ActivityID             string     `db:"activity_id"               json:"activity_id"`
	OccurDate              time.Time  `db:"occur_date"                json:"occur_date"`
	State                  string     `db:"state"                     json:"state"`
	CompletedAt            *time.Time `db:"completed_at"              json:"completed_at,omitempty"`
	PerActivityNotifiedAt  *time.Time `db:"per_activity_notified_at"  json:"per_activity_notified_at,omitempty"`
	DigestNotifiedAt       *time.Time `db:"digest_notified_at"        json:"digest_notified_at,omitempty"`
}

// Repository is the data-access contract consumed by the service layer.
type Repository interface {
	// Upsert inserts an occurrence with state='pending' for (activityID, date).
	// If an occurrence already exists for that pair, the existing row is returned
	// unchanged (idempotent). The returned *Occurrence always reflects the current
	// DB state.
	Upsert(ctx context.Context, activityID string, date time.Time) (*Occurrence, error)

	// GetByID returns the occurrence with the given UUID, or ErrNotFound.
	GetByID(ctx context.Context, id string) (*Occurrence, error)

	// ListByActivityAndDateRange returns all occurrences for the given activity
	// within the inclusive date range [from, to], ordered by occur_date.
	ListByActivityAndDateRange(ctx context.Context, activityID string, from, to time.Time) ([]*Occurrence, error)

	// ListByUserAndDate returns all occurrences belonging to a user on a
	// specific date, in the user's activity display order. Uses a single JOIN —
	// no N+1.
	ListByUserAndDate(ctx context.Context, userID string, date time.Time) ([]*Occurrence, error)

	// UpdateState sets state on the occurrence identified by id.
	// When state is 'done', completed_at is set to NOW(); otherwise it is
	// cleared to NULL. Returns the updated occurrence, or ErrNotFound.
	UpdateState(ctx context.Context, id string, state string) (*Occurrence, error)
}

type sqlxRepository struct {
	db *sqlx.DB
}

// NewRepository returns a Repository backed by the provided *sqlx.DB.
func NewRepository(db *sqlx.DB) Repository {
	return &sqlxRepository{db: db}
}

func (r *sqlxRepository) Upsert(ctx context.Context, activityID string, date time.Time) (*Occurrence, error) {
	// ON CONFLICT DO UPDATE with a no-op (set id = occurrences.id) so that
	// RETURNING always yields the current row regardless of whether it was
	// just inserted or already existed.
	const q = `
		INSERT INTO occurrences (activity_id, occur_date, state)
		VALUES ($1, $2, 'pending')
		ON CONFLICT (activity_id, occur_date) DO UPDATE
			SET id = occurrences.id
		RETURNING id, activity_id, occur_date, state,
		          completed_at, per_activity_notified_at, digest_notified_at`

	var o Occurrence
	if err := r.db.GetContext(ctx, &o, q, activityID, date); err != nil {
		return nil, fmt.Errorf("occurrences: upsert: %w", err)
	}
	return &o, nil
}

func (r *sqlxRepository) GetByID(ctx context.Context, id string) (*Occurrence, error) {
	var o Occurrence
	if err := r.db.GetContext(ctx, &o, `SELECT * FROM occurrences WHERE id = $1`, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("occurrences: get by id: %w", err)
	}
	return &o, nil
}

func (r *sqlxRepository) ListByActivityAndDateRange(
	ctx context.Context, activityID string, from, to time.Time,
) ([]*Occurrence, error) {
	const q = `
		SELECT *
		FROM   occurrences
		WHERE  activity_id = $1
		  AND  occur_date BETWEEN $2 AND $3
		ORDER  BY occur_date`

	var list []*Occurrence
	if err := r.db.SelectContext(ctx, &list, q, activityID, from, to); err != nil {
		return nil, fmt.Errorf("occurrences: list by activity and date range: %w", err)
	}
	return list, nil
}

func (r *sqlxRepository) ListByUserAndDate(ctx context.Context, userID string, date time.Time) ([]*Occurrence, error) {
	// Single JOIN — no N+1. We select only occurrences columns to avoid
	// scanning activity columns into the Occurrence struct.
	const q = `
		SELECT o.id,
		       o.activity_id,
		       o.occur_date,
		       o.state,
		       o.completed_at,
		       o.per_activity_notified_at,
		       o.digest_notified_at
		FROM   occurrences o
		JOIN   activities  a ON a.id = o.activity_id
		WHERE  a.user_id   = $1
		  AND  o.occur_date = $2
		ORDER  BY a.sort_order, a.created_at`

	var list []*Occurrence
	if err := r.db.SelectContext(ctx, &list, q, userID, date); err != nil {
		return nil, fmt.Errorf("occurrences: list by user and date: %w", err)
	}
	return list, nil
}

func (r *sqlxRepository) UpdateState(ctx context.Context, id string, state string) (*Occurrence, error) {
	// Set completed_at to NOW() when transitioning to 'done'; clear it otherwise.
	// This rule is simple enough to belong in the repo rather than the service.
	const q = `
		UPDATE occurrences SET
			state        = $2,
			completed_at = CASE WHEN $2 = 'done' THEN NOW() ELSE NULL END
		WHERE id = $1
		RETURNING id, activity_id, occur_date, state,
		          completed_at, per_activity_notified_at, digest_notified_at`

	var o Occurrence
	if err := r.db.GetContext(ctx, &o, q, id, state); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("occurrences: update state: %w", err)
	}
	return &o, nil
}
