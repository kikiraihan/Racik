// Racik API server — backend Go native (net/http) + PostgreSQL.
package main

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kikiraihan/racik/backend/internal/ai"
	"github.com/kikiraihan/racik/backend/internal/applog"
	"github.com/kikiraihan/racik/backend/internal/config"
	"github.com/kikiraihan/racik/backend/internal/db"
	"github.com/kikiraihan/racik/backend/internal/handlers"
)

func main() {
	// Log server juga disimpan di memori (3000 baris terakhir) supaya bisa dilihat dari
	// Panel admin → Log server tanpa akses ke container.
	serverLog := applog.NewRing(3000)
	log.SetOutput(io.MultiWriter(os.Stderr, serverLog))

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
	if err := handlers.EnsureAdmins(ctx, conn, cfg); err != nil {
		log.Fatalf("admin: %v", err)
	}

	// System log ke Elasticsearch (activity user & request ke vendor AI). Dijalankan dengan
	// context terpisah supaya sisa log tetap terkirim saat shutdown.
	logger := applog.New(applog.Config{
		URL: cfg.LogESURL, User: cfg.LogESUser, Password: cfg.LogESPassword, APIKey: cfg.LogESAPIKey,
		IndexPrefix: cfg.LogIndexPrefix, RetentionDays: cfg.LogRetentionDays,
	})
	logCtx, stopLog := context.WithCancel(context.Background())
	go logger.Run(logCtx)
	go func() {
		setupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if err := logger.Setup(setupCtx); err != nil {
			log.Printf("PERINGATAN log: setup index template Elasticsearch gagal: %v", err)
		}
	}()

	api := handlers.New(conn, cfg, logger)
	api.ServerLog = serverLog
	// Katalog model (Panel admin → Katalog model): model env jadi model bawaan, lalu model
	// katalog yang aktif dipasang ke router (tersimpan di database).
	api.SyncCatalog(ctx)
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           api.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
		// Write timeout dibuat panjang karena generate AI bisa memakan waktu lama.
		// Magic Design = 1 panggilan teks + gambar (maks 4, paralel 3 → 2 putaran),
		// jadi bisa sampai 3x AI_TIMEOUT_SEC.
		WriteTimeout: time.Duration(3*cfg.AITimeoutSec+30) * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	go checkModels(ctx, cfg)
	go func() {
		log.Printf("Racik API berjalan di %s (AI kenari.id aktif: %v, log Elasticsearch: %v)", cfg.Addr, cfg.KenariAPIKey != "", logger.Enabled())
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	stopLog()
	logger.Wait(10 * time.Second)
	log.Println("server berhenti")
}

// checkModels mencocokkan model di env dengan daftar /models provider (kenari, Groq,
// NVIDIA teks, OpenRouter) dan hanya mencatat peringatan — id model yang salah baru
// ketahuan saat user generate.
func checkModels(ctx context.Context, cfg config.Config) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	for _, w := range ai.FromConfig(cfg).CheckModels(ctx) {
		log.Printf("PERINGATAN %s", w)
	}
}
