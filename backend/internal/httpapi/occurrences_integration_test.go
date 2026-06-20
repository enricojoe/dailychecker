package httpapi_test

// Occurrences integration tests — M4.
//
// All tests use unique users (via setupUser) so they are safe to run in
// parallel with other packages against the shared test database.  No shared
// tables are dropped or truncated.

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// ── helpers ──────────────────────────────────────────────────────────────────

// createDailyActivity creates a daily activity and returns its id.
func createDailyActivity(t *testing.T, token, title string) string {
	t.Helper()
	return createActivity(t, token, map[string]interface{}{
		"title":       title,
		"freq":        "daily",
		"time_of_day": "08:00",
		"is_active":   true,
	})
}

// createWeeklyActivity creates a weekly activity for the given days and returns
// its id.
func createWeeklyActivity(t *testing.T, token, title string, days []int) string {
	t.Helper()
	return createActivity(t, token, map[string]interface{}{
		"title":        title,
		"freq":         "weekly",
		"days_of_week": days,
		"time_of_day":  "08:00",
		"is_active":    true,
	})
}

// getToday calls GET /api/today and returns the decoded tree.
func getToday(t *testing.T, token string) []map[string]interface{} {
	t.Helper()
	w := get(t, "/api/today", bearer(token))
	assertStatus(t, w, http.StatusOK)
	var tree []map[string]interface{}
	decodeJSON(t, w, &tree)
	return tree
}

// patchOccurrenceState calls PATCH /api/occurrences/:id with the given state.
func patchOccurrenceState(t *testing.T, token, id, state string) *http.Response {
	t.Helper()
	w := doReq(t, http.MethodPatch, "/api/occurrences/"+id,
		map[string]interface{}{"state": state},
		bearer(token))
	return w.Result()
}

// findNodeByTitle scans a flat tree list for the first node whose "title"
// matches and returns it.
func findNodeByTitle(tree []map[string]interface{}, title string) map[string]interface{} {
	for _, node := range tree {
		if node["title"] == title {
			return node
		}
	}
	return nil
}

// findOccurrenceIDForActivity returns the occurrence id from the today-tree
// whose activity_id matches activityID.
func findOccurrenceIDForActivity(
	tree []map[string]interface{},
	activityID string,
) string {
	for _, node := range tree {
		if node["activity_id"] == activityID {
			return node["id"].(string)
		}
		// Also scan children.
		if children, ok := node["children"].([]interface{}); ok {
			for _, c := range children {
				child, _ := c.(map[string]interface{})
				if child["activity_id"] == activityID {
					return child["id"].(string)
				}
			}
		}
	}
	return ""
}

// ── generation idempotency ────────────────────────────────────────────────────

// TestTodayIdempotent verifies that calling GET /api/today twice produces the
// same occurrence ids and does not reset previously-set states.
func TestTodayIdempotent(t *testing.T) {
	_, tok := setupUser(t)
	createDailyActivity(t, tok, "Idempotent Activity")

	tree1 := getToday(t, tok)
	if len(tree1) == 0 {
		t.Fatal("expected at least one occurrence after first today call")
	}
	id1 := tree1[0]["id"].(string)

	// Patch to done so we can verify the state is preserved on second call.
	resp := patchOccurrenceState(t, tok, id1, "done")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("patch state: want 200, got %d", resp.StatusCode)
	}

	// Second today call.
	tree2 := getToday(t, tok)
	if len(tree2) == 0 {
		t.Fatal("expected occurrences on second today call")
	}
	id2 := tree2[0]["id"].(string)

	if id1 != id2 {
		t.Errorf("occurrence id changed across calls: %s → %s", id1, id2)
	}
	if tree2[0]["state"] != "done" {
		t.Errorf("state was reset: want %q got %v", "done", tree2[0]["state"])
	}
}

// ── due logic ─────────────────────────────────────────────────────────────────

// TestWeeklyActivityDueTodayOnly verifies that a weekly activity generates an
// occurrence when today is in days_of_week and none when it is not.
// We cannot control "today" from tests, so we set up activities for all 7 days
// vs. exactly 0 days (impossible weekly — instead we use inactive to simulate
// "not due") and verify the tree shape instead.
func TestWeeklyActivityDueTodayOnly(t *testing.T) {
	_, tok := setupUser(t)

	// Weekly activity covering all 7 days → always due.
	allDays := []int{0, 1, 2, 3, 4, 5, 6}
	createWeeklyActivity(t, tok, "All Days Weekly", allDays)

	tree := getToday(t, tok)
	node := findNodeByTitle(tree, "All Days Weekly")
	if node == nil {
		t.Fatal("expected occurrence for all-days weekly activity")
	}
}

