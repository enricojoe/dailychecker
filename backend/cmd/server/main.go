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
	"github.com/enricojoe/dailychecker/internal/occurrences"
	"github.com/enricojoe/dailychecker/internal/scheduler"
	"github.com/enricojoe/dailychecker/internal/telegram"
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

	// Load the Jakarta timezone once; fail fast on misconfiguration.
	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		log.Fatalf("main: load timezone %q: %v", cfg.Timezone, err)
	}

	// Construct repositories and services — dependencies flow inward.
	userRepo := users.NewRepository(database)
	tokenRepo := auth.NewTokenRepository(database)
	authSvc := auth.NewService(userRepo, tokenRepo, cfg)

	actRepo := activities.NewRepository(database)
	actSvc := activities.NewService(actRepo)

	occRepo := occurrences.NewRepository(database)
	occSvc := occurrences.NewService(occRepo, actRepo, loc)

	// Telegram is optional: a missing bot token is not fatal — the server
	// boots fine without it and the Telegram routes are not registered.
	var tgSvc *telegram.Service
	var poller *telegram.Poller
	var sched *scheduler.Scheduler

	routerCfg := httpapi.RouterConfig{
		CORSAllowedOrigins: cfg.CORSAllowedOrigins,
	}

	if cfg.TelegramBotToken != "" {
		tgClient := telegram.NewClient(cfg.TelegramBotToken, "https://api.telegram.org", nil)
		tgSvc = telegram.NewService(userRepo, cfg, tgClient)
		sched = scheduler.New(occRepo, tgClient, loc, time.Now, cfg.DigestHour, cfg.AppPublicURL)

		switch cfg.TelegramMode {
		case "webhook":
			// Webhook mode: register the route; do NOT start the poller.
			routerCfg.TelegramWebhookMode = true
			routerCfg.TelegramWebhookSecret = cfg.TelegramWebhookSecret
			log.Println("telegram: webhook mode")
		default:
			// Polling mode (default): start the long-poll loop.
			poller = telegram.NewPoller(cfg.TelegramBotToken, "https://api.telegram.org", nil, tgSvc)
			log.Println("telegram: polling mode")
		}
	} else {
		log.Println("telegram: disabled (TELEGRAM_BOT_TOKEN not set); scheduler not started")
	}

	router := httpapi.NewRouter(authSvc, actSvc, occSvc, tgSvc, cfg.JWTSecret, routerCfg)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// pollerCtx is cancelled on shutdown so the poll loop and scheduler exit cleanly.
	pollerCtx, cancelPoller := context.WithCancel(context.Background())
	defer cancelPoller()

	if poller != nil {
		go poller.Run(pollerCtx)
	}

	if sched != nil {
		sched.Start(pollerCtx)
	}

	// In webhook mode, register the webhook with Telegram on startup.
	// A failure here is logged but does not crash the server — the endpoint
	// will receive no traffic until the registration succeeds, but the server
	// itself is fully operational (important in dev where there is no real token).
	if cfg.TelegramBotToken != "" && cfg.TelegramMode == "webhook" && cfg.TelegramWebhookURL != "" {
		webhookURL := cfg.TelegramWebhookURL + "/api/telegram/webhook"
		tgClient := telegram.NewClient(cfg.TelegramBotToken, "https://api.telegram.org", nil)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := tgClient.SetWebhook(ctx, webhookURL, cfg.TelegramWebhookSecret); err != nil {
			log.Printf("telegram: SetWebhook failed (server continues): %v", err)
		} else {
			log.Printf("telegram: webhook registered at %s", webhookURL)
		}
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

	// Stop the Telegram poller and scheduler before draining HTTP connections.
	cancelPoller()
	if sched != nil {
		sched.Stop()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("server: forced shutdown: %v", err)
	}

	log.Println("server: stopped")
}
