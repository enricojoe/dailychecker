package occurrences_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/enricojoe/dailychecker/internal/activities"
	"github.com/enricojoe/dailychecker/internal/occurrences"
	"github.com/enricojoe/dailychecker/internal/testhelper"
	"github.com/enricojoe/dailychecker/internal/users"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

var testDB *sqlx.DB

func TestMain(m *testing.M) {
	db, skip := testhelper.OpenTestDB()
	if skip {
		fmt.Println("DATABASE_URL not set — skipping occurrences integration tests")
		os.Exit(0)
	}
	testDB = db
	code := m.Run()
	testDB.Close()
	os.Exit(code)
}

func TestOccurrenceRepository(t *testing.T) {
	occRepo := occurrences.NewRepository(testDB)
	actRepo := activities.NewRepository(testDB)
	userRepo := users.NewRepository(testDB)
	ctx := context.Background()

	// --- Shared fixtures: user + activity ---
	owner := &users.User{Name: "Occ Owner", Username: fmt.Sprintf("occowner_%d", time.Now().UnixNano()), PasswordHash: "hash"}
	if err := userRepo.Create(ctx, owner); err != nil {
		t.Fatalf("setup create user: %v", err)
	}
	t.Cleanup(func() {
		testDB.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, owner.ID)
	})

	act := &activities.Activity{
		UserID:     owner.ID,
		Title:      "Daily push-ups",
		Freq:       "daily",
		DaysOfWeek: pq.Int64Array{},
		TimeOfDay:  "06:30:00",
		SortOrder:  0,
		IsActive:   true,
	}
	if err := actRepo.Create(ctx, act); err != nil {
		t.Fatalf("setup create activity: %v", err)
	}

	// Use today as Jakarta date (v1 assumption: single timezone handled by service).
	today := time.Now().UTC().Truncate(24 * time.Hour)

	// --- Upsert (insert path) ---
	o, err := occRepo.Upsert(ctx, act.ID, today)
	if err != nil {
		t.Fatalf("Upsert (insert): %v", err)
	}
	if o.ID == "" {
		t.Fatal("Upsert: ID not set")
	}
	if o.State != occurrences.StatePending {
		t.Errorf("initial state: want %q, got %q", occurrences.StatePending, o.State)
	}

	t.Run("Upsert_Idempotent", func(t *testing.T) {
		o2, err := occRepo.Upsert(ctx, act.ID, today)
		if err != nil {
			t.Fatalf("Upsert (idempotent): %v", err)
		}
		if o2.ID != o.ID {
			t.Errorf("idempotent upsert returned different ID: first=%q second=%q", o.ID, o2.ID)
		}
		if o2.State != o.State {
			t.Errorf("idempotent upsert changed state: was %q, now %q", o.State, o2.State)
		}
	})

	t.Run("GetByID", func(t *testing.T) {
		got, err := occRepo.GetByID(ctx, o.ID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if got.ActivityID != act.ID {
			t.Errorf("ActivityID: want %q, got %q", act.ID, got.ActivityID)
		}
	})

	t.Run("GetByIDNotFound", func(t *testing.T) {
		_, err := occRepo.GetByID(ctx, "00000000-0000-0000-0000-000000000000")
		if !errors.Is(err, occurrences.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})

	t.Run("UpdateState_Done", func(t *testing.T) {
		updated, err := occRepo.UpdateState(ctx, o.ID, occurrences.StateDone)
		if err != nil {
			t.Fatalf("UpdateState done: %v", err)
		}
		if updated.State != occurrences.StateDone {
			t.Errorf("state: want %q, got %q", occurrences.StateDone, updated.State)
		}
		if updated.CompletedAt == nil {
			t.Error("CompletedAt should be set when transitioning to done")
		}
	})

	t.Run("UpdateState_Pending_ClearsCompletedAt", func(t *testing.T) {
		updated, err := occRepo.UpdateState(ctx, o.ID, occurrences.StatePending)
		if err != nil {
			t.Fatalf("UpdateState pending: %v", err)
		}
		if updated.State != occurrences.StatePending {
			t.Errorf("state: want %q, got %q", occurrences.StatePending, updated.State)
		}
		if updated.CompletedAt != nil {
			t.Errorf("CompletedAt should be nil when state=pending, got %v", updated.CompletedAt)
		}
	})

	t.Run("UpdateState_Partial", func(t *testing.T) {
		updated, err := occRepo.UpdateState(ctx, o.ID, occurrences.StatePartial)
		if err != nil {
			t.Fatalf("UpdateState partial: %v", err)
		}
		if updated.State != occurrences.StatePartial {
			t.Errorf("state: want %q, got %q", occurrences.StatePartial, updated.State)
		}
		if updated.CompletedAt != nil {
			t.Errorf("CompletedAt should be nil when state=partial, got %v", updated.CompletedAt)
		}
	})

	t.Run("UpdateStateNotFound", func(t *testing.T) {
		_, err := occRepo.UpdateState(ctx, "00000000-0000-0000-0000-000000000000", occurrences.StateDone)
		if !errors.Is(err, occurrences.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})

	t.Run("ListByActivityAndDateRange", func(t *testing.T) {
		yesterday := today.AddDate(0, 0, -1)
		tomorrow := today.AddDate(0, 0, 1)

		list, err := occRepo.ListByActivityAndDateRange(ctx, act.ID, yesterday, tomorrow)
		if err != nil {
			t.Fatalf("ListByActivityAndDateRange: %v", err)
		}
		if len(list) == 0 {
			t.Fatal("expected at least one occurrence in the range")
		}
		// Verify ascending date order.
		for i := 1; i < len(list); i++ {
			if list[i].OccurDate.Before(list[i-1].OccurDate) {
				t.Errorf("results not ordered by occur_date at index %d", i)
			}
		}
	})

	t.Run("ListByUserAndDate", func(t *testing.T) {
		list, err := occRepo.ListByUserAndDate(ctx, owner.ID, today)
		if err != nil {
			t.Fatalf("ListByUserAndDate: %v", err)
		}
		if len(list) == 0 {
			t.Fatal("expected at least one occurrence for this user today")
		}
		found := false
		for _, occ := range list {
			if occ.ActivityID == act.ID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("activity %q not found in ListByUserAndDate result", act.ID)
		}
	})
}