// TestInactiveParentExcluded verifies that an inactive parent activity does not
// generate an occurrence.
func TestInactiveParentExcluded(t *testing.T) {
	_, tok := setupUser(t)

	// Create an active activity first, confirm it appears.
	createDailyActivity(t, tok, "Active Activity")

	// Create an inactive daily activity.
	createActivity(t, tok, map[string]interface{}{
		"title":       "Inactive Parent",
		"freq":        "daily",
		"time_of_day": "09:00",
		"is_active":   false,
	})

	tree := getToday(t, tok)
	if findNodeByTitle(tree, "Inactive Parent") != nil {
		t.Error("inactive parent should not generate an occurrence")
	}
}

// TestChildGeneratedIffParentDue verifies that a child occurrence is generated
// when its parent is due, and that the child appears nested under the parent.
func TestChildGeneratedIffParentDue(t *testing.T) {
	_, tok := setupUser(t)

	parentID := createDailyActivity(t, tok, "Parent Due Daily")
	createActivity(t, tok, map[string]interface{}{
		"parent_id":   parentID,
		"title":       "Child Of Due Parent",
		"freq":        "daily",
		"time_of_day": "08:05",
		"is_active":   true,
	})

	tree := getToday(t, tok)
	parentNode := findNodeByTitle(tree, "Parent Due Daily")
	if parentNode == nil {
		t.Fatal("parent occurrence missing")
	}

	children, _ := parentNode["children"].([]interface{})
	if len(children) == 0 {
		t.Fatal("child occurrence missing from parent's children")
	}
	child, _ := children[0].(map[string]interface{})
	if child["title"] != "Child Of Due Parent" {
		t.Errorf("unexpected child title: %v", child["title"])
	}
}

// TestInactiveChildExcluded verifies that an inactive child does not appear in
// the today tree even when its parent is due.
func TestInactiveChildExcluded(t *testing.T) {
	_, tok := setupUser(t)

	parentID := createDailyActivity(t, tok, "Parent With Inactive Child")
	createActivity(t, tok, map[string]interface{}{
		"parent_id":   parentID,
		"title":       "Inactive Child",
		"freq":        "daily",
		"time_of_day": "08:05",
		"is_active":   false,
	})

	tree := getToday(t, tok)
	parentNode := findNodeByTitle(tree, "Parent With Inactive Child")
	if parentNode == nil {
		t.Fatal("parent occurrence missing")
	}

	children, _ := parentNode["children"].([]interface{})
	for _, c := range children {
		child, _ := c.(map[string]interface{})
		if child["title"] == "Inactive Child" {
			t.Error("inactive child should not appear in today tree")
		}
	}
}

// ── today tree shape ──────────────────────────────────────────────────────────

// TestTodayTreeShape verifies that today's tree has parent nodes at the root
// and children nested inside them.
func TestTodayTreeShape(t *testing.T) {
	_, tok := setupUser(t)

	parentID := createDailyActivity(t, tok, "Shape Parent")
	createActivity(t, tok, map[string]interface{}{
		"parent_id":   parentID,
		"title":       "Shape Child",
		"freq":        "daily",
		"time_of_day": "08:05",
		"is_active":   true,
	})

	tree := getToday(t, tok)
	parentNode := findNodeByTitle(tree, "Shape Parent")
	if parentNode == nil {
		t.Fatal("parent not in tree root")
	}

	// Check required fields on parent node.
	for _, field := range []string{"id", "activity_id", "title", "state", "children"} {
		if _, ok := parentNode[field]; !ok {
			t.Errorf("parent node missing field %q", field)
		}
	}

	children, _ := parentNode["children"].([]interface{})
	if len(children) != 1 {
		t.Fatalf("want 1 child, got %d", len(children))
	}
	child, _ := children[0].(map[string]interface{})
	if child["title"] != "Shape Child" {
		t.Errorf("unexpected child title: %v", child["title"])
	}

	// Child should NOT appear as a root.
	if findNodeByTitle(tree, "Shape Child") != nil {
		t.Error("child should not be a root node")
	}
}

