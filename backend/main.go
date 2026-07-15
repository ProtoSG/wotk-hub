package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"workhub/config"
	"workhub/middleware"
	"workhub/modules/auth"
	"workhub/modules/couple"
	"workhub/modules/dbmanager"
	"workhub/modules/finances"
	"workhub/modules/ytdlp"
	"workhub/store"

	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
)

// subscriptionsPollInterval controls how often due subscriptions are charged
// in the background. See finances.ProcessDueSubscriptions.
const subscriptionsPollInterval = 5 * time.Minute

// shutdownTimeout bounds how long graceful shutdown waits for in-flight
// requests to finish before forcing close.
const shutdownTimeout = 10 * time.Second

func main() {
	godotenv.Load()
	cfg := config.Load()

	if cfg.JWTSecret == "" {
		log.Fatal("JWT_SECRET is required")
	}
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	appDB, err := store.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("app database: %v", err)
	}
	if err := store.Migrate(appDB); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.CORS(cfg.CORSOrigin))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Auth's own router handles the public/protected split internally
	// (Register/Login/Refresh are public, Me/Logout require JWTAuth) since
	// chi doesn't let a parent r.Use exempt specific paths.
	r.Mount("/api/auth", auth.Routes(appDB, cfg.JWTSecret, cfg.CookieSecure))

	r.Group(func(pr chi.Router) {
		pr.Use(middleware.JWTAuth(cfg.JWTSecret))
		pr.With(middleware.RequireRole("admin")).Mount("/api/db", dbmanager.Routes())
		pr.Mount("/api/finances", finances.Routes(appDB))
		pr.With(middleware.RequireRole("admin", "guest")).Mount("/api/couple", couple.Routes(appDB))
		pr.With(middleware.RequireRole("admin", "guest")).Mount("/api/ytdlp", ytdlp.Routes(cfg.YtdlpCookiesPath))
	})

	// Unauthenticated by design (token-gated, not JWT) — for sharing with
	// someone who doesn't have an account. Not mounted at all unless
	// YTDLP_PUBLIC_TOKEN is set, so it can't be exposed by accident.
	if cfg.YtdlpPublicToken != "" {
		r.Mount("/api/ytdlp/public", ytdlp.PublicRoutes(cfg.YtdlpPublicToken, cfg.YtdlpCookiesPath))
	}

	addr := fmt.Sprintf(":%s", cfg.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Background job: charge due subscriptions on a timer instead of doing
	// it as a side effect of finances GET requests (List/Summary/Budgets
	// used to call processDue inline — see finances.ProcessDueSubscriptions).
	subStop := make(chan struct{})
	subDone := make(chan struct{})
	ticker := time.NewTicker(subscriptionsPollInterval)
	go func() {
		defer close(subDone)
		if err := finances.ProcessDueSubscriptions(appDB); err != nil {
			log.Printf("subscriptions: process due failed: %v", err)
		}
		for {
			select {
			case <-ticker.C:
				if err := finances.ProcessDueSubscriptions(appDB); err != nil {
					log.Printf("subscriptions: process due failed: %v", err)
				}
			case <-subStop:
				return
			}
		}
	}()

	go func() {
		log.Printf("Backend running on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("shutting down...")

	ticker.Stop()
	close(subStop)
	<-subDone

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("server shutdown: %v", err)
	}
	if err := appDB.Close(); err != nil {
		log.Printf("app database close: %v", err)
	}
	log.Println("shutdown complete")
}
