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
	"github.com/lib/pq"
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

// CalendarDay holds aggregated occurrence counts for a single calendar date.
type CalendarDay struct {
	Date    time.Time `db:"occur_date"`
	Pending int       `db:"pending"`
	Partial int       `db:"partial"`
	Done    int       `db:"done"`
	Total   int       `db:"total"`
}

// ReminderRow is the minimal projection returned by ListDueReminders.
// It carries only the fields the scheduler needs to send a notification.
type ReminderRow struct {
	OccurrenceID string `db:"occurrence_id"`
	ChatID       int64  `db:"chat_id"`
	Title        string `db:"title"`
}

// DigestRow is the minimal projection returned by ListDigestItems.
// Rows are ordered by user so the scheduler can group them into per-user messages.
type DigestRow struct {
	OccurrenceID string `db:"occurrence_id"`
	UserID       string `db:"user_id"`
	ChatID       int64  `db:"chat_id"`
	Title        string `db:"title"`
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

	// ListGroupByParentAndDate returns all occurrences for a parent activity and
	// all of its direct children on the given date. The parent occurrence appears
	// first (parent_id IS NULL on its activity), children follow. Uses a single
	// JOIN — no N+1.  Returns ErrNotFound when the parent occurrence does not
	// exist on that date.
	ListGroupByParentAndDate(ctx context.Context, parentActivityID string, date time.Time) ([]*Occurrence, error)

	// ListCalendarSummary returns per-day aggregated counts of pending/partial/done
	// occurrences owned by userID within the inclusive date range [from, to].
	// Uses a single grouped JOIN query — no N+1.
	ListCalendarSummary(ctx context.Context, userID string, from, to time.Time) ([]*CalendarDay, error)

	// ListDueReminders returns one ReminderRow per top-level (parent_id IS NULL),
	// active occurrence that:
	//   - falls on the given date,
	//   - has time_of_day <= asOf's time-of-day portion (Jakarta),
	//   - is not in state 'done',
	//   - has per_activity_notified_at IS NULL (not yet sent),
	//   - belongs to a user with a non-NULL telegram_chat_id.
	//
	// The <= comparison means a reminder missed during downtime still fires on
	// the next tick; the dedup flag (per_activity_notified_at) guarantees it
	// fires exactly once per occurrence. Uses a single JOIN — no N+1.
	ListDueReminders(ctx context.Context, date time.Time, asOf time.Time) ([]ReminderRow, error)

	// MarkPerActivityNotified sets per_activity_notified_at = NOW() on the
	// occurrence identified by occurrenceID.
	MarkPerActivityNotified(ctx context.Context, occurrenceID string) error

	// ListDigestItems returns one DigestRow per not-done occurrence (any level)
	// on the given date where digest_notified_at IS NULL and the owning user has
	// a non-NULL telegram_chat_id. Rows are ordered by user_id so the scheduler
	// can group them into per-user messages without a second query. Uses a single
	// JOIN — no N+1.
	ListDigestItems(ctx context.Context, date time.Time) ([]DigestRow, error)

	// MarkDigestNotified sets digest_notified_at = NOW() on all occurrences
	// whose IDs are in occurrenceIDs. A single UPDATE IN (...) is used — no N+1.
	MarkDigestNotified(ctx context.Context, occurrenceIDs []string) error
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

func (r *sqlxRepository) ListGroupByParentAndDate(
	ctx context.Context, parentActivityID string, date time.Time,
) ([]*Occurrence, error) {
	// Fetch occurrences for the parent activity and all of its direct children
	// on the given date in a single JOIN. The parent row comes first (parent_id
	// IS NULL), children follow in sort_order/created_at order.
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
		WHERE  o.occur_date = $2
		  AND  (a.id = $1 OR a.parent_id = $1)
		ORDER  BY a.parent_id NULLS FIRST, a.sort_order, a.created_at`

	var list []*Occurrence
	if err := r.db.SelectContext(ctx, &list, q, parentActivityID, date); err != nil {
		return nil, fmt.Errorf("occurrences: list group by parent and date: %w", err)
	}
	if len(list) == 0 {
		return nil, ErrNotFound
	}
	return list, nil
}

func (r *sqlxRepository) ListCalendarSummary(
	ctx context.Context, userID string, from, to time.Time,
) ([]*CalendarDay, error) {
	// Single grouped JOIN: count per (date, state) pivot across the date range.
	// FILTER syntax keeps state counts without a sub-query.
	const q = `
		SELECT
		    o.occur_date,
		    COUNT(*) FILTER (WHERE o.state = 'pending') AS pending,
		    COUNT(*) FILTER (WHERE o.state = 'partial') AS partial,
		    COUNT(*) FILTER (WHERE o.state = 'done')    AS done,
		    COUNT(*) AS total
		FROM   occurrences o
		JOIN   activities  a ON a.id = o.activity_id
		WHERE  a.user_id    = $1
		  AND  o.occur_date BETWEEN $2 AND $3
		GROUP  BY o.occur_date
		ORDER  BY o.occur_date`

	var list []*CalendarDay
	if err := r.db.SelectContext(ctx, &list, q, userID, from, to); err != nil {
		return nil, fmt.Errorf("occurrences: list calendar summary: %w", err)
	}
	return list, nil
}

// ListDueReminders returns reminder rows for top-level active occurrences on
// date whose scheduled time_of_day has been reached (time_of_day <= asOf's
// time portion in Jakarta), are not done, have not yet been notified, and
// belong to a user with a telegram_chat_id.
//
// The asOf parameter is expected to be in Jakarta local time already; only its
// time-of-day component (HH:MM:SS) is used in the comparison.
func (r *sqlxRepository) ListDueReminders(ctx context.Context, date time.Time, asOf time.Time) ([]ReminderRow, error) {
	// Format the time-of-day portion as HH:MM:SS for the cast comparison.
	// time_of_day is stored as a Postgres TIME column; we compare it against
	// the time-of-day extracted from asOf.
	const q = `
		SELECT
		    o.id                  AS occurrence_id,
		    u.telegram_chat_id    AS chat_id,
		    a.title               AS title
		FROM   occurrences o
		JOIN   activities  a ON a.id    = o.activity_id
		JOIN   users       u ON u.id    = a.user_id
		WHERE  o.occur_date                = $1
		  AND  a.parent_id               IS NULL
		  AND  a.is_active                = TRUE
		  AND  a.time_of_day             <= $2::TIME
		  AND  o.state                   != 'done'
		  AND  o.per_activity_notified_at IS NULL
		  AND  u.telegram_chat_id        IS NOT NULL`

	// Pass only the date portion for occur_date, and a HH:MM:SS string for
	// the time comparison so Postgres can cast it to TIME cleanly.
	timeStr := asOf.Format("15:04:05")

	var rows []ReminderRow
	if err := r.db.SelectContext(ctx, &rows, q, date, timeStr); err != nil {
		return nil, fmt.Errorf("occurrences: list due reminders: %w", err)
	}
	return rows, nil
}

// MarkPerActivityNotified sets per_activity_notified_at = NOW() on a single
// occurrence. A missed occurrence can still be marked after a restart since
// ListDueReminders will have re-selected it.
func (r *sqlxRepository) MarkPerActivityNotified(ctx context.Context, occurrenceID string) error {
	const q = `
		UPDATE occurrences
		SET    per_activity_notified_at = NOW()
		WHERE  id = $1`

	if _, err := r.db.ExecContext(ctx, q, occurrenceID); err != nil {
		return fmt.Errorf("occurrences: mark per-activity notified: %w", err)
	}
	return nil
}

// ListDigestItems returns all not-done, not-yet-digest-notified occurrences on
// date for users with a telegram_chat_id. Both parent and child occurrences
// are included. Rows are ordered by user_id for efficient grouping by the caller.
func (r *sqlxRepository) ListDigestItems(ctx context.Context, date time.Time) ([]DigestRow, error) {
	const q = `
		SELECT
		    o.id                AS occurrence_id,
		    a.user_id           AS user_id,
		    u.telegram_chat_id  AS chat_id,
		    a.title             AS title
		FROM   occurrences o
		JOIN   activities  a ON a.id = o.activity_id
		JOIN   users       u ON u.id = a.user_id
		WHERE  o.occur_date              = $1
		  AND  o.state                  != 'done'
		  AND  o.digest_notified_at     IS NULL
		  AND  u.telegram_chat_id       IS NOT NULL
		ORDER  BY a.user_id, a.parent_id NULLS FIRST, a.sort_order, a.created_at`

	var rows []DigestRow
	if err := r.db.SelectContext(ctx, &rows, q, date); err != nil {
		return nil, fmt.Errorf("occurrences: list digest items: %w", err)
	}
	return rows, nil
}

// MarkDigestNotified sets digest_notified_at = NOW() on all occurrences in
// occurrenceIDs using a single UPDATE ... WHERE id = ANY(...) — no N+1.
func (r *sqlxRepository) MarkDigestNotified(ctx context.Context, occurrenceIDs []string) error {
	if len(occurrenceIDs) == 0 {
		return nil
	}

	// lib/pq supports array binding via pq.Array, which maps []string to
	// Postgres text[] so we can use = ANY($1) instead of IN (...).
	const q = `
		UPDATE occurrences
		SET    digest_notified_at = NOW()
		WHERE  id = ANY($1)`

	if _, err := r.db.ExecContext(ctx, q, pq.Array(occurrenceIDs)); err != nil {
		return fmt.Errorf("occurrences: mark digest notified: %w", err)
	}
	return nil
}