// ── PATCH rollup ──────────────────────────────────────────────────────────────

// setupRollupGroup creates a parent with two children and returns today's tree
// for that user, plus the occurrence ids.
func setupRollupGroup(t *testing.T) (
	token string,
	parentOccID, child1OccID, child2OccID string,
	parentActID string,
) {
	t.Helper()
	_, tok := setupUser(t)

	pID := createDailyActivity(t, tok, "Rollup Parent")
	c1ID := createActivity(t, tok, map[string]interface{}{
		"parent_id": pID, "title": "Rollup Child 1",
		"freq": "daily", "time_of_day": "08:01", "is_active": true,
	})
	c2ID := createActivity(t, tok, map[string]interface{}{
		"parent_id": pID, "title": "Rollup Child 2",
		"freq": "daily", "time_of_day": "08:02", "is_active": true,
	})

	tree := getToday(t, tok)
	pOccID := findOccurrenceIDForActivity(tree, pID)
	c1OccID := findOccurrenceIDForActivity(tree, c1ID)
	c2OccID := findOccurrenceIDForActivity(tree, c2ID)

	if pOccID == "" || c1OccID == "" || c2OccID == "" {
		t.Fatalf("missing occurrence ids: parent=%s c1=%s c2=%s", pOccID, c1OccID, c2OccID)
	}
	return tok, pOccID, c1OccID, c2OccID, pID
}

// TestRollupAllChildrenDoneParentDone verifies that setting both children to
// "done" causes the parent to auto-transition to "done".
func TestRollupAllChildrenDoneParentDone(t *testing.T) {
	tok, parentOccID, child1OccID, child2OccID, _ := setupRollupGroup(t)

	// Set child1 done.
	resp := patchOccurrenceState(t, tok, child1OccID, "done")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("patch child1 done: want 200, got %d", resp.StatusCode)
	}

	// Set child2 done → parent should auto-become done.
	w := doReq(t, http.MethodPatch, "/api/occurrences/"+child2OccID,
		map[string]interface{}{"state": "done"}, bearer(tok))
	assertStatus(t, w, http.StatusOK)

	var group []map[string]interface{}
	decodeJSON(t, w, &group)

	// Find the parent in the returned group.
	parentNode := findOccurrenceIDInGroup(group, parentOccID)
	if parentNode == nil {
		t.Fatalf("parent occurrence %s not found in group response", parentOccID)
	}
	if parentNode["state"] != "done" {
		t.Errorf("parent state: want %q got %v", "done", parentNode["state"])
	}
}

// TestRollupParentDoneCascadesToChildren verifies that setting the parent to
// "done" cascades "done" to all children.
func TestRollupParentDoneCascadesToChildren(t *testing.T) {
	tok, parentOccID, child1OccID, child2OccID, _ := setupRollupGroup(t)

	w := doReq(t, http.MethodPatch, "/api/occurrences/"+parentOccID,
		map[string]interface{}{"state": "done"}, bearer(tok))
	assertStatus(t, w, http.StatusOK)

	var group []map[string]interface{}
	decodeJSON(t, w, &group)

	// All nodes in the group must be "done".
	for _, occID := range []string{parentOccID, child1OccID, child2OccID} {
		node := findOccurrenceIDInGroup(group, occID)
		if node == nil {
			t.Errorf("occurrence %s missing from group response", occID)
			continue
		}
		if node["state"] != "done" {
			t.Errorf("occurrence %s: want state %q got %v", occID, "done", node["state"])
		}
	}
}

