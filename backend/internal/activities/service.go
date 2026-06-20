package activities

import (
	"context"
	"errors"
	"fmt"

	"github.com/lib/pq"
)

// Service-level sentinel errors.
// The httpapi layer maps these to the appropriate HTTP status codes.
var (
	// ErrInvalidSchedule is returned when the freq/days_of_week combination is
	// inconsistent: "weekly" with an empty days_of_week, or "daily" with a
	// non-empty days_of_week.
	ErrInvalidSchedule = errors.New("activities: freq and days_of_week are inconsistent")

	// ErrInvalidParent is returned when the requested parent_id does not exist,
	// does not belong to the authenticated user, is itself a sub-activity, or
	// equals the activity's own id.
	ErrInvalidParent = errors.New("activities: parent not found or not eligible")

	// ErrHasChildren is returned when an attempt is made to assign a parent_id to
	// an activity that already has children (would create nesting deeper than one level).
	ErrHasChildren = errors.New("activities: cannot assign a parent to an activity that already has children")
)

// ActivityNode is the tree representation of an Activity returned by List.
// Top-level activities are the roots; sub-activities appear in Children.
// Children is always a non-nil slice so the JSON response encodes "children":[]
// rather than "children":null.
type ActivityNode struct {
	Activity
	Children []*ActivityNode `json:"children"`
}

// CreateInput carries the caller-supplied fields for creating a new activity.
type CreateInput struct {
	ParentID   *string
	Title      string
	Notes      *string
	Freq       string
	DaysOfWeek []int64
	TimeOfDay  string
	SortOrder  int
	IsActive   bool
}

// UpdateInput carries the caller-supplied fields for a partial update.
// A nil pointer means "field not provided — leave unchanged".
//
// Note: clearing nullable string fields (Notes, ParentID) to SQL NULL is not
// supported via this struct; a nil value is treated as "not provided". This
// limitation is intentional for M3 and can be addressed in a later milestone
// with a custom optional-value wrapper.
type UpdateInput struct {
	Title      *string
	Notes      *string
	Freq       *string
	DaysOfWeek *[]int64
	TimeOfDay  *string
	SortOrder  *int
	IsActive   *bool
	// ParentID: nil = not provided; non-nil = set to this value (pass a pointer
	// to an empty string to express "clear", though the service treats the empty
	// string as an invalid UUID and will fail validation).
	ParentID *string
}

// Service provides business-logic operations over activity templates.
// All write operations are scoped to the authenticated user; the repository is
// accessed only via the Repository interface.
type Service struct {
	repo Repository
}

// NewService constructs a Service backed by the provided Repository.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Create inserts a new activity owned by userID.
// If CreateInput.ParentID is set, the referenced parent must belong to userID,
// must itself be a top-level activity (its own ParentID must be nil), and must
// not equal the new activity's future id (self-reference is rejected by
// the single-level-depth check implicitly, since the activity does not yet exist).
func (s *Service) Create(ctx context.Context, userID string, in CreateInput) (*Activity, error) {
	// Normalise nil DaysOfWeek to empty slice so the DB stores '{}' not NULL.
	days := in.DaysOfWeek
	if days == nil {
		days = []int64{}
	}

	// Validate schedule before hitting the DB.
	if err := validateSchedule(in.Freq, pq.Int64Array(days)); err != nil {
		return nil, err
	}

	if in.ParentID != nil {
		if err := s.validateParent(ctx, *in.ParentID, userID, ""); err != nil {
			return nil, err
		}
	}

	a := &Activity{
		UserID:     userID,
		ParentID:   in.ParentID,
		Title:      in.Title,
		Notes:      in.Notes,
		Freq:       in.Freq,
		DaysOfWeek: pq.Int64Array(days),
		TimeOfDay:  in.TimeOfDay,
		SortOrder:  in.SortOrder,
		IsActive:   in.IsActive,
	}
	if err := s.repo.Create(ctx, a); err != nil {
		return nil, fmt.Errorf("activities service: create: %w", err)
	}
	return a, nil
}

// GetByID returns the activity identified by id, only if it belongs to userID.
// Returns ErrNotFound when the activity does not exist or belongs to another user
// (ownership violation is treated as not-found to avoid leaking existence).
func (s *Service) GetByID(ctx context.Context, userID, id string) (*Activity, error) {
	a, err := s.repo.GetByID(ctx, id)
	if err != nil {
		// ErrNotFound propagates as-is; all other errors are wrapped.
		return nil, err
	}
	if a.UserID != userID {
		return nil, ErrNotFound // treat as not-found — don't leak existence
	}
	return a, nil
}

// List returns the authenticated user's activities as a nested tree.
// A single DB query fetches all activities; the tree is built in memory from
// the ParentID field. Top-level activities are the roots; sub-activities appear
// under their parent's Children slice.
func (s *Service) List(ctx context.Context, userID string) ([]*ActivityNode, error) {
	flat, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("activities service: list: %w", err)
	}
	return buildTree(flat), nil
}

