// Racik API server — backend Go native (net/http) + PostgreSQL.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"slices"
	"syscall"
	"time"

	"github.com/kikiraihan/racik/backend/internal/ai"
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
		// Magic Design = 1 panggilan teks + gambar (maks 4, paralel 3 → 2 putaran),
		// jadi bisa sampai 3x AI_TIMEOUT_SEC.
		WriteTimeout: time.Duration(3*cfg.AITimeoutSec+30) * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	if cfg.KenariAPIKey != "" {
		go checkKenariModels(ctx, cfg)
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

// checkKenariModels mencocokkan model di env dengan GET {KENARI_BASE_URL}/models dan
// hanya mencatat peringatan — id model yang salah baru ketahuan saat user generate.
func checkKenariModels(ctx context.Context, cfg config.Config) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	client := ai.New(cfg.KenariBaseURL, cfg.KenariAPIKey, 30*time.Second)
	for _, g := range []struct {
		endpoint string
		models   []string
	}{{"chat", cfg.KenariTextModels}, {"images", cfg.KenariImageModels}} {
		ids, err := client.ListModels(ctx, g.endpoint)
		if err != nil || len(ids) == 0 {
			log.Printf("kenari: daftar model (%s) tidak bisa dibaca, cek model dilewati: %v", g.endpoint, err)
			return
		}
		for _, m := range g.models {
			if !slices.Contains(ids, m) {
				log.Printf("PERINGATAN kenari: model %q tidak ada di daftar /models (%s) — cek KENARI_*_MODEL(S) di .env", m, g.endpoint)
			}
		}
	}
}