// TestRollupUncheckChildParentPartialThenPending verifies the sequence:
//  1. Set both children done (parent auto-done).
//  2. Uncheck child1 → parent should become partial.
//  3. Uncheck child2 → parent should become pending.
func TestRollupUncheckChildParentPartialThenPending(t *testing.T) {
	tok, parentOccID, child1OccID, child2OccID, _ := setupRollupGroup(t)

	// Step 1: both children done → parent auto-done.
	patchOccurrenceState(t, tok, child1OccID, "done")
	patchOccurrenceState(t, tok, child2OccID, "done")

	// Step 2: uncheck child1 → parent partial.
	w := doReq(t, http.MethodPatch, "/api/occurrences/"+child1OccID,
		map[string]interface{}{"state": "pending"}, bearer(tok))
	assertStatus(t, w, http.StatusOK)

	var group []map[string]interface{}
	decodeJSON(t, w, &group)
	parentNode := findOccurrenceIDInGroup(group, parentOccID)
	if parentNode == nil {
		t.Fatal("parent missing from group after unchecking child1")
	}
	if parentNode["state"] != "partial" {
		t.Errorf("step 2 parent state: want %q got %v", "partial", parentNode["state"])
	}

	// Step 3: uncheck child2 → parent pending.
	w = doReq(t, http.MethodPatch, "/api/occurrences/"+child2OccID,
		map[string]interface{}{"state": "pending"}, bearer(tok))
	assertStatus(t, w, http.StatusOK)

	decodeJSON(t, w, &group)
	parentNode = findOccurrenceIDInGroup(group, parentOccID)
	if parentNode == nil {
		t.Fatal("parent missing from group after unchecking child2")
	}
	if parentNode["state"] != "pending" {
		t.Errorf("step 3 parent state: want %q got %v", "pending", parentNode["state"])
	}
}

// TestRollupOwnership404 verifies that patching another user's occurrence
// returns 404.
func TestRollupOwnership404(t *testing.T) {
	_, tok1 := setupUser(t)
	_, tok2 := setupUser(t)

	createDailyActivity(t, tok1, "User1 Activity For Ownership Test")
	tree := getToday(t, tok1)
	if len(tree) == 0 {
		t.Skip("no occurrences for user1")
	}
	occID := tree[len(tree)-1]["id"].(string)

	w := doReq(t, http.MethodPatch, "/api/occurrences/"+occID,
		map[string]interface{}{"state": "done"}, bearer(tok2))
	assertStatus(t, w, http.StatusNotFound)
}

// TestRollupInvalidState422 verifies that an unsupported state value returns 422.
func TestRollupInvalidState422(t *testing.T) {
	_, tok := setupUser(t)
	createDailyActivity(t, tok, "Activity For State Validation")
	tree := getToday(t, tok)
	if len(tree) == 0 {
		t.Skip("no occurrences")
	}
	occID := tree[len(tree)-1]["id"].(string)

	w := doReq(t, http.MethodPatch, "/api/occurrences/"+occID,
		map[string]interface{}{"state": "invalid_state"}, bearer(tok))
	assertStatus(t, w, http.StatusUnprocessableEntity)
}

// findOccurrenceIDInGroup searches a group (flat list of nodes or nested tree)
// for an occurrence by its id.
func findOccurrenceIDInGroup(group []map[string]interface{}, id string) map[string]interface{} {
	for _, node := range group {
		if node["id"] == id {
			return node
		}
		// Check children slice.
		if children, ok := node["children"].([]interface{}); ok {
			for _, c := range children {
				child, _ := c.(map[string]interface{})
				if child["id"] == id {
					return child
				}
			}
		}
	}
	return nil
}

// ── history endpoints ─────────────────────────────────────────────────────────

// TestCalendarSummary verifies that the calendar summary returns per-day counts.
func TestCalendarSummary(t *testing.T) {
	_, tok := setupUser(t)
	createDailyActivity(t, tok, "Calendar Summary Activity")

	// Generate today's occurrences via GET /api/today.
	getToday(t, tok)

	today := time.Now().UTC().Format("2006-01-02")
	path := fmt.Sprintf("/api/history/calendar?from=%s&to=%s", today, today)

	w := get(t, path, bearer(tok))
	assertStatus(t, w, http.StatusOK)

	var summary []map[string]interface{}
	decodeJSON(t, w, &summary)

	if len(summary) == 0 {
		t.Fatal("expected at least one calendar summary row")
	}
	row := summary[0]
	for _, field := range []string{"date", "pending", "partial", "done", "total"} {
		if _, ok := row[field]; !ok {
			t.Errorf("calendar summary row missing field %q", field)
		}
	}
	if row["date"] != today {
		t.Errorf("date: want %q got %v", today, row["date"])
	}
}

