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

	// Unique phone per test run to avoid cross-run conflicts.
	phone := fmt.Sprintf("+1555%09d", time.Now().UnixNano()%1_000_000_000)

	u := &users.User{
		Name:         "Alice",
		Phone:        phone,
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

	t.Run("DuplicatePhone", func(t *testing.T) {
		dup := &users.User{Name: "Bob", Phone: phone, PasswordHash: "x"}
		err := repo.Create(ctx, dup)
		if err != users.ErrConflict {
			t.Fatalf("duplicate phone: want ErrConflict, got %v", err)
		}
	})

	t.Run("GetByID", func(t *testing.T) {
		got, err := repo.GetByID(ctx, u.ID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if got.Phone != phone {
			t.Errorf("phone: want %q, got %q", phone, got.Phone)
		}
	})

	t.Run("GetByIDNotFound", func(t *testing.T) {
		_, err := repo.GetByID(ctx, "00000000-0000-0000-0000-000000000000")
		if err != users.ErrNotFound {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})

	t.Run("GetByPhone", func(t *testing.T) {
		got, err := repo.GetByPhone(ctx, phone)
		if err != nil {
			t.Fatalf("GetByPhone: %v", err)
		}
		if got.ID != u.ID {
			t.Errorf("ID: want %q, got %q", u.ID, got.ID)
		}
	})

	t.Run("GetByPhoneNotFound", func(t *testing.T) {
		_, err := repo.GetByPhone(ctx, "+10000000000")
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
