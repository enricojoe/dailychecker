package auth_test

import (
	"testing"
	"time"

	"github.com/enricojoe/dailychecker/internal/auth"
)

func TestHashPassword(t *testing.T) {
	const password = "hunter2-correct-horse"
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "" {
		t.Fatal("HashPassword: returned empty hash")
	}
	if hash == password {
		t.Fatal("HashPassword: hash equals plaintext")
	}
}

func TestCheckPassword(t *testing.T) {
	const password = "correct-horse-battery-staple"
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword setup: %v", err)
	}

	t.Run("correct password", func(t *testing.T) {
		if err := auth.CheckPassword(hash, password); err != nil {
			t.Errorf("CheckPassword correct: got unexpected error: %v", err)
		}
	})
	t.Run("wrong password", func(t *testing.T) {
		if err := auth.CheckPassword(hash, "wrong-password"); err == nil {
			t.Error("CheckPassword wrong: expected error, got nil")
		}
	})
	t.Run("empty password", func(t *testing.T) {
		if err := auth.CheckPassword(hash, ""); err == nil {
			t.Error("CheckPassword empty: expected error, got nil")
		}
	})
}

func TestIssueAndParseAccessToken(t *testing.T) {
	const (
		userID = "11111111-1111-1111-1111-111111111111"
		secret = "unit-test-secret"
	)

	t.Run("happy path", func(t *testing.T) {
		tok, err := auth.IssueAccessToken(userID, secret, time.Minute)
		if err != nil {
			t.Fatalf("IssueAccessToken: %v", err)
		}
		if tok == "" {
			t.Fatal("IssueAccessToken: returned empty string")
		}
		got, err := auth.ParseAccessToken(tok, secret)
		if err != nil {
			t.Fatalf("ParseAccessToken: %v", err)
		}
		if got != userID {
			t.Errorf("ParseAccessToken subject: want %q, got %q", userID, got)
		}
	})

	t.Run("expired token", func(t *testing.T) {
		// Issue a token that is already expired by 1 minute.
		tok, err := auth.IssueAccessToken(userID, secret, -time.Minute)
		if err != nil {
			t.Fatalf("IssueAccessToken: %v", err)
		}
		_, err = auth.ParseAccessToken(tok, secret)
		if err == nil {
			t.Fatal("ParseAccessToken: expected error for expired token, got nil")
		}
	})

	t.Run("wrong secret", func(t *testing.T) {
		tok, err := auth.IssueAccessToken(userID, secret, time.Minute)
		if err != nil {
			t.Fatalf("IssueAccessToken: %v", err)
		}
		_, err = auth.ParseAccessToken(tok, "a-different-secret")
		if err == nil {
			t.Fatal("ParseAccessToken: expected error for wrong secret, got nil")
		}
	})

	t.Run("malformed token", func(t *testing.T) {
		_, err := auth.ParseAccessToken("this.is.not.a.valid.jwt", secret)
		if err == nil {
			t.Fatal("ParseAccessToken: expected error for malformed token, got nil")
		}
	})

	t.Run("empty string", func(t *testing.T) {
		_, err := auth.ParseAccessToken("", secret)
		if err == nil {
			t.Fatal("ParseAccessToken: expected error for empty string, got nil")
		}
	})
}

func TestGenerateRefreshToken(t *testing.T) {
	raw1, hash1, err := auth.GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	if raw1 == "" {
		t.Fatal("GenerateRefreshToken: raw is empty")
	}
	if hash1 == "" {
		t.Fatal("GenerateRefreshToken: hash is empty")
	}
	if raw1 == hash1 {
		t.Fatal("GenerateRefreshToken: raw and hash must differ")
	}

	// Two calls must produce different tokens.
	raw2, hash2, err := auth.GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken second call: %v", err)
	}
	if raw1 == raw2 {
		t.Fatal("GenerateRefreshToken: two calls returned identical raw tokens")
	}
	if hash1 == hash2 {
		t.Fatal("GenerateRefreshToken: two calls returned identical hashes")
	}
}
