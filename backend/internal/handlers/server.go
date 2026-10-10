// Package handlers berisi seluruh endpoint REST API Racik.
package handlers

import (
	"database/sql"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/kikiraihan/racik/backend/internal/ai"
	"github.com/kikiraihan/racik/backend/internal/config"
	"github.com/kikiraihan/racik/backend/internal/httpx"
)

type Server struct {
	DB  *sql.DB
	Cfg config.Config
	AI  *ai.Client
}

func New(db *sql.DB, cfg config.Config) *Server {
	_ = os.MkdirAll(cfg.UploadDir, 0o755)
	return &Server{DB: db, Cfg: cfg, AI: ai.New(cfg.KenariBaseURL, cfg.KenariAPIKey, time.Duration(cfg.AITimeoutSec)*time.Second)}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	auth := func(h http.HandlerFunc) http.HandlerFunc { return httpx.RequireAuth(s.Cfg.JWTSecret, h) }

	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		httpx.JSON(w, 200, map[string]any{"ok": true, "ai": s.AI.Configured()})
	})

	// Auth
	mux.HandleFunc("POST /api/auth/register", s.register)
	mux.HandleFunc("POST /api/auth/login", s.login)
	mux.HandleFunc("POST /api/auth/guest", s.guest)
	mux.HandleFunc("POST /api/auth/upgrade", auth(s.upgradeGuest))
	mux.HandleFunc("GET /api/auth/me", auth(s.me))

	// Designs
	mux.HandleFunc("GET /api/designs", auth(s.listDesigns))
	mux.HandleFunc("POST /api/designs", auth(s.createDesign))
	mux.HandleFunc("GET /api/designs/{id}", auth(s.getDesign))
	mux.HandleFunc("PUT /api/designs/{id}", auth(s.updateDesign))
	mux.HandleFunc("DELETE /api/designs/{id}", auth(s.deleteDesign))
	mux.HandleFunc("POST /api/designs/{id}/duplicate", auth(s.duplicateDesign))

	// Assets (upload gambar)
	mux.HandleFunc("GET /api/assets", auth(s.listAssets))
	mux.HandleFunc("POST /api/assets", auth(s.uploadAsset))
	mux.HandleFunc("DELETE /api/assets/{id}", auth(s.deleteAsset))

	// Templates
	mux.HandleFunc("GET /api/templates", s.listTemplates)
	mux.HandleFunc("GET /api/templates/{id}", s.getTemplate)

	// Brand kit
	mux.HandleFunc("GET /api/brand-kit", auth(s.getBrandKit))
	mux.HandleFunc("PUT /api/brand-kit", auth(s.putBrandKit))

	// AI connector (kenari.id)
	mux.HandleFunc("GET /api/ai/models", s.aiModels)
	mux.HandleFunc("POST /api/ai/design", auth(s.aiDesign))
	mux.HandleFunc("POST /api/ai/refine", auth(s.aiRefine))
	mux.HandleFunc("POST /api/ai/image", auth(s.aiImage))
	mux.HandleFunc("POST /api/ai/image-edit", auth(s.aiImageEdit))
	mux.HandleFunc("POST /api/ai/copy", auth(s.aiCopy))
	mux.HandleFunc("GET /api/ai/generations", auth(s.listGenerations))
	mux.HandleFunc("POST /api/ai/generations/{id}/rate", auth(s.rateGeneration))

	// File statis hasil upload / AI
	files := http.StripPrefix("/uploads/", http.FileServer(http.Dir(filepath.Clean(s.Cfg.UploadDir))))
	mux.Handle("GET /uploads/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		files.ServeHTTP(w, r)
	}))

	return httpx.Logger(httpx.CORS(s.Cfg.CORSOrigins, mux))
}