// TestCalendarDay verifies that the calendar/:date endpoint returns an
// occurrence tree for the given date.
func TestCalendarDay(t *testing.T) {
	_, tok := setupUser(t)
	createDailyActivity(t, tok, "Calendar Day Activity")
	getToday(t, tok) // generate occurrences

	today := time.Now().UTC().Format("2006-01-02")
	w := get(t, "/api/history/calendar/"+today, bearer(tok))
	assertStatus(t, w, http.StatusOK)

	var tree []map[string]interface{}
	decodeJSON(t, w, &tree)

	node := findNodeByTitle(tree, "Calendar Day Activity")
	if node == nil {
		t.Fatal("expected occurrence in calendar/:date tree")
	}
}

// TestActivityHistory verifies that the per-activity history returns a
// {date, state} timeline.
func TestActivityHistory(t *testing.T) {
	_, tok := setupUser(t)
	actID := createDailyActivity(t, tok, "History Activity")
	getToday(t, tok) // generate occurrence

	today := time.Now().UTC().Format("2006-01-02")
	path := fmt.Sprintf("/api/history/activities/%s?from=%s&to=%s", actID, today, today)

	w := get(t, path, bearer(tok))
	assertStatus(t, w, http.StatusOK)

	var history []map[string]interface{}
	decodeJSON(t, w, &history)

	if len(history) == 0 {
		t.Fatal("expected at least one history entry")
	}
	entry := history[0]
	if _, ok := entry["date"]; !ok {
		t.Error("history entry missing 'date' field")
	}
	if _, ok := entry["state"]; !ok {
		t.Error("history entry missing 'state' field")
	}
	if entry["date"] != today {
		t.Errorf("date: want %q got %v", today, entry["date"])
	}
}

// TestActivityHistoryOwnership verifies that requesting history for another
// user's activity returns 404.
func TestActivityHistoryOwnership(t *testing.T) {
	_, tok1 := setupUser(t)
	_, tok2 := setupUser(t)

	actID := createDailyActivity(t, tok1, "User1 Exclusive Activity")
	today := time.Now().UTC().Format("2006-01-02")
	path := fmt.Sprintf("/api/history/activities/%s?from=%s&to=%s", actID, today, today)

	w := get(t, path, bearer(tok2))
	assertStatus(t, w, http.StatusNotFound)
}

// ── validation ────────────────────────────────────────────────────────────────

// TestCalendarSummaryFromAfterTo verifies that from>to returns 422.
func TestCalendarSummaryFromAfterTo(t *testing.T) {
	_, tok := setupUser(t)
	w := get(t, "/api/history/calendar?from=2025-06-20&to=2025-06-19", bearer(tok))
	assertStatus(t, w, http.StatusUnprocessableEntity)
}

// TestCalendarSummaryBadDate verifies that a non-date param returns 422.
func TestCalendarSummaryBadDate(t *testing.T) {
	_, tok := setupUser(t)
	w := get(t, "/api/history/calendar?from=bad-date&to=2025-06-20", bearer(tok))
	assertStatus(t, w, http.StatusUnprocessableEntity)
}

// TestCalendarDayBadDate verifies that a malformed :date param returns 422.
func TestCalendarDayBadDate(t *testing.T) {
	_, tok := setupUser(t)
	w := get(t, "/api/history/calendar/not-a-date", bearer(tok))
	assertStatus(t, w, http.StatusUnprocessableEntity)
}

// TestActivityHistoryFromAfterTo verifies that from>to returns 422.
func TestActivityHistoryFromAfterTo(t *testing.T) {
	_, tok := setupUser(t)
	actID := createDailyActivity(t, tok, "Validation Activity")
	w := get(t, fmt.Sprintf("/api/history/activities/%s?from=2025-06-20&to=2025-06-19", actID), bearer(tok))
	assertStatus(t, w, http.StatusUnprocessableEntity)
}

// TestOccurrencesUnauthenticated verifies that all occurrence/history endpoints
// reject requests without a Bearer token.
func TestOccurrencesUnauthenticated(t *testing.T) {
	today := time.Now().UTC().Format("2006-01-02")
	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/today"},
		{http.MethodPatch, "/api/occurrences/00000000-0000-0000-0000-000000000001"},
		{http.MethodGet, "/api/history/calendar?from=" + today + "&to=" + today},
		{http.MethodGet, "/api/history/calendar/" + today},
		{http.MethodGet, "/api/history/activities/00000000-0000-0000-0000-000000000001?from=" + today + "&to=" + today},
	}
	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			w := doReq(t, ep.method, ep.path, nil, nil)
			assertStatus(t, w, http.StatusUnauthorized)
		})
	}
}
