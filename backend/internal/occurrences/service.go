package occurrences

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/enricojoe/dailychecker/internal/activities"
	"github.com/enricojoe/dailychecker/internal/rollup"
)

// Service-level sentinel errors.
var (
	// ErrInvalidState is returned when an unsupported state value is provided.
	ErrInvalidState = errors.New("occurrences: invalid state")
)

// validStates is the accepted set of occurrence state values.
var validStates = map[string]struct{}{
	StatePending: {},
	StatePartial: {},
	StateDone:    {},
}

// OccurrenceNode is the tree representation of an occurrence returned by Today
// and CalendarDay.  The parent occurrence is the root; child occurrences appear
// in Children. Children is always non-nil so the JSON encodes "children":[]
// rather than "children":null.
type OccurrenceNode struct {
	ID          string          `json:"id"`
	ActivityID  string          `json:"activity_id"`
	Title       string          `json:"title"`
	State       string          `json:"state"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
	Children    []*OccurrenceNode `json:"children"`
}

// DaySummary aggregates per-day occurrence counts for the calendar history view.
type DaySummary struct {
	Date    string `json:"date"`
	Pending int    `json:"pending"`
	Partial int    `json:"partial"`
	Done    int    `json:"done"`
	Total   int    `json:"total"`
}

// ActivityHistoryEntry is a single {date, state} tuple for the per-activity
// history timeline.
type ActivityHistoryEntry struct {
	Date  string `json:"date"`
	State string `json:"state"`
}

// Service orchestrates occurrence generation, state transitions with rollup,
// and history queries.
type Service struct {
	repo     Repository
	actRepo  activities.Repository
	location *time.Location
}

// NewService constructs a Service.
// loc is the Jakarta *time.Location loaded once at startup.
func NewService(repo Repository, actRepo activities.Repository, loc *time.Location) *Service {
	return &Service{
		repo:    repo,
		actRepo: actRepo,
		location: loc,
	}
}

// Today generates occurrences for today's Jakarta date (idempotent) and returns
// the occurrence tree for the authenticated user.
func (s *Service) Today(ctx context.Context, userID string) ([]*OccurrenceNode, error) {
	today := s.todayDate()
	if err := s.GenerateForDate(ctx, userID, today); err != nil {
		return nil, err
	}
	return s.CalendarDay(ctx, userID, today)
}

// GenerateForDate creates occurrences (state=pending) for every activity that
// is due on date for userID.  It is idempotent: calling it twice for the same
// date leaves existing occurrence states unchanged.
func (s *Service) GenerateForDate(ctx context.Context, userID string, date time.Time) error {
	flat, err := s.actRepo.ListByUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("occurrences service: generate: list activities: %w", err)
	}

	// Build a parent-id → children map and a quick lookup by id.
	byID := make(map[string]*activities.Activity, len(flat))
	children := make(map[string][]*activities.Activity)
	for _, a := range flat {
		byID[a.ID] = a
		if a.ParentID != nil {
			children[*a.ParentID] = append(children[*a.ParentID], a)
		}
	}

	weekday := int(date.Weekday()) // 0=Sun…6=Sat — matches days_of_week encoding

	for _, a := range flat {
		// Only process top-level, active activities here.
		if a.ParentID != nil || !a.IsActive {
			continue
		}
		if !isDue(a, weekday) {
			continue
		}

		// Parent is due — upsert the parent occurrence.
		if _, err := s.repo.Upsert(ctx, a.ID, date); err != nil {
			return fmt.Errorf("occurrences service: generate: upsert parent %s: %w", a.ID, err)
		}

		// Upsert all active children of this due parent.
		for _, child := range children[a.ID] {
			if !child.IsActive {
				continue
			}
			if _, err := s.repo.Upsert(ctx, child.ID, date); err != nil {
				return fmt.Errorf("occurrences service: generate: upsert child %s: %w", child.ID, err)
			}
		}
	}
	return nil
}

// CalendarDay returns the occurrence tree for the given date and user.
// It does NOT generate occurrences — call GenerateForDate first when needed.
func (s *Service) CalendarDay(ctx context.Context, userID string, date time.Time) ([]*OccurrenceNode, error) {
	occs, err := s.repo.ListByUserAndDate(ctx, userID, date)
	if err != nil {
		return nil, fmt.Errorf("occurrences service: calendar day: %w", err)
	}

	// Fetch activity metadata (title, parent_id) to build the tree.
	flat, err := s.actRepo.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("occurrences service: calendar day: list activities: %w", err)
	}
	actByID := make(map[string]*activities.Activity, len(flat))
	for _, a := range flat {
		actByID[a.ID] = a
	}

	return buildOccurrenceTree(occs, actByID), nil
}

// SetState applies a state change to the occurrence identified by occurrenceID,
// runs rollup, and persists every change.  Returns the updated occurrence group
// (parent node with children) for the caller to return to the HTTP layer.
// Returns ErrNotFound when the occurrence does not exist or does not belong to
// userID (cross-user access is treated as not-found to avoid leaking existence).
// Returns ErrInvalidState for unsupported state values.
func (s *Service) SetState(
	ctx context.Context, userID, occurrenceID, newState string,
) ([]*OccurrenceNode, error) {
	if _, ok := validStates[newState]; !ok {
		return nil, ErrInvalidState
	}

	// Load the target occurrence.
	occ, err := s.repo.GetByID(ctx, occurrenceID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("occurrences service: set state: get by id: %w", err)
	}

	// Verify ownership: the occurrence's activity must belong to userID.
	act, err := s.actRepo.GetByID(ctx, occ.ActivityID)
	if err != nil {
		return nil, fmt.Errorf("occurrences service: set state: get activity: %w", err)
	}
	if act.UserID != userID {
		return nil, ErrNotFound // treat cross-user as not-found
	}

	// Determine the parent activity id for this occurrence's group.
	parentActivityID := occ.ActivityID
	if act.ParentID != nil {
		parentActivityID = *act.ParentID
	}

	// Fetch the entire occurrence group (parent + all children) for the date.
	groupOccs, err := s.repo.ListGroupByParentAndDate(ctx, parentActivityID, occ.OccurDate)
	if err != nil {
		return nil, fmt.Errorf("occurrences service: set state: list group: %w", err)
	}

	// Map group occurrences to rollup.NodeState values.
	// The first occurrence is the parent (ListGroupByParentAndDate guarantees
	// parent first via ORDER BY parent_id NULLS FIRST).
	parentNode := rollup.NodeState{ID: groupOccs[0].ID, State: groupOccs[0].State}
	childNodes := make([]rollup.NodeState, 0, len(groupOccs)-1)
	for _, o := range groupOccs[1:] {
		childNodes = append(childNodes, rollup.NodeState{ID: o.ID, State: o.State})
	}

	changes := rollup.Apply(parentNode, childNodes, occurrenceID, newState)

	// Persist every returned change.
	for _, ch := range changes {
		if _, err := s.repo.UpdateState(ctx, ch.ID, ch.State); err != nil {
			return nil, fmt.Errorf("occurrences service: set state: update state %s: %w", ch.ID, err)
		}
	}

	// Return the refreshed group as a tree.
	// Re-fetch the group so the response reflects updated completed_at timestamps.
	updatedGroup, err := s.repo.ListGroupByParentAndDate(ctx, parentActivityID, occ.OccurDate)
	if err != nil {
		return nil, fmt.Errorf("occurrences service: set state: reload group: %w", err)
	}

	flat, err := s.actRepo.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("occurrences service: set state: list activities: %w", err)
	}
	actByID := make(map[string]*activities.Activity, len(flat))
	for _, a := range flat {
		actByID[a.ID] = a
	}

	return buildOccurrenceTree(updatedGroup, actByID), nil
}

// CalendarSummary returns per-day aggregated occurrence counts for the given
// user over the inclusive date range [from, to].
func (s *Service) CalendarSummary(
	ctx context.Context, userID string, from, to time.Time,
) ([]*DaySummary, error) {
	rows, err := s.repo.ListCalendarSummary(ctx, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("occurrences service: calendar summary: %w", err)
	}

	result := make([]*DaySummary, len(rows))
	for i, r := range rows {
		result[i] = &DaySummary{
			Date:    r.Date.Format("2006-01-02"),
			Pending: r.Pending,
			Partial: r.Partial,
			Done:    r.Done,
			Total:   r.Total,
		}
	}
	return result, nil
}

// ActivityHistory returns the state timeline for a single activity owned by
// userID over the inclusive date range [from, to].  Returns ErrNotFound when
// the activity does not exist or belongs to another user.
func (s *Service) ActivityHistory(
	ctx context.Context, userID, activityID string, from, to time.Time,
) ([]*ActivityHistoryEntry, error) {
	// Ownership check.
	act, err := s.actRepo.GetByID(ctx, activityID)
	if err != nil {
		if errors.Is(err, activities.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("occurrences service: activity history: get activity: %w", err)
	}
	if act.UserID != userID {
		return nil, ErrNotFound
	}

	occs, err := s.repo.ListByActivityAndDateRange(ctx, activityID, from, to)
	if err != nil {
		return nil, fmt.Errorf("occurrences service: activity history: %w", err)
	}

	result := make([]*ActivityHistoryEntry, len(occs))
	for i, o := range occs {
		result[i] = &ActivityHistoryEntry{
			Date:  o.OccurDate.Format("2006-01-02"),
			State: o.State,
		}
	}
	return result, nil
}

// todayDate returns midnight of today in the Jakarta timezone, as a UTC
// time.Time suitable for comparison against DATE columns.
func (s *Service) todayDate() time.Time {
	now := time.Now().In(s.location)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

// isDue reports whether activity a is due on weekday wd (0=Sunday…6=Saturday).
// Only top-level activities are passed here; the caller screens for IsActive.
func isDue(a *activities.Activity, weekday int) bool {
	if a.Freq == "daily" {
		return true
	}
	// freq == "weekly"
	for _, d := range a.DaysOfWeek {
		if int(d) == weekday {
			return true
		}
	}
	return false
}

// buildOccurrenceTree converts a flat list of occurrences + the activity map
// into a tree of OccurrenceNode values.  Top-level occurrences (whose activity
// has no parent) are the roots; child occurrences appear under their parent's
// Children slice.
func buildOccurrenceTree(
	occs []*Occurrence,
	actByID map[string]*activities.Activity,
) []*OccurrenceNode {
	if len(occs) == 0 {
		return []*OccurrenceNode{}
	}

	// Map occurrence id → OccurrenceNode.
	nodeByOccID := make(map[string]*OccurrenceNode, len(occs))
	// Map activity id → OccurrenceNode (to attach children to their parent).
	nodeByActID := make(map[string]*OccurrenceNode, len(occs))

	for _, o := range occs {
		a := actByID[o.ActivityID]
		title := ""
		if a != nil {
			title = a.Title
		}
		node := &OccurrenceNode{
			ID:          o.ID,
			ActivityID:  o.ActivityID,
			Title:       title,
			State:       o.State,
			CompletedAt: o.CompletedAt,
			Children:    []*OccurrenceNode{},
		}
		nodeByOccID[o.ID] = node
		nodeByActID[o.ActivityID] = node
	}

	roots := make([]*OccurrenceNode, 0)
	for _, o := range occs {
		node := nodeByOccID[o.ID]
		a := actByID[o.ActivityID]
		if a == nil || a.ParentID == nil {
			// Top-level occurrence.
			roots = append(roots, node)
		} else if parentNode, ok := nodeByActID[*a.ParentID]; ok {
			parentNode.Children = append(parentNode.Children, node)
		} else {
			// Parent occurrence not in the set (shouldn't happen in normal flow).
			roots = append(roots, node)
		}
	}
	return roots
}
