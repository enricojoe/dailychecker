// Package db provides PostgreSQL connectivity and schema migration utilities.
package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// Connect opens a sqlx connection to the given Postgres URL and verifies it
// with a Ping. Callers are responsible for calling Close on the returned DB.
func Connect(databaseURL string) (*sqlx.DB, error) {
	db, err := sqlx.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("db: open: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}
	return db, nil
}

// RunMigrations applies all pending up migrations found in migrationsDir.
// It is a no-op (not an error) when there are no .sql files or nothing new to migrate.
func RunMigrations(db *sql.DB, migrationsDir string) error {
	empty, err := dirHasNoSQLFiles(migrationsDir)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	if empty {
		log.Printf("migrate: no SQL files in %s — skipping", migrationsDir)
		return nil
	}

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("migrate: postgres driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://"+migrationsDir,
		"postgres",
		driver,
	)
	if err != nil {
		return fmt.Errorf("migrate: init: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate: up: %w", err)
	}

	srcErr, dbErr := m.Close()
	if srcErr != nil {
		log.Printf("migrate: close source: %v", srcErr)
	}
	if dbErr != nil {
		log.Printf("migrate: close db: %v", dbErr)
	}

	log.Println("migrate: migrations applied successfully")
	return nil
}

// dirHasNoSQLFiles reports whether dir contains no *.sql files.
func dirHasNoSQLFiles(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, fmt.Errorf("read dir %q: %w", dir, err)
	}
	for _, e := range entries {
		if !e.IsDir() && strings.EqualFold(filepath.Ext(e.Name()), ".sql") {
			return false, nil
		}
	}
	return true, nil
}
