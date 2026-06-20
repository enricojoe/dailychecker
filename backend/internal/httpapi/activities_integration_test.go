package httpapi_test

// Activities integration tests.
// TestMain and the HTTP helper functions (doReq, post, get, bearer,
// assertStatus, decodeJSON) are defined in auth_integration_test.go and are
// shared across all files in this package.

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// ── helpers specific to activities tests ─────────────────────────────────────

func patchReq(t *testing.T, path string, body interface{}, headers map[string]string) *http.Response {
	t.Helper()
	w := doReq(t, http.MethodPatch, path, body, headers)
	return w.Result()
}

func delReq(t *testing.T, path string, headers map[string]string) *http.Response {
	t.Helper()
	w := doReq(t, http.MethodDelete, path, nil, headers)
	return w.Result()
}

// setupUser registers a new user, logs in, and returns the access token.
func setupUser(t *testing.T) (userID, accessToken string) {
	t.Helper()
	username := fmt.Sprintf("actuser%d", time.Now().UnixNano())
	const password = "Password123!"

	w := post(t, "/api/auth/register", map[string]interface{}{
		"name": "Test User", "username": username, "password": password,
	}, nil)
	assertStatus(t, w, http.StatusCreated)
	var regBody struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &regBody)
	userID = regBody.ID

	w = post(t, "/api/auth/login", map[string]interface{}{
		"username": username, "password": password,
	}, nil)
	assertStatus(t, w, http.StatusOK)
	access, _ := tokenPairFrom(t, w)
	return userID, access
}

// createActivity is a test helper that POSTs to /api/activities and returns the
// activity id. Fails the test if the status is not 201.
func createActivity(t *testing.T, token string, body map[string]interface{}) string {
	t.Helper()
	w := post(t, "/api/activities", body, bearer(token))
	assertStatus(t, w, http.StatusCreated)
	var a struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &a)
	if a.ID == "" {
		t.Fatal("createActivity: id missing from response")
	}
	return a.ID
}

// ── tests ─────────────────────────────────────────────────────────────────────

// TestActivitiesUnauthenticated verifies that all activities endpoints reject
// requests without a valid Bearer token.
func TestActivitiesUnauthenticated(t *testing.T) {
	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/activities"},
		{http.MethodPost, "/api/activities"},
		{http.MethodGet, "/api/activities/some-id"},
		{http.MethodPatch, "/api/activities/some-id"},
		{http.MethodDelete, "/api/activities/some-id"},
	}
	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			w := doReq(t, ep.method, ep.path, nil, nil)
			assertStatus(t, w, http.StatusUnauthorized)
		})
	}
}

// TestActivitiesListEmpty verifies that a new user gets an empty activities list.
func TestActivitiesListEmpty(t *testing.T) {
	_, tok := setupUser(t)
	w := get(t, "/api/activities", bearer(tok))
	assertStatus(t, w, http.StatusOK)
	var tree []interface{}
	decodeJSON(t, w, &tree)
	if len(tree) != 0 {
		t.Errorf("want empty tree, got %d items", len(tree))
	}
}

// TestActivitiesCreateDaily covers creating a valid daily activity.
func TestActivitiesCreateDaily(t *testing.T) {
	_, tok := setupUser(t)
	w := post(t, "/api/activities", map[string]interface{}{
		"title":       "Morning Run",
		"freq":        "daily",
		"time_of_day": "07:00",
		"sort_order":  0,
		"is_active":   true,
	}, bearer(tok))
	assertStatus(t, w, http.StatusCreated)

	var a struct {
		ID          string `json:"id"`
		Title       string `json:"title"`
		Freq        string `json:"freq"`
		TimeOfDay   string `json:"time_of_day"`
		DaysOfWeek  []int  `json:"days_of_week"`
	}
	decodeJSON(t, w, &a)
	if a.ID == "" {
		t.Error("id missing")
	}
	if a.Title != "Morning Run" {
		t.Errorf("title: want %q got %q", "Morning Run", a.Title)
	}
	if a.Freq != "daily" {
		t.Errorf("freq: want %q got %q", "daily", a.Freq)
	}
	if a.TimeOfDay != "07:00:00" {
		t.Errorf("time_of_day: want %q got %q (expected normalisation to HH:MM:SS)", "07:00:00", a.TimeOfDay)
	}
}

