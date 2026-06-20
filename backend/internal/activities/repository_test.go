package activities_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/enricojoe/dailychecker/internal/activities"
	"github.com/enricojoe/dailychecker/internal/testhelper"
	"github.com/enricojoe/dailychecker/internal/users"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

var testDB *sqlx.DB

func TestMain(m *testing.M) {
	db, skip := testhelper.OpenTestDB()
	if skip {
		fmt.Println("DATABASE_URL not set — skipping activities integration tests")
		os.Exit(0)
	}
	testDB = db
	code := m.Run()
	testDB.Close()
	os.Exit(code)
}

func TestActivityRepository(t *testing.T) {
	repo := activities.NewRepository(testDB)
	userRepo := users.NewRepository(testDB)
	ctx := context.Background()

	phone := fmt.Sprintf("+1555%09d", time.Now().UnixNano()%1_000_000_000)
	owner := &users.User{Name: "Activity Owner", Phone: phone, PasswordHash: "hash"}
	if err := userRepo.Create(ctx, owner); err != nil {
		t.Fatalf("setup create user: %v", err)
	}
	t.Cleanup(func() {
		testDB.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, owner.ID)
	})

	// --- Create a top-level daily activity ---
	parent := &activities.Activity{
		UserID:     owner.ID,
		Title:      "Morning run",
		Freq:       "daily",
		DaysOfWeek: pq.Int64Array{},
		TimeOfDay:  "07:00:00",
		SortOrder:  1,
		IsActive:   true,
	}
	if err := repo.Create(ctx, parent); err != nil {
		t.Fatalf("Create parent: %v", err)
	}
	if parent.ID == "" {
		t.Fatal("Create parent: ID not populated")
	}

	t.Run("GetByID", func(t *testing.T) {
		got, err := repo.GetByID(ctx, parent.ID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if got.Title != "Morning run" {
			t.Errorf("Title: want %q, got %q", "Morning run", got.Title)
		}
		if got.TimeOfDay != "07:00:00" {
			t.Errorf("TimeOfDay: want %q, got %q", "07:00:00", got.TimeOfDay)
		}
		if got.ParentID != nil {
			t.Errorf("ParentID should be nil for top-level activity, got %v", got.ParentID)
		}
	})

	t.Run("GetByIDNotFound", func(t *testing.T) {
		_, err := repo.GetByID(ctx, "00000000-0000-0000-0000-000000000000")
		if !errors.Is(err, activities.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})

	// --- Create a child (weekly) activity ---
	notes := "stretch 5 min first"
	child := &activities.Activity{
		UserID:     owner.ID,
		ParentID:   &parent.ID,
		Title:      "Stretch",
		Notes:      &notes,
		Freq:       "weekly",
		DaysOfWeek: pq.Int64Array{1, 3, 5}, // Mon, Wed, Fri
		TimeOfDay:  "07:05:00",
		SortOrder:  0,
		IsActive:   true,
	}
	if err := repo.Create(ctx, child); err != nil {
		t.Fatalf("Create child: %v", err)
	}
	if child.ParentID == nil || *child.ParentID != parent.ID {
		t.Fatalf("child ParentID mismatch: want %q, got %v", parent.ID, child.ParentID)
	}

	t.Run("ListByUser_Order", func(t *testing.T) {
		list, err := repo.ListByUser(ctx, owner.ID)
		if err != nil {
			t.Fatalf("ListByUser: %v", err)
		}
		if len(list) < 2 {
			t.Fatalf("want >=2 items, got %d", len(list))
		}
		// Top-level activities (parent_id IS NULL) must precede children.
		if list[0].ParentID != nil {
			t.Errorf("first item should be top-level; got ParentID=%v", list[0].ParentID)
		}
	})

	t.Run("Update", func(t *testing.T) {
		parent.Title = "Evening run"
		parent.IsActive = false
		if err := repo.Update(ctx, parent); err != nil {
			t.Fatalf("Update: %v", err)
		}
		got, err := repo.GetByID(ctx, parent.ID)
		if err != nil {
			t.Fatalf("GetByID after update: %v", err)
		}
		if got.Title != "Evening run" {
			t.Errorf("Title after update: want %q, got %q", "Evening run", got.Title)
		}
		if got.IsActive {
			t.Error("IsActive should be false after update")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		// Create a disposable activity so we can delete it without disturbing other sub-tests.
		d := &activities.Activity{
			UserID:     owner.ID,
			Title:      "To delete",
			Freq:       "daily",
			DaysOfWeek: pq.Int64Array{},
			TimeOfDay:  "08:00:00",
			IsActive:   true,
		}
		if err := repo.Create(ctx, d); err != nil {
			t.Fatalf("Create for delete: %v", err)
		}
		if err := repo.Delete(ctx, d.ID); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		_, err := repo.GetByID(ctx, d.ID)
		if !errors.Is(err, activities.ErrNotFound) {
			t.Fatalf("after delete: want ErrNotFound, got %v", err)
		}
	})

	t.Run("DeleteNotFound", func(t *testing.T) {
		err := repo.Delete(ctx, "00000000-0000-0000-0000-000000000000")
		if !errors.Is(err, activities.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})

	t.Run("DeleteCascadesToChild", func(t *testing.T) {
		// Deleting the parent should cascade-delete the child via ON DELETE CASCADE.
		if err := repo.Delete(ctx, parent.ID); err != nil {
			t.Fatalf("Delete parent: %v", err)
		}
		_, err := repo.GetByID(ctx, child.ID)
		if !errors.Is(err, activities.ErrNotFound) {
			t.Fatalf("child should be gone after parent delete; got %v", err)
		}
	})
}
