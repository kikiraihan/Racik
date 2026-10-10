// Racik API server — backend Go native (net/http) + PostgreSQL.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/kikiraihan/racik/backend/internal/config"
	"github.com/kikiraihan/racik/backend/internal/db"
	"github.com/kikiraihan/racik/backend/internal/handlers"
)

func main() {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	conn, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		log.Fatal(err)
	}
	if err := handlers.SeedTemplates(ctx, conn); err != nil {
		log.Fatalf("seed template: %v", err)
	}

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handlers.New(conn, cfg).Routes(),
		ReadHeaderTimeout: 10 * time.Second,
		// Write timeout dibuat panjang karena generate AI bisa memakan waktu lama.
		WriteTimeout: time.Duration(cfg.AITimeoutSec+30) * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	go func() {
		log.Printf("Racik API berjalan di %s (AI kenari.id aktif: %v)", cfg.Addr, cfg.KenariAPIKey != "")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	log.Println("server berhenti")
}