// TestActivitiesCreateWeekly covers creating a valid weekly activity.
func TestActivitiesCreateWeekly(t *testing.T) {
	_, tok := setupUser(t)
	w := post(t, "/api/activities", map[string]interface{}{
		"title":        "Weekly Review",
		"freq":         "weekly",
		"days_of_week": []int{1, 5}, // Monday and Friday
		"time_of_day":  "18:00:00",
	}, bearer(tok))
	assertStatus(t, w, http.StatusCreated)
	var a struct {
		ID         string `json:"id"`
		DaysOfWeek []int  `json:"days_of_week"`
	}
	decodeJSON(t, w, &a)
	if a.ID == "" {
		t.Error("id missing")
	}
	if len(a.DaysOfWeek) != 2 {
		t.Errorf("days_of_week: want 2 elements, got %v", a.DaysOfWeek)
	}
}

// TestActivitiesCreateSubActivity covers creating a valid sub-activity under a
// top-level parent.
func TestActivitiesCreateSubActivity(t *testing.T) {
	_, tok := setupUser(t)

	parentID := createActivity(t, tok, map[string]interface{}{
		"title": "Parent", "freq": "daily", "time_of_day": "08:00",
	})

	w := post(t, "/api/activities", map[string]interface{}{
		"parent_id":   parentID,
		"title":       "Child",
		"freq":        "daily",
		"time_of_day": "08:05",
	}, bearer(tok))
	assertStatus(t, w, http.StatusCreated)

	var a struct {
		ID       string  `json:"id"`
		ParentID *string `json:"parent_id"`
	}
	decodeJSON(t, w, &a)
	if a.ParentID == nil || *a.ParentID != parentID {
		t.Errorf("parent_id: want %q, got %v", parentID, a.ParentID)
	}
}

// TestActivitiesCreateValidation covers the 422 validation cases for POST.
func TestActivitiesCreateValidation(t *testing.T) {
	_, tok := setupUser(t)

	// Create a valid parent for sub-activity depth tests.
	parentID := createActivity(t, tok, map[string]interface{}{
		"title": "Parent", "freq": "daily", "time_of_day": "08:00",
	})
	// Create a child to use as an invalid grandparent reference.
	childID := createActivity(t, tok, map[string]interface{}{
		"parent_id": parentID, "title": "Child", "freq": "daily", "time_of_day": "08:05",
	})

	cases := []struct {
		name string
		body map[string]interface{}
	}{
		{
			name: "missing title",
			body: map[string]interface{}{"freq": "daily", "time_of_day": "08:00"},
		},
		{
			name: "missing freq",
			body: map[string]interface{}{"title": "X", "time_of_day": "08:00"},
		},
		{
			name: "missing time_of_day",
			body: map[string]interface{}{"title": "X", "freq": "daily"},
		},
		{
			name: "invalid freq value",
			body: map[string]interface{}{"title": "X", "freq": "monthly", "time_of_day": "08:00"},
		},
		{
			name: "weekly with empty days_of_week",
			body: map[string]interface{}{"title": "X", "freq": "weekly", "days_of_week": []int{}, "time_of_day": "08:00"},
		},
		{
			name: "daily with non-empty days_of_week",
			body: map[string]interface{}{"title": "X", "freq": "daily", "days_of_week": []int{1}, "time_of_day": "08:00"},
		},
		{
			name: "days_of_week value out of range",
			body: map[string]interface{}{"title": "X", "freq": "weekly", "days_of_week": []int{7}, "time_of_day": "08:00"},
		},
		{
			name: "days_of_week with duplicates",
			body: map[string]interface{}{"title": "X", "freq": "weekly", "days_of_week": []int{1, 1}, "time_of_day": "08:00"},
		},
		{
			name: "invalid time_of_day format",
			body: map[string]interface{}{"title": "X", "freq": "daily", "time_of_day": "25:00"},
		},
		{
			name: "time_of_day garbage",
			body: map[string]interface{}{"title": "X", "freq": "daily", "time_of_day": "not-a-time"},
		},
		{
			name: "parent_id pointing to a sub-activity (depth > 1 forbidden)",
			body: map[string]interface{}{
				"parent_id": childID, "title": "Grandchild",
				"freq": "daily", "time_of_day": "08:10",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := post(t, "/api/activities", tc.body, bearer(tok))
			assertStatus(t, w, http.StatusUnprocessableEntity)
		})
	}
}

