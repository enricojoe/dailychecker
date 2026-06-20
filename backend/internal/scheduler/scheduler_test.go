package scheduler_test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/enricojoe/dailychecker/internal/activities"
	"github.com/enricojoe/dailychecker/internal/occurrences"
	"github.com/enricojoe/dailychecker/internal/scheduler"
	"github.com/enricojoe/dailychecker/internal/testhelper"
	"github.com/enricojoe/dailychecker/internal/users"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"strings"
)

// ── package-level DB shared across all tests in this package ─────────────────

var testDB *sqlx.DB

func TestMain(m *testing.M) {
	db, skip := testhelper.OpenTestDB()
	if skip {
		fmt.Println("DATABASE_URL not set — skipping scheduler integration tests")
		os.Exit(0)
	}
	testDB = db
	code := m.Run()
	testDB.Close()
	os.Exit(code)
}

// ── mock telegram.Client ──────────────────────────────────────────────────────

// mockTelegramClient records all SendMessage calls; no real network.
type mockTelegramClient struct {
	mu   sync.Mutex
	sent []sentMsg
}

type sentMsg struct {
	chatID int64
	text   string
}

func (m *mockTelegramClient) SendMessage(_ context.Context, chatID int64, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, sentMsg{chatID: chatID, text: text})
	return nil
}

func (m *mockTelegramClient) calls() []sentMsg {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]sentMsg, len(m.sent))
	copy(out, m.sent)
	return out
}

func (m *mockTelegramClient) reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = nil
}

// ── test fixture helpers ──────────────────────────────────────────────────────

var jakartaLoc *time.Location

func init() {
	var err error
	jakartaLoc, err = time.LoadLocation("Asia/Jakarta")
	if err != nil {
		panic("scheduler_test: load Jakarta loc: " + err.Error())
	}
}

// uniqueUsername generates a unique username to avoid conflicts between
// parallel test cases. Each test creates its own user so they don't interfere.
func uniqueUsername(suffix string) string {
	return fmt.Sprintf("sched_%d_%s", time.Now().UnixNano(), suffix)
}

// fixture holds all created objects for a single test scenario.
type fixture struct {
	user     *users.User
	activity *activities.Activity
	occ      *occurrences.Occurrence
	// jakartaDate is the date portion used for the occurrence (midnight Jakarta).
	jakartaDate time.Time
}

// createFixture creates a unique user + top-level activity + occurrence for the
// given date. The activity's time_of_day is set to activityTimeStr (e.g. "08:00:00").
// The user optionally gets a telegram_chat_id when withChatID is true.
//
// Cleanup is registered on t so rows are removed after the test; cascading
// deletes on the user row handle activities and occurrences automatically.
func createFixture(
	t *testing.T,
	ctx context.Context,
	jakartaDate time.Time,
	activityTimeStr string,
	withChatID bool,
	chatID int64,
) *fixture {
	t.Helper()

	userRepo := users.NewRepository(testDB)
	actRepo := activities.NewRepository(testDB)
	occRepo := occurrences.NewRepository(testDB)

	u := &users.User{
		Name:         "SchedTest " + t.Name(),
		Username:     uniqueUsername(fmt.Sprintf("%d", chatID)),
		PasswordHash: "hash",
	}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatalf("createFixture: create user: %v", err)
	}
	if withChatID {
		u.TelegramChatID = &chatID
		if err := userRepo.Update(ctx, u); err != nil {
			t.Fatalf("createFixture: update user telegram_chat_id: %v", err)
		}
	}
	t.Cleanup(func() {
		testDB.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, u.ID)
	})

	act := &activities.Activity{
		UserID:     u.ID,
		Title:      "Test Activity " + t.Name(),
		Freq:       "daily",
		DaysOfWeek: pq.Int64Array{},
		TimeOfDay:  activityTimeStr,
		SortOrder:  0,
		IsActive:   true,
	}
	if err := actRepo.Create(ctx, act); err != nil {
		t.Fatalf("createFixture: create activity: %v", err)
	}

	occ, err := occRepo.Upsert(ctx, act.ID, jakartaDate)
	if err != nil {
		t.Fatalf("createFixture: upsert occurrence: %v", err)
	}

	return &fixture{
		user:        u,
		activity:    act,
		occ:         occ,
		jakartaDate: jakartaDate,
	}
}

