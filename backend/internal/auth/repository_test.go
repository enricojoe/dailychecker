package auth_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/enricojoe/dailychecker/internal/auth"
	"github.com/enricojoe/dailychecker/internal/testhelper"
	"github.com/enricojoe/dailychecker/internal/users"
	"github.com/jmoiron/sqlx"
)

var testDB *sqlx.DB

func TestMain(m *testing.M) {
	db, skip := testhelper.OpenTestDB()
	if skip {
		fmt.Println("DATABASE_URL not set — skipping auth integration tests")
		os.Exit(0)
	}
	testDB = db
	code := m.Run()
	testDB.Close()
	os.Exit(code)
}

func TestTokenRepository(t *testing.T) {
	tokenRepo := auth.NewTokenRepository(testDB)
	userRepo := users.NewRepository(testDB)
	ctx := context.Background()

	// Create a throw-away user to satisfy the refresh_tokens.user_id FK.
	owner := &users.User{Name: "Token Owner", Username: fmt.Sprintf("tokenowner_%d", time.Now().UnixNano()), PasswordHash: "hash"}
	if err := userRepo.Create(ctx, owner); err != nil {
		t.Fatalf("setup create user: %v", err)
	}
	t.Cleanup(func() {
		testDB.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, owner.ID)
	})

	tok := &auth.RefreshToken{
		UserID:    owner.ID,
		TokenHash: fmt.Sprintf("hash-%d", time.Now().UnixNano()),
		ExpiresAt: time.Now().Add(24 * time.Hour).UTC(),
	}

	t.Run("Create", func(t *testing.T) {
		if err := tokenRepo.Create(ctx, tok); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if tok.ID == "" {
			t.Fatal("Create: ID not populated")
		}
		if tok.CreatedAt.IsZero() {
			t.Fatal("Create: CreatedAt not populated")
		}
	})

	t.Run("GetByHash", func(t *testing.T) {
		got, err := tokenRepo.GetByHash(ctx, tok.TokenHash)
		if err != nil {
			t.Fatalf("GetByHash: %v", err)
		}
		if got.ID != tok.ID {
			t.Errorf("ID: want %q, got %q", tok.ID, got.ID)
		}
		if got.UserID != owner.ID {
			t.Errorf("UserID: want %q, got %q", owner.ID, got.UserID)
		}
		if got.RevokedAt != nil {
			t.Errorf("RevokedAt should be nil on fresh token, got %v", got.RevokedAt)
		}
		if !got.IsValid() {
			t.Error("fresh token should be valid")
		}
	})

	t.Run("GetByHashNotFound", func(t *testing.T) {
		_, err := tokenRepo.GetByHash(ctx, "nonexistent-hash")
		if !errors.Is(err, auth.ErrTokenNotFound) {
			t.Fatalf("want ErrTokenNotFound, got %v", err)
		}
	})

	t.Run("Revoke", func(t *testing.T) {
		if err := tokenRepo.Revoke(ctx, tok.ID); err != nil {
			t.Fatalf("Revoke: %v", err)
		}
		got, err := tokenRepo.GetByHash(ctx, tok.TokenHash)
		if err != nil {
			t.Fatalf("GetByHash after revoke: %v", err)
		}
		if got.RevokedAt == nil {
			t.Error("RevokedAt should be set after Revoke")
		}
		if got.IsValid() {
			t.Error("revoked token should not be valid")
		}
	})

	t.Run("RevokeNotFound", func(t *testing.T) {
		err := tokenRepo.Revoke(ctx, "00000000-0000-0000-0000-000000000000")
		if !errors.Is(err, auth.ErrTokenNotFound) {
			t.Fatalf("want ErrTokenNotFound, got %v", err)
		}
	})
}