// TestActivitiesCreateOwnershipRejection verifies that using another user's
// activity as parent_id returns 422 (invalid parent, not leaking 404).
func TestActivitiesCreateOwnershipRejection(t *testing.T) {
	_, tok1 := setupUser(t)
	_, tok2 := setupUser(t)

	// User1 creates an activity.
	user1ActivityID := createActivity(t, tok1, map[string]interface{}{
		"title": "User1 Activity", "freq": "daily", "time_of_day": "09:00",
	})

	// User2 tries to use it as a parent.
	w := post(t, "/api/activities", map[string]interface{}{
		"parent_id":   user1ActivityID,
		"title":       "User2 Child",
		"freq":        "daily",
		"time_of_day": "09:05",
	}, bearer(tok2))
	assertStatus(t, w, http.StatusUnprocessableEntity)
}

// TestActivitiesGetByID covers the get-by-id endpoint: happy path and 404.
func TestActivitiesGetByID(t *testing.T) {
	_, tok := setupUser(t)
	id := createActivity(t, tok, map[string]interface{}{
		"title": "Fetch Me", "freq": "daily", "time_of_day": "10:00",
	})

	t.Run("found", func(t *testing.T) {
		w := get(t, "/api/activities/"+id, bearer(tok))
		assertStatus(t, w, http.StatusOK)
		var a struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		}
		decodeJSON(t, w, &a)
		if a.ID != id {
			t.Errorf("id: want %q got %q", id, a.ID)
		}
	})

	t.Run("not found", func(t *testing.T) {
		w := get(t, "/api/activities/00000000-0000-0000-0000-000000000000", bearer(tok))
		assertStatus(t, w, http.StatusNotFound)
	})
}

// TestActivitiesGetByIDOwnership verifies that fetching another user's activity
// returns 404 (not leaking existence).
func TestActivitiesGetByIDOwnership(t *testing.T) {
	_, tok1 := setupUser(t)
	_, tok2 := setupUser(t)

	id := createActivity(t, tok1, map[string]interface{}{
		"title": "User1 Private", "freq": "daily", "time_of_day": "11:00",
	})

	w := get(t, "/api/activities/"+id, bearer(tok2))
	assertStatus(t, w, http.StatusNotFound)
}

// TestActivitiesPatch covers partial updates via PATCH.
func TestActivitiesPatch(t *testing.T) {
	_, tok := setupUser(t)
	id := createActivity(t, tok, map[string]interface{}{
		"title": "Original Title", "freq": "daily", "time_of_day": "12:00",
	})

	t.Run("update title", func(t *testing.T) {
		w := doReq(t, http.MethodPatch, "/api/activities/"+id,
			map[string]interface{}{"title": "Updated Title"},
			bearer(tok))
		assertStatus(t, w, http.StatusOK)
		var a struct{ Title string `json:"title"` }
		decodeJSON(t, w, &a)
		if a.Title != "Updated Title" {
			t.Errorf("title: want %q got %q", "Updated Title", a.Title)
		}
	})

	t.Run("update sort_order and is_active", func(t *testing.T) {
		w := doReq(t, http.MethodPatch, "/api/activities/"+id,
			map[string]interface{}{"sort_order": 5, "is_active": false},
			bearer(tok))
		assertStatus(t, w, http.StatusOK)
		var a struct {
			SortOrder int  `json:"sort_order"`
			IsActive  bool `json:"is_active"`
		}
		decodeJSON(t, w, &a)
		if a.SortOrder != 5 {
			t.Errorf("sort_order: want 5 got %d", a.SortOrder)
		}
		if a.IsActive {
			t.Error("is_active: want false")
		}
	})

	t.Run("update freq to weekly with days_of_week", func(t *testing.T) {
		w := doReq(t, http.MethodPatch, "/api/activities/"+id,
			map[string]interface{}{"freq": "weekly", "days_of_week": []int{0, 6}},
			bearer(tok))
		assertStatus(t, w, http.StatusOK)
		var a struct {
			Freq       string `json:"freq"`
			DaysOfWeek []int  `json:"days_of_week"`
		}
		decodeJSON(t, w, &a)
		if a.Freq != "weekly" {
			t.Errorf("freq: want weekly got %q", a.Freq)
		}
		if len(a.DaysOfWeek) != 2 {
			t.Errorf("days_of_week: want 2, got %v", a.DaysOfWeek)
		}
	})
}