// newScheduler constructs a Scheduler for testing with the given mock client and fixed clock.
func newScheduler(mock *mockTelegramClient, clock scheduler.Clock) *scheduler.Scheduler {
	return scheduler.New(
		occurrences.NewRepository(testDB),
		mock,
		jakartaLoc,
		clock,
		22,
		"https://example.com",
	)
}

// todayJakarta returns today midnight in Jakarta (the occurrence date to use).
func todayJakarta() time.Time {
	now := time.Now().In(jakartaLoc)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, jakartaLoc)
}

// ── tests ─────────────────────────────────────────────────────────────────────

// TestReminderFiresExactlyOnce verifies that calling RunReminderTick twice
// with the same clock time sends exactly one Telegram message (dedup via flag).
func TestReminderFiresExactlyOnce(t *testing.T) {
	ctx := context.Background()

	date := todayJakarta()
	// Schedule the activity at 08:00; inject now = 09:00 so it is past due.
	const activityTime = "08:00:00"
	now := time.Date(date.Year(), date.Month(), date.Day(), 9, 0, 0, 0, jakartaLoc)

	var chatID int64 = 111001
	f := createFixture(t, ctx, date, activityTime, true, chatID)

	mock := &mockTelegramClient{}
	sched := newScheduler(mock, func() time.Time { return now })

	// First tick — should send.
	if err := sched.RunReminderTick(ctx, now); err != nil {
		t.Fatalf("RunReminderTick (1st): %v", err)
	}
	if got := len(mock.calls()); got != 1 {
		t.Fatalf("after 1st tick: want 1 send, got %d", got)
	}
	if mock.calls()[0].chatID != chatID {
		t.Errorf("chat id: want %d, got %d", chatID, mock.calls()[0].chatID)
	}
	if !strings.Contains(mock.calls()[0].text, f.activity.Title) {
		t.Errorf("message should contain activity title %q, got %q", f.activity.Title, mock.calls()[0].text)
	}

	// Second tick with same now — should be a no-op.
	if err := sched.RunReminderTick(ctx, now); err != nil {
		t.Fatalf("RunReminderTick (2nd): %v", err)
	}
	if got := len(mock.calls()); got != 1 {
		t.Fatalf("after 2nd tick: want still 1 send (idempotent), got %d", got)
	}
}

// TestReminderRespectsTime verifies that an activity whose time_of_day is in
// the future relative to now does NOT trigger a reminder.
func TestReminderRespectsTime(t *testing.T) {
	ctx := context.Background()

	date := todayJakarta()
	// Activity scheduled at 20:00; inject now = 10:00 so it hasn't fired yet.
	const activityTime = "20:00:00"
	now := time.Date(date.Year(), date.Month(), date.Day(), 10, 0, 0, 0, jakartaLoc)

	var chatID int64 = 111002
	createFixture(t, ctx, date, activityTime, true, chatID)

	mock := &mockTelegramClient{}
	sched := newScheduler(mock, func() time.Time { return now })

	if err := sched.RunReminderTick(ctx, now); err != nil {
		t.Fatalf("RunReminderTick: %v", err)
	}
	if got := len(mock.calls()); got != 0 {
		t.Errorf("expected no send (future time), got %d sends", got)
	}
}

