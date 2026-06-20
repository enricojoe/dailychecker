package users_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/enricojoe/dailychecker/internal/testhelper"
	"github.com/enricojoe/dailychecker/internal/users"
	"github.com/jmoiron/sqlx"
)

var testDB *sqlx.DB

func TestMain(m *testing.M) {
	db, skip := testhelper.OpenTestDB()
	if skip {
		fmt.Println("DATABASE_URL not set — skipping users integration tests")
		os.Exit(0)
	}
	testDB = db
	code := m.Run()
	testDB.Close()
	os.Exit(code)
}

func TestUserRepository(t *testing.T) {
	repo := users.NewRepository(testDB)
	ctx := context.Background()

	// Unique username per test run to avoid cross-run conflicts.
	username := fmt.Sprintf("alice_%d", time.Now().UnixNano())

	u := &users.User{
		Name:         "Alice",
		Username:     username,
		PasswordHash: "$2a$12$placeholder",
	}
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if u.ID == "" {
		t.Fatal("Create: ID not populated from DB")
	}
	if u.CreatedAt.IsZero() {
		t.Fatal("Create: CreatedAt not populated from DB")
	}

	// All sub-tests share this user; cleanup cascades via ON DELETE CASCADE.
	t.Cleanup(func() {
		testDB.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, u.ID)
	})

	t.Run("DuplicateUsername", func(t *testing.T) {
		dup := &users.User{Name: "Bob", Username: username, PasswordHash: "x"}
		err := repo.Create(ctx, dup)
		if err != users.ErrConflict {
			t.Fatalf("duplicate username: want ErrConflict, got %v", err)
		}
	})

	t.Run("GetByID", func(t *testing.T) {
		got, err := repo.GetByID(ctx, u.ID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if got.Username != username {
			t.Errorf("username: want %q, got %q", username, got.Username)
		}
	})

	t.Run("GetByIDNotFound", func(t *testing.T) {
		_, err := repo.GetByID(ctx, "00000000-0000-0000-0000-000000000000")
		if err != users.ErrNotFound {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})

	t.Run("GetByUsername", func(t *testing.T) {
		got, err := repo.GetByUsername(ctx, username)
		if err != nil {
			t.Fatalf("GetByUsername: %v", err)
		}
		if got.ID != u.ID {
			t.Errorf("ID: want %q, got %q", u.ID, got.ID)
		}
	})

	t.Run("GetByUsernameNotFound", func(t *testing.T) {
		_, err := repo.GetByUsername(ctx, "nonexistent_user_xyz")
		if err != users.ErrNotFound {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})

	t.Run("Update", func(t *testing.T) {
		chatID := int64(987654321)
		token := "tg-link-token"
		linkedAt := time.Now().UTC().Truncate(time.Second)

		u.Name = "Alice Updated"
		u.TelegramChatID = &chatID
		u.TelegramLinkToken = &token
		u.TelegramLinkedAt = &linkedAt

		if err := repo.Update(ctx, u); err != nil {
			t.Fatalf("Update: %v", err)
		}
		if u.UpdatedAt.IsZero() {
			t.Fatal("Update: UpdatedAt not refreshed")
		}

		got, err := repo.GetByID(ctx, u.ID)
		if err != nil {
			t.Fatalf("GetByID after update: %v", err)
		}
		if got.Name != "Alice Updated" {
			t.Errorf("name: want %q, got %q", "Alice Updated", got.Name)
		}
		if got.TelegramChatID == nil || *got.TelegramChatID != chatID {
			t.Errorf("telegram_chat_id: want %d, got %v", chatID, got.TelegramChatID)
		}
		if got.TelegramLinkToken == nil || *got.TelegramLinkToken != token {
			t.Errorf("telegram_link_token: want %q, got %v", token, got.TelegramLinkToken)
		}
	})
}