// TestActivitiesPatchValidation covers the 422 cases for PATCH.
func TestActivitiesPatchValidation(t *testing.T) {
	_, tok := setupUser(t)

	// Base daily activity.
	id := createActivity(t, tok, map[string]interface{}{
		"title": "Base", "freq": "daily", "time_of_day": "14:00",
	})
	// Top-level activity with children, to test ErrHasChildren.
	parentWithChildren := createActivity(t, tok, map[string]interface{}{
		"title": "Parent", "freq": "daily", "time_of_day": "15:00",
	})
	createActivity(t, tok, map[string]interface{}{
		"parent_id": parentWithChildren, "title": "Child",
		"freq": "daily", "time_of_day": "15:05",
	})

	cases := []struct {
		name   string
		target string
		body   map[string]interface{}
	}{
		{
			name:   "empty title",
			target: id,
			body:   map[string]interface{}{"title": ""},
		},
		{
			name:   "invalid freq",
			target: id,
			body:   map[string]interface{}{"freq": "biweekly"},
		},
		{
			name:   "invalid time_of_day",
			target: id,
			body:   map[string]interface{}{"time_of_day": "99:00"},
		},
		{
			name:   "days_of_week with out-of-range value",
			target: id,
			body:   map[string]interface{}{"days_of_week": []int{8}},
		},
		{
			name:   "days_of_week with duplicate",
			target: id,
			body:   map[string]interface{}{"days_of_week": []int{2, 2}},
		},
		{
			name:   "parent_id equal own id",
			target: id,
			body:   map[string]interface{}{"parent_id": id},
		},
		{
			name:   "switch to weekly without providing days_of_week (cross-field, service validates)",
			target: id, // currently daily with empty days_of_week
			body:   map[string]interface{}{"freq": "weekly"},
		},
		{
			name:   "add parent_id to activity that already has children",
			target: parentWithChildren,
			body:   map[string]interface{}{"parent_id": id},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doReq(t, http.MethodPatch, "/api/activities/"+tc.target, tc.body, bearer(tok))
			assertStatus(t, w, http.StatusUnprocessableEntity)
		})
	}
}

// TestActivitiesPatchOwnership verifies that patching another user's activity
// returns 404.
func TestActivitiesPatchOwnership(t *testing.T) {
	_, tok1 := setupUser(t)
	_, tok2 := setupUser(t)

	id := createActivity(t, tok1, map[string]interface{}{
		"title": "User1 Activity", "freq": "daily", "time_of_day": "16:00",
	})

	w := doReq(t, http.MethodPatch, "/api/activities/"+id,
		map[string]interface{}{"title": "Hacked"},
		bearer(tok2))
	assertStatus(t, w, http.StatusNotFound)
}

// TestActivitiesDelete covers the delete endpoint.
func TestActivitiesDelete(t *testing.T) {
	_, tok := setupUser(t)
	id := createActivity(t, tok, map[string]interface{}{
		"title": "Delete Me", "freq": "daily", "time_of_day": "17:00",
	})

	t.Run("delete existing", func(t *testing.T) {
		w := doReq(t, http.MethodDelete, "/api/activities/"+id, nil, bearer(tok))
		assertStatus(t, w, http.StatusNoContent)
	})
	t.Run("delete same id again → 404", func(t *testing.T) {
		w := doReq(t, http.MethodDelete, "/api/activities/"+id, nil, bearer(tok))
		assertStatus(t, w, http.StatusNotFound)
	})
	t.Run("delete non-existent", func(t *testing.T) {
		w := doReq(t, http.MethodDelete, "/api/activities/00000000-0000-0000-0000-000000000000", nil, bearer(tok))
		assertStatus(t, w, http.StatusNotFound)
	})
}

