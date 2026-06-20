package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/enricojoe/dailychecker/internal/activities"
	"github.com/enricojoe/dailychecker/internal/auth"
	"github.com/enricojoe/dailychecker/internal/config"
	"github.com/enricojoe/dailychecker/internal/db"
	"github.com/enricojoe/dailychecker/internal/httpapi"
	"github.com/enricojoe/dailychecker/internal/users"
)

func main() {
	// Resolve paths relative to the working directory so that
	// `cd backend && go run ./cmd/server` picks up backend/.env and backend/migrations.
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("main: getwd: %v", err)
	}

	cfg, err := config.Load(filepath.Join(wd, ".env"))
	if err != nil {
		log.Fatalf("main: config: %v", err)
	}

	database, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("main: db: %v", err)
	}
	defer database.Close()

	if err := db.RunMigrations(cfg.DatabaseURL, filepath.Join(wd, "migrations")); err != nil {
		log.Fatalf("main: migrations: %v", err)
	}

	// Construct repositories and services — dependencies flow inward.
	userRepo := users.NewRepository(database)
	tokenRepo := auth.NewTokenRepository(database)
	authSvc := auth.NewService(userRepo, tokenRepo, cfg)

	actRepo := activities.NewRepository(database)
	actSvc := activities.NewService(actRepo)

	router := httpapi.NewRouter(authSvc, actSvc, cfg.JWTSecret)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("server: listening on %s (env=%s)", srv.Addr, cfg.AppEnv)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: ListenAndServe: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("server: shutting down gracefully")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("server: forced shutdown: %v", err)
	}

	log.Println("server: stopped")
}
