package db

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/joho/godotenv"
)

// migrationsDir resolves backend/migrations/ relative to this source file.
func migrationsDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "migrations"
	}
	// This file: backend/internal/db/db_test.go
	// backend/migrations/ is two directories up.
	return filepath.Join(filepath.Dir(file), "..", "..", "migrations")
}

// testDatabaseURL loads DATABASE_URL from the environment (with .env fallback).
func testDatabaseURL() string {
	_, file, _, ok := runtime.Caller(0)
	if ok {
		envFile := filepath.Join(filepath.Dir(file), "..", "..", ".env")
		_ = godotenv.Load(envFile)
	}
	return os.Getenv("DATABASE_URL")
}

// TestMigrationsUpDown verifies that all migrations apply and roll back cleanly.
func TestMigrationsUpDown(t *testing.T) {
	url := testDatabaseURL()
	if url == "" {
		t.Skip("DATABASE_URL not set — skipping migration integration test")
	}

	mdir := migrationsDir()

	// Ensure a clean slate: roll back whatever might be applied.
	if err := RunMigrationsDown(url, mdir); err != nil {
		t.Fatalf("initial down (clean slate): %v", err)
	}

	// --- UP ---
	if err := RunMigrations(url, mdir); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	// Open a separate connection to query schema info.
	conn, err := Connect(url)
	if err != nil {
		t.Fatalf("connect for verification: %v", err)
	}
	defer conn.Close()

	// Spot-check: all four tables must exist.
	tables := []string{"users", "refresh_tokens", "activities", "occurrences"}
	for _, tbl := range tables {
		var exists bool
		row := conn.QueryRow(
			`SELECT EXISTS (
				SELECT 1 FROM information_schema.tables
				WHERE table_schema = 'public' AND table_name = $1
			)`, tbl)
		if err := row.Scan(&exists); err != nil {
			t.Fatalf("check table %q: %v", tbl, err)
		}
		if !exists {
			t.Errorf("table %q not found after migrate up", tbl)
		}
	}

	// Spot-check required indexes.
	indexes := []struct{ table, index string }{
		{"activities", "idx_activities_user_id_is_active"},
		{"occurrences", "occurrences_activity_date_unique"},
		{"occurrences", "idx_occurrences_occur_date_state"},
	}
	for _, idx := range indexes {
		var exists bool
		row := conn.QueryRow(
			`SELECT EXISTS (
				SELECT 1 FROM pg_indexes
				WHERE schemaname = 'public'
				  AND tablename  = $1
				  AND indexname  = $2
			)`, idx.table, idx.index)
		if err := row.Scan(&exists); err != nil {
			t.Fatalf("check index %q on %q: %v", idx.index, idx.table, err)
		}
		if !exists {
			t.Errorf("index %q on table %q not found after migrate up", idx.index, idx.table)
		}
	}

	// --- DOWN ---
	if err := RunMigrationsDown(url, mdir); err != nil {
		t.Fatalf("migrate down: %v", err)
	}

	// All four tables must be gone.
	for _, tbl := range tables {
		var exists bool
		row := conn.QueryRow(
			`SELECT EXISTS (
				SELECT 1 FROM information_schema.tables
				WHERE table_schema = 'public' AND table_name = $1
			)`, tbl)
		if err := row.Scan(&exists); err != nil {
			t.Fatalf("post-down check table %q: %v", tbl, err)
		}
		if exists {
			t.Errorf("table %q still exists after migrate down", tbl)
		}
	}

	// --- UP again (leave DB in migrated state for other test packages) ---
	if err := RunMigrations(url, mdir); err != nil {
		t.Fatalf("migrate up (second pass): %v", err)
	}

	fmt.Println("migrate up→down→up cycle: OK")
}