// TestReminderSkipsDoneOccurrence verifies that an occurrence already in state
// 'done' is not reminded.
func TestReminderSkipsDoneOccurrence(t *testing.T) {
	ctx := context.Background()

	date := todayJakarta()
	const activityTime = "08:00:00"
	now := time.Date(date.Year(), date.Month(), date.Day(), 9, 0, 0, 0, jakartaLoc)

	var chatID int64 = 111003
	f := createFixture(t, ctx, date, activityTime, true, chatID)

	// Mark the occurrence done.
	occRepo := occurrences.NewRepository(testDB)
	if _, err := occRepo.UpdateState(ctx, f.occ.ID, occurrences.StateDone); err != nil {
		t.Fatalf("UpdateState done: %v", err)
	}

	mock := &mockTelegramClient{}
	sched := newScheduler(mock, func() time.Time { return now })

	if err := sched.RunReminderTick(ctx, now); err != nil {
		t.Fatalf("RunReminderTick: %v", err)
	}
	if got := len(mock.calls()); got != 0 {
		t.Errorf("expected no send for done occurrence, got %d", got)
	}
}

// TestReminderSkipsUserWithoutChatID verifies that users without a
// telegram_chat_id are excluded from reminders.
func TestReminderSkipsUserWithoutChatID(t *testing.T) {
	ctx := context.Background()

	date := todayJakarta()
	const activityTime = "08:00:00"
	now := time.Date(date.Year(), date.Month(), date.Day(), 9, 0, 0, 0, jakartaLoc)

	// withChatID = false
	createFixture(t, ctx, date, activityTime, false, 0)

	mock := &mockTelegramClient{}
	sched := newScheduler(mock, func() time.Time { return now })

	if err := sched.RunReminderTick(ctx, now); err != nil {
		t.Fatalf("RunReminderTick: %v", err)
	}
	if got := len(mock.calls()); got != 0 {
		t.Errorf("expected no send for user without chat_id, got %d", got)
	}
}

// TestReminderSkipsSubActivities verifies that child activities (parent_id IS
// NOT NULL) are never directly reminded — only top-level activities trigger
// per-activity reminders.
func TestReminderSkipsSubActivities(t *testing.T) {
	ctx := context.Background()

	date := todayJakarta()
	const activityTime = "08:00:00"
	now := time.Date(date.Year(), date.Month(), date.Day(), 9, 0, 0, 0, jakartaLoc)

	var chatID int64 = 111005
	// Create the parent fixture (creates user + parent activity + occurrence).
	f := createFixture(t, ctx, date, activityTime, true, chatID)

	actRepo := activities.NewRepository(testDB)
	occRepo := occurrences.NewRepository(testDB)

	// Create a child activity under the parent.
	parentID := f.activity.ID
	child := &activities.Activity{
		UserID:     f.user.ID,
		ParentID:   &parentID,
		Title:      "Child Activity " + t.Name(),
		Freq:       "daily",
		DaysOfWeek: pq.Int64Array{},
		TimeOfDay:  activityTime,
		SortOrder:  0,
		IsActive:   true,
	}
	if err := actRepo.Create(ctx, child); err != nil {
		t.Fatalf("create child activity: %v", err)
	}
	// Upsert an occurrence for the child.
	if _, err := occRepo.Upsert(ctx, child.ID, date); err != nil {
		t.Fatalf("upsert child occurrence: %v", err)
	}

	mock := &mockTelegramClient{}
	sched := newScheduler(mock, func() time.Time { return now })

	if err := sched.RunReminderTick(ctx, now); err != nil {
		t.Fatalf("RunReminderTick: %v", err)
	}

	calls := mock.calls()
	// Only the parent occurrence should trigger a send (exactly one).
	if len(calls) != 1 {
		t.Fatalf("expected exactly 1 reminder (parent only), got %d", len(calls))
	}
	if calls[0].chatID != chatID {
		t.Errorf("chat id: want %d, got %d", chatID, calls[0].chatID)
	}
}

