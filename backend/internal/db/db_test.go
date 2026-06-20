package db

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

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

// adminDSN derives a connection string that targets the "postgres" maintenance
// database on the same server as dsn. This is required because CREATE DATABASE
// and DROP DATABASE cannot target the database being managed.
func adminDSN(dsn string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("parse DSN: %w", err)
	}
	u.Path = "/postgres"
	return u.String(), nil
}

// createIsolatedDB creates a fresh throwaway database on the same Postgres
// server as appDSN, then returns both the DSN for that database and a cleanup
// function that drops it. The cleanup function should be deferred by the
// caller immediately after a successful return.
func createIsolatedDB(t *testing.T, appDSN string) (string, func()) {
	t.Helper()

	// Unique name: test_migrations_<unix-millis>_<pid>
	dbName := fmt.Sprintf("test_migrations_%d_%d", time.Now().UnixMilli(), os.Getpid())

	adminURL, err := adminDSN(appDSN)
	if err != nil {
		t.Fatalf("createIsolatedDB: admin DSN: %v", err)
	}

	// CREATE / DROP DATABASE cannot run inside a transaction and need a direct
	// *sql.DB connection to the maintenance database.
	admin, err := sql.Open("postgres", adminURL)
	if err != nil {
		t.Fatalf("createIsolatedDB: open admin conn: %v", err)
	}
	if err := admin.Ping(); err != nil {
		admin.Close()
		t.Fatalf("createIsolatedDB: ping admin: %v", err)
	}

	if _, err := admin.Exec(`CREATE DATABASE "` + dbName + `"`); err != nil {
		admin.Close()
		t.Fatalf("createIsolatedDB: CREATE DATABASE %q: %v", dbName, err)
	}

	// Derive the isolated DSN by swapping the database path.
	u, _ := url.Parse(appDSN)
	u.Path = "/" + dbName
	isolatedDSN := u.String()

	cleanup := func() {
		// Terminate any remaining connections to the throwaway database so that
		// DROP DATABASE does not fail with "other sessions are using the database".
		_, _ = admin.Exec(
			`SELECT pg_terminate_backend(pid)
			 FROM pg_stat_activity
			 WHERE datname = $1 AND pid <> pg_backend_pid()`, dbName,
		)
		if _, err := admin.Exec(`DROP DATABASE "` + dbName + `"`); err != nil {
			t.Logf("createIsolatedDB cleanup: DROP DATABASE %q: %v (non-fatal)", dbName, err)
		}
		admin.Close()
	}

	return isolatedDSN, cleanup
}

// TestMigrationsUpDown verifies that all migrations apply and roll back cleanly.
// It operates on a dedicated throwaway database so it never disrupts the shared
// test database used by concurrently running package test binaries.
func TestMigrationsUpDown(t *testing.T) {
	url := testDatabaseURL()
	if url == "" {
		t.Skip("DATABASE_URL not set — skipping migration integration test")
	}

	mdir := migrationsDir()

	// Provision a fresh, isolated database for this test alone.
	isolatedDSN, cleanup := createIsolatedDB(t, url)
	defer cleanup()

	// Ensure a clean slate on the isolated DB (no-op for a brand-new database,
	// but kept for symmetry with the original test intent).
	if err := RunMigrationsDown(isolatedDSN, mdir); err != nil {
		t.Fatalf("initial down (clean slate): %v", err)
	}

	// --- UP ---
	if err := RunMigrations(isolatedDSN, mdir); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	// Open a separate connection to query schema info.
	conn, err := Connect(isolatedDSN)
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
	if err := RunMigrationsDown(isolatedDSN, mdir); err != nil {
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

	// --- UP again (verify idempotence; isolated DB is dropped in cleanup) ---
	if err := RunMigrations(isolatedDSN, mdir); err != nil {
		t.Fatalf("migrate up (second pass): %v", err)
	}

	fmt.Println("migrate up→down→up cycle: OK")
}
