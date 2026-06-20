// Package activities provides the activity domain type (schedule templates),
// repository interface, and sqlx-backed implementation.
// Sub-activities are activities with a non-nil ParentID; they share the
// parent's schedule and cannot have their own children (v1 constraint, enforced
// by the service layer in Milestone 3).
package activities

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// Sentinel errors returned by the repository.
var (
	// ErrNotFound is returned when a query matches no rows.
	ErrNotFound = errors.New("activities: not found")
)

// Activity represents a row in the activities table.
//
// TimeOfDay is the Jakarta local time string in "HH:MM:SS" format (Postgres TIME
// column). Using string avoids ambiguity in pq's TIME→time.Time scanning.
//
// DaysOfWeek is meaningful only when Freq == "weekly"; it holds the scheduled
// days (0=Sunday … 6=Saturday). For daily activities, the slice is empty.
type Activity struct {
	ID          string        `db:"id"           json:"id"`
	UserID      string        `db:"user_id"      json:"user_id"`
	ParentID    *string       `db:"parent_id"    json:"parent_id,omitempty"`
	Title       string        `db:"title"        json:"title"`
	Notes       *string       `db:"notes"        json:"notes,omitempty"`
	Freq        string        `db:"freq"         json:"freq"`
	DaysOfWeek  pq.Int64Array `db:"days_of_week" json:"days_of_week"`
	TimeOfDay   string        `db:"time_of_day"  json:"time_of_day"`
	SortOrder   int           `db:"sort_order"   json:"sort_order"`
	IsActive    bool          `db:"is_active"    json:"is_active"`
	CreatedAt   time.Time     `db:"created_at"   json:"created_at"`
	UpdatedAt   time.Time     `db:"updated_at"   json:"updated_at"`
}

// Repository is the data-access contract consumed by the service layer.
type Repository interface {
	// Create inserts a new activity. ID, CreatedAt, and UpdatedAt are set by
	// the database and written back into a on success.
	Create(ctx context.Context, a *Activity) error

	// GetByID returns the activity with the given UUID, or ErrNotFound.
	GetByID(ctx context.Context, id string) (*Activity, error)

	// ListByUser returns all activities owned by userID, ordered so that
	// top-level activities (parent_id IS NULL) precede children, then by
	// sort_order and created_at. Callers build the in-memory tree from this
	// flat list using the ParentID field.
	ListByUser(ctx context.Context, userID string) ([]*Activity, error)

	// Update persists changes to all mutable fields. UpdatedAt is refreshed
	// by the database and written back into a.
	Update(ctx context.Context, a *Activity) error

	// Delete removes the activity and, via ON DELETE CASCADE, all its
	// children and occurrences. Returns ErrNotFound if no row matched.
	Delete(ctx context.Context, id string) error
}

type sqlxRepository struct {
	db *sqlx.DB
}

// NewRepository returns a Repository backed by the provided *sqlx.DB.
func NewRepository(db *sqlx.DB) Repository {
	return &sqlxRepository{db: db}
}

func (r *sqlxRepository) Create(ctx context.Context, a *Activity) error {
	const q = `
		INSERT INTO activities
			(user_id, parent_id, title, notes, freq, days_of_week, time_of_day, sort_order, is_active)
		VALUES
			(:user_id, :parent_id, :title, :notes, :freq, :days_of_week, :time_of_day, :sort_order, :is_active)
		RETURNING id, created_at, updated_at`

	stmt, args, err := sqlx.Named(q, a)
	if err != nil {
		return fmt.Errorf("activities: create named: %w", err)
	}
	stmt = r.db.Rebind(stmt)

	row := r.db.QueryRowContext(ctx, stmt, args...)
	if err := row.Scan(&a.ID, &a.CreatedAt, &a.UpdatedAt); err != nil {
		return fmt.Errorf("activities: create: %w", err)
	}
	return nil
}

// selectCols is the explicit column list used in all SELECT queries.
// time_of_day is cast to TEXT so that lib/pq delivers it as a plain "HH:MM:SS"
// string rather than a time.Time (which pq uses for Postgres TIME columns).
const selectCols = `
	id, user_id, parent_id, title, notes, freq, days_of_week,
	time_of_day::TEXT AS time_of_day,
	sort_order, is_active, created_at, updated_at
`

func (r *sqlxRepository) GetByID(ctx context.Context, id string) (*Activity, error) {
	var a Activity
	q := `SELECT` + selectCols + `FROM activities WHERE id = $1`
	if err := r.db.GetContext(ctx, &a, q, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("activities: get by id: %w", err)
	}
	return &a, nil
}

func (r *sqlxRepository) ListByUser(ctx context.Context, userID string) ([]*Activity, error) {
	// ORDER BY: top-level activities (parent_id IS NULL) first, then children.
	// Within each group, order by sort_order then created_at for a stable list.
	// Callers reconstruct the tree in memory from the ParentID field.
	q := `SELECT` + selectCols + `
		FROM   activities
		WHERE  user_id = $1
		ORDER  BY parent_id NULLS FIRST, sort_order, created_at`

	var list []*Activity
	if err := r.db.SelectContext(ctx, &list, q, userID); err != nil {
		return nil, fmt.Errorf("activities: list by user: %w", err)
	}
	return list, nil
}

func (r *sqlxRepository) Update(ctx context.Context, a *Activity) error {
	const q = `
		UPDATE activities SET
			parent_id    = :parent_id,
			title        = :title,
			notes        = :notes,
			freq         = :freq,
			days_of_week = :days_of_week,
			time_of_day  = :time_of_day,
			sort_order   = :sort_order,
			is_active    = :is_active,
			updated_at   = NOW()
		WHERE id = :id
		RETURNING updated_at`

	stmt, args, err := sqlx.Named(q, a)
	if err != nil {
		return fmt.Errorf("activities: update named: %w", err)
	}
	stmt = r.db.Rebind(stmt)

	row := r.db.QueryRowContext(ctx, stmt, args...)
	if err := row.Scan(&a.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("activities: update: %w", err)
	}
	return nil
}

func (r *sqlxRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM activities WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("activities: delete: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("activities: delete rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
