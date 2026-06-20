// Package testhelper provides shared utilities for repository integration tests.
// Tests that call OpenTestDB require a real PostgreSQL instance. OpenTestDB
// attempts to load backend/.env automatically (via godotenv) so that
// DATABASE_URL is available without the caller needing to export it manually.
// If DATABASE_URL is still unset after the .env load attempt, OpenTestDB
// signals the caller to skip.
package testhelper

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/enricojoe/dailychecker/internal/db"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
)

// OpenTestDB opens a connection to the test database and runs all pending
// migrations. Returns (nil, true) if DATABASE_URL is unset after attempting
// to load backend/.env — callers should treat this as a skip signal (call
// os.Exit(0) from TestMain). Returns (*sqlx.DB, false) on success. Panics on
// connection or migration failure so that test output is immediately actionable.
func OpenTestDB() (*sqlx.DB, bool) {
	// Try loading backend/.env so tests run without manual environment setup.
	// This is a best-effort load: if the file is absent or already overridden
	// by the real environment, it silently does nothing.
	if envFile := dotEnvPath(); envFile != "" {
		_ = godotenv.Load(envFile) // existing env vars are NOT overwritten
	}

	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return nil, true
	}

	conn, err := db.Connect(url)
	if err != nil {
		panic(fmt.Sprintf("testhelper: connect: %v", err))
	}

	if err := db.RunMigrations(url, MigrationsDir()); err != nil {
		panic(fmt.Sprintf("testhelper: migrate: %v", err))
	}

	return conn, false
}

// MigrationsDir returns the absolute path to backend/migrations/.
// It resolves relative to this source file so it works regardless of the
// working directory when `go test ./...` is invoked.
func MigrationsDir() string {
	if d := os.Getenv("MIGRATIONS_DIR"); d != "" {
		return d
	}
	// This file lives at backend/internal/testhelper/db.go.
	// backend/migrations/ is two directories up from this file's directory.
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "migrations" // last-resort relative fallback
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "migrations")
}

// dotEnvPath returns the absolute path to backend/.env, resolved relative to
// this source file. Returns "" if the caller location cannot be determined.
func dotEnvPath() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	// This file: backend/internal/testhelper/db.go
	// backend/.env is three directories up from this file's directory.
	return filepath.Join(filepath.Dir(file), "..", "..", ".env")
}