// TestDigestOneMessagePerUser verifies that a single digest message is sent
// per user summarising all not-done items for today.
func TestDigestOneMessagePerUser(t *testing.T) {
	ctx := context.Background()

	date := todayJakarta()
	const activityTime = "08:00:00"
	now := time.Date(date.Year(), date.Month(), date.Day(), 22, 0, 0, 0, jakartaLoc)

	var chatID int64 = 222001
	f := createFixture(t, ctx, date, activityTime, true, chatID)

	mock := &mockTelegramClient{}
	sched := newScheduler(mock, func() time.Time { return now })

	if err := sched.RunDigest(ctx, now); err != nil {
		t.Fatalf("RunDigest: %v", err)
	}

	calls := mock.calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 digest message, got %d", len(calls))
	}
	msg := calls[0].text
	if calls[0].chatID != chatID {
		t.Errorf("chat id: want %d, got %d", chatID, calls[0].chatID)
	}
	if !strings.Contains(msg, f.activity.Title) {
		t.Errorf("digest message should contain activity title %q, got:\n%s", f.activity.Title, msg)
	}
	if !strings.Contains(msg, "https://example.com") {
		t.Errorf("digest message should contain AppPublicURL, got:\n%s", msg)
	}
}

// TestDigestIdempotent verifies that calling RunDigest twice sends only one
// message (the second call is a no-op once digest_notified_at is set).
func TestDigestIdempotent(t *testing.T) {
	ctx := context.Background()

	date := todayJakarta()
	const activityTime = "08:00:00"
	now := time.Date(date.Year(), date.Month(), date.Day(), 22, 0, 0, 0, jakartaLoc)

	var chatID int64 = 222002
	createFixture(t, ctx, date, activityTime, true, chatID)

	mock := &mockTelegramClient{}
	sched := newScheduler(mock, func() time.Time { return now })

	if err := sched.RunDigest(ctx, now); err != nil {
		t.Fatalf("RunDigest (1st): %v", err)
	}
	if got := len(mock.calls()); got != 1 {
		t.Fatalf("after 1st digest: want 1 send, got %d", got)
	}

	if err := sched.RunDigest(ctx, now); err != nil {
		t.Fatalf("RunDigest (2nd): %v", err)
	}
	if got := len(mock.calls()); got != 1 {
		t.Fatalf("after 2nd digest: want still 1 send (idempotent), got %d", got)
	}
}

// TestDigestSkipsDoneOccurrence verifies that occurrences already in state
// 'done' are excluded from the digest.
func TestDigestSkipsDoneOccurrence(t *testing.T) {
	ctx := context.Background()

	date := todayJakarta()
	const activityTime = "08:00:00"
	now := time.Date(date.Year(), date.Month(), date.Day(), 22, 0, 0, 0, jakartaLoc)

	var chatID int64 = 222003
	f := createFixture(t, ctx, date, activityTime, true, chatID)

	occRepo := occurrences.NewRepository(testDB)
	if _, err := occRepo.UpdateState(ctx, f.occ.ID, occurrences.StateDone); err != nil {
		t.Fatalf("UpdateState done: %v", err)
	}

	mock := &mockTelegramClient{}
	sched := newScheduler(mock, func() time.Time { return now })

	if err := sched.RunDigest(ctx, now); err != nil {
		t.Fatalf("RunDigest: %v", err)
	}
	if got := len(mock.calls()); got != 0 {
		t.Errorf("expected no digest for all-done user, got %d sends", got)
	}
}

// TestDigestSkipsUserWithoutChatID verifies that users without a
// telegram_chat_id receive no digest.
func TestDigestSkipsUserWithoutChatID(t *testing.T) {
	ctx := context.Background()

	date := todayJakarta()
	const activityTime = "08:00:00"
	now := time.Date(date.Year(), date.Month(), date.Day(), 22, 0, 0, 0, jakartaLoc)

	createFixture(t, ctx, date, activityTime, false, 0)

	mock := &mockTelegramClient{}
	sched := newScheduler(mock, func() time.Time { return now })

	if err := sched.RunDigest(ctx, now); err != nil {
		t.Fatalf("RunDigest: %v", err)
	}
	if got := len(mock.calls()); got != 0 {
		t.Errorf("expected no digest for user without chat_id, got %d", got)
	}
}