// TestActivitiesDeleteOwnership verifies that deleting another user's activity
// returns 404.
func TestActivitiesDeleteOwnership(t *testing.T) {
	_, tok1 := setupUser(t)
	_, tok2 := setupUser(t)

	id := createActivity(t, tok1, map[string]interface{}{
		"title": "User1 Activity", "freq": "daily", "time_of_day": "18:00",
	})

	w := doReq(t, http.MethodDelete, "/api/activities/"+id, nil, bearer(tok2))
	assertStatus(t, w, http.StatusNotFound)
}

// TestActivitiesListTree verifies that the list endpoint returns a nested tree
// where children appear under their parent.
func TestActivitiesListTree(t *testing.T) {
	_, tok := setupUser(t)

	parentID := createActivity(t, tok, map[string]interface{}{
		"title": "Parent Activity", "freq": "daily", "time_of_day": "06:00",
	})
	childID := createActivity(t, tok, map[string]interface{}{
		"parent_id": parentID, "title": "Child Activity",
		"freq": "daily", "time_of_day": "06:05",
	})

	w := get(t, "/api/activities", bearer(tok))
	assertStatus(t, w, http.StatusOK)

	var tree []struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		ParentID *string `json:"parent_id"`
		Children []struct {
			ID       string  `json:"id"`
			Title    string  `json:"title"`
			ParentID *string `json:"parent_id"`
		} `json:"children"`
	}
	decodeJSON(t, w, &tree)

	// Find the parent node.
	var parentNode *struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		ParentID *string `json:"parent_id"`
		Children []struct {
			ID       string  `json:"id"`
			Title    string  `json:"title"`
			ParentID *string `json:"parent_id"`
		} `json:"children"`
	}
	for i := range tree {
		if tree[i].ID == parentID {
			parentNode = &tree[i]
			break
		}
	}
	if parentNode == nil {
		t.Fatalf("parent activity %q not found in tree root; tree = %+v", parentID, tree)
	}
	if parentNode.ParentID != nil {
		t.Errorf("parent should have null parent_id, got %v", parentNode.ParentID)
	}
	if len(parentNode.Children) != 1 {
		t.Fatalf("parent should have 1 child, got %d; children = %+v", len(parentNode.Children), parentNode.Children)
	}
	child := parentNode.Children[0]
	if child.ID != childID {
		t.Errorf("child id: want %q got %q", childID, child.ID)
	}
	if child.ParentID == nil || *child.ParentID != parentID {
		t.Errorf("child.parent_id: want %q got %v", parentID, child.ParentID)
	}
}

// TestActivitiesDeleteCascadesToChildren verifies that deleting a parent also
// removes its children (via ON DELETE CASCADE), reflected in the list response.
func TestActivitiesDeleteCascadesToChildren(t *testing.T) {
	_, tok := setupUser(t)

	parentID := createActivity(t, tok, map[string]interface{}{
		"title": "Cascade Parent", "freq": "daily", "time_of_day": "20:00",
	})
	createActivity(t, tok, map[string]interface{}{
		"parent_id": parentID, "title": "Cascade Child",
		"freq": "daily", "time_of_day": "20:05",
	})

	// Delete the parent.
	w := doReq(t, http.MethodDelete, "/api/activities/"+parentID, nil, bearer(tok))
	assertStatus(t, w, http.StatusNoContent)

	// The list should now be empty for this user (all activities were deleted).
	w = get(t, "/api/activities", bearer(tok))
	assertStatus(t, w, http.StatusOK)
	var tree []interface{}
	decodeJSON(t, w, &tree)
	if len(tree) != 0 {
		t.Errorf("expected empty tree after cascade delete, got %d items", len(tree))
	}
}