// Update applies the non-nil fields in UpdateInput to the activity identified
// by id. Ownership is enforced: ErrNotFound is returned when the activity
// does not exist or belongs to a different user.
//
// After applying the changes, the resulting freq/days_of_week combination is
// validated; ErrInvalidSchedule is returned if inconsistent.
func (s *Service) Update(ctx context.Context, userID, id string, in UpdateInput) (*Activity, error) {
	a, err := s.GetByID(ctx, userID, id)
	if err != nil {
		return nil, err
	}

	// Apply non-nil fields.
	if in.Title != nil {
		a.Title = *in.Title
	}
	if in.Notes != nil {
		a.Notes = in.Notes
	}
	if in.Freq != nil {
		a.Freq = *in.Freq
	}
	if in.DaysOfWeek != nil {
		a.DaysOfWeek = pq.Int64Array(*in.DaysOfWeek)
	}
	if in.TimeOfDay != nil {
		a.TimeOfDay = *in.TimeOfDay
	}
	if in.SortOrder != nil {
		a.SortOrder = *in.SortOrder
	}
	if in.IsActive != nil {
		a.IsActive = *in.IsActive
	}
	if in.ParentID != nil {
		// Validate: target parent must exist, belong to user, be top-level,
		// and not equal this activity's own id.
		if err := s.validateParent(ctx, *in.ParentID, userID, id); err != nil {
			return nil, err
		}
		// Validate: this activity must not already have children (depth constraint).
		if err := s.assertNoChildren(ctx, userID, id); err != nil {
			return nil, err
		}
		a.ParentID = in.ParentID
	}

	// Cross-field schedule validation on the merged result.
	if err := validateSchedule(a.Freq, a.DaysOfWeek); err != nil {
		return nil, err
	}

	if err := s.repo.Update(ctx, a); err != nil {
		return nil, fmt.Errorf("activities service: update: %w", err)
	}
	return a, nil
}

// Delete removes the activity identified by id, only if it belongs to userID.
// ON DELETE CASCADE in the schema removes all children and occurrences.
func (s *Service) Delete(ctx context.Context, userID, id string) error {
	// Ownership check before deleting.
	if _, err := s.GetByID(ctx, userID, id); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

// ---- private helpers --------------------------------------------------------

// validateSchedule returns ErrInvalidSchedule if the freq/days_of_week
// combination violates domain rules.
func validateSchedule(freq string, days pq.Int64Array) error {
	switch freq {
	case "daily":
		if len(days) > 0 {
			return ErrInvalidSchedule
		}
	case "weekly":
		if len(days) == 0 {
			return ErrInvalidSchedule
		}
	default:
		return ErrInvalidSchedule
	}
	return nil
}

// validateParent confirms that parentID is a valid, user-owned, top-level
// activity. selfID is the id of the activity being created or updated; pass ""
// on Create (the activity has no id yet). Returns ErrInvalidParent on any
// violation so that callers always map to 422 without revealing whether the
// parent exists.
func (s *Service) validateParent(ctx context.Context, parentID, userID, selfID string) error {
	if selfID != "" && parentID == selfID {
		return ErrInvalidParent
	}
	parent, err := s.repo.GetByID(ctx, parentID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrInvalidParent
		}
		return fmt.Errorf("activities service: validate parent: %w", err)
	}
	if parent.UserID != userID {
		return ErrInvalidParent // treat wrong-user as not-found
	}
	if parent.ParentID != nil {
		// Parent is itself a sub-activity; nesting to depth 2 is forbidden.
		return ErrInvalidParent
	}
	return nil
}

// assertNoChildren returns ErrHasChildren if the given activity already has at
// least one child. Uses a single ListByUser call and scans in memory — no N+1.
func (s *Service) assertNoChildren(ctx context.Context, userID, activityID string) error {
	flat, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("activities service: assert no children: %w", err)
	}
	for _, a := range flat {
		if a.ParentID != nil && *a.ParentID == activityID {
			return ErrHasChildren
		}
	}
	return nil
}

// buildTree converts a flat, parent-first-ordered list into a tree of
// ActivityNode values. Top-level nodes (ParentID == nil) are returned as roots;
// child nodes appear under their parent's Children slice.
// Children is always initialised to a non-nil empty slice so it serialises to
// "children":[] rather than "children":null.
func buildTree(flat []*Activity) []*ActivityNode {
	if len(flat) == 0 {
		return []*ActivityNode{}
	}

	byID := make(map[string]*ActivityNode, len(flat))
	for _, a := range flat {
		byID[a.ID] = &ActivityNode{Activity: *a, Children: []*ActivityNode{}}
	}

	roots := make([]*ActivityNode, 0)
	for _, a := range flat {
		node := byID[a.ID]
		if a.ParentID == nil {
			roots = append(roots, node)
		} else if parent, ok := byID[*a.ParentID]; ok {
			parent.Children = append(parent.Children, node)
		}
	}
	return roots
}
