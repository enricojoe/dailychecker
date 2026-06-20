package occurrences_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/enricojoe/dailychecker/internal/activities"
	"github.com/enricojoe/dailychecker/internal/occurrences"
	"github.com/enricojoe/dailychecker/internal/users"
	"github.com/lib/pq"
)

// TestGenerateForDateWeeklyDueLogic verifies that a weekly activity produces an
// occurrence only on its scheduled weekday — closing the M4 DoD requirement that
// "a weekly activity only generates on its days". GenerateForDate takes an
// explicit date, so the scheduled-day and wrong-day cases are tested directly
// without manipulating the wall clock.
//
// Calendar anchors (UTC midnight, matching how the service stores DATE values):
//
//	2026-06-22 is a Monday (weekday 1)
//	2026-06-23 is a Tuesday (weekday 2)
func TestGenerateForDateWeeklyDueLogic(t *testing.T) {
	occRepo := occurrences.NewRepository(testDB)
	actRepo := activities.NewRepository(testDB)
	userRepo := users.NewRepository(testDB)
	svc := occurrences.NewService(occRepo, actRepo, time.UTC)
	ctx := context.Background()

	phone := fmt.Sprintf("+1556%09d", time.Now().UnixNano()%1_000_000_000)
	owner := &users.User{Name: "Weekly Owner", Phone: phone, PasswordHash: "hash"}
	if err := userRepo.Create(ctx, owner); err != nil {
		t.Fatalf("setup create user: %v", err)
	}
	t.Cleanup(func() {
		testDB.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, owner.ID)
	})

	// Weekly activity scheduled only on Monday (1).
	act := &activities.Activity{
		UserID:     owner.ID,
		Title:      "Monday-only review",
		Freq:       "weekly",
		DaysOfWeek: pq.Int64Array{1},
		TimeOfDay:  "09:00:00",
		IsActive:   true,
	}
	if err := actRepo.Create(ctx, act); err != nil {
		t.Fatalf("setup create activity: %v", err)
	}

	monday := time.Date(2026, 6, 22, 0, 0, 0, 0, time.UTC)
	tuesday := time.Date(2026, 6, 23, 0, 0, 0, 0, time.UTC)

	if int(monday.Weekday()) != 1 || int(tuesday.Weekday()) != 2 {
		t.Fatalf("calendar anchor wrong: monday wd=%d tuesday wd=%d", monday.Weekday(), tuesday.Weekday())
	}

	countOn := func(date time.Time) int {
		t.Helper()
		occs, err := occRepo.ListByActivityAndDateRange(ctx, act.ID, date, date)
		if err != nil {
			t.Fatalf("list occurrences for %s: %v", date.Format("2006-01-02"), err)
		}
		return len(occs)
	}

	// Scheduled day (Monday) → exactly one occurrence.
	if err := svc.GenerateForDate(ctx, owner.ID, monday); err != nil {
		t.Fatalf("generate monday: %v", err)
	}
	if got := countOn(monday); got != 1 {
		t.Fatalf("weekly activity on scheduled day: want 1 occurrence, got %d", got)
	}

	// Wrong day (Tuesday) → no occurrence.
	if err := svc.GenerateForDate(ctx, owner.ID, tuesday); err != nil {
		t.Fatalf("generate tuesday: %v", err)
	}
	if got := countOn(tuesday); got != 0 {
		t.Fatalf("weekly activity on unscheduled day: want 0 occurrences, got %d", got)
	}
}
