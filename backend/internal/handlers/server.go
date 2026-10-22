// Package handlers berisi seluruh endpoint REST API Racik.
package handlers

import (
	"database/sql"
	"net/http"
	"os"
	"path/filepath"

	"github.com/kikiraihan/racik/backend/internal/ai"
	"github.com/kikiraihan/racik/backend/internal/applog"
	"github.com/kikiraihan/racik/backend/internal/auth"
	"github.com/kikiraihan/racik/backend/internal/config"
	"github.com/kikiraihan/racik/backend/internal/httpx"
)

type Server struct {
	DB  *sql.DB
	Cfg config.Config
	AI  *ai.Router
	Log *applog.Logger // system log: activity user & request ke vendor AI (Elasticsearch)
	// ServerLog: baris log server terakhir di memori (menu Log server di panel admin).
	ServerLog *applog.Ring

	Google *auth.GoogleVerifier // verifikasi ID token untuk login Google
}

func New(db *sql.DB, cfg config.Config, logger *applog.Logger) *Server {
	_ = os.MkdirAll(cfg.UploadDir, 0o755)
	s := &Server{DB: db, Cfg: cfg, AI: ai.FromConfig(cfg), Log: logger, Google: auth.NewGoogleVerifier(cfg.GoogleClientIDs)}
	s.AI.Observe = s.logVendorCall
	return s
}

func (s *Server) auth(h http.HandlerFunc) http.HandlerFunc {
	return httpx.RequireAuth(s.Cfg.JWTSecret, h)
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	auth := s.auth
	admin := s.requireAdmin
	// track: endpoint riskan yang dicatat di activity log.
	track := s.track

	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		httpx.JSON(w, 200, map[string]any{"ok": true, "ai": s.AI.Configured()})
	})

	// Auth
	mux.HandleFunc("POST /api/auth/register", track("auth.register", s.register))
	mux.HandleFunc("POST /api/auth/login", track("auth.login", s.login))
	mux.HandleFunc("POST /api/auth/guest", track("auth.guest", s.guest))
	mux.HandleFunc("POST /api/auth/upgrade", track("auth.upgrade", auth(s.upgradeGuest)))
	mux.HandleFunc("GET /api/auth/me", auth(s.me))
	// Login Google: config publik (client ID) untuk tombol di frontend + tukar ID token jadi JWT Racik.
	mux.HandleFunc("GET /api/auth/config", s.authConfig)
	mux.HandleFunc("POST /api/auth/google", track("auth.google", s.googleAuth))

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

	// AI connector (kenari.id + provider lain, lihat internal/ai/router.go)
	mux.HandleFunc("GET /api/ai/models", s.aiModels)
	// Fitur AI berbayar: saldo dipotong per hit (lihat billing.go).
	mux.HandleFunc("POST /api/ai/design", track("ai.design", auth(s.aiDesign)))
	mux.HandleFunc("POST /api/ai/refine", track("ai.refine", auth(s.aiRefine)))
	mux.HandleFunc("POST /api/ai/image", track("ai.image", auth(s.aiImage)))
	mux.HandleFunc("POST /api/ai/image-edit", track("ai.image_edit", auth(s.aiImageEdit)))
	mux.HandleFunc("POST /api/ai/copy", track("ai.copy", auth(s.aiCopy)))
	mux.HandleFunc("GET /api/ai/generations", auth(s.listGenerations))
	mux.HandleFunc("POST /api/ai/generations/{id}/rate", auth(s.rateGeneration))

	// Saldo user
	mux.HandleFunc("GET /api/wallet", auth(s.wallet))

	// Admin: user & saldo, katalog model, konfigurasi AI, konfigurasi harga, system log
	mux.HandleFunc("GET /api/admin/users", admin(s.adminListUsers))
	mux.HandleFunc("PATCH /api/admin/users/{id}", track("admin.user_role", admin(s.adminUpdateUser)))
	mux.HandleFunc("POST /api/admin/users/{id}/balance", track("admin.balance", admin(s.adminAdjustBalance)))
	mux.HandleFunc("GET /api/admin/users/{id}/transactions", admin(s.adminUserTx))
	mux.HandleFunc("GET /api/admin/ai-config", admin(s.adminGetAIConfig))
	mux.HandleFunc("PUT /api/admin/ai-config", track("admin.ai_config", admin(s.adminPutAIConfig)))
	// Katalog model: model yang tersedia di sistem, ditambah dari daftar OpenRouter / kenari
	// tanpa ubah env (catalog.go). Konfigurasi AI hanya menawarkan model katalog yang aktif.
	mux.HandleFunc("GET /api/admin/models", admin(s.adminListModels))
	mux.HandleFunc("GET /api/admin/models/remote", admin(s.adminRemoteModels))
	mux.HandleFunc("POST /api/admin/models", track("admin.models_add", admin(s.adminAddModels)))
	mux.HandleFunc("PATCH /api/admin/models/{id}", track("admin.model_update", admin(s.adminUpdateModel)))
	mux.HandleFunc("DELETE /api/admin/models/{id}", track("admin.model_delete", admin(s.adminDeleteModel)))
	mux.HandleFunc("GET /api/admin/pricing", admin(s.adminGetPricing))
	mux.HandleFunc("PUT /api/admin/pricing", track("admin.pricing", admin(s.adminPutPricing)))
	mux.HandleFunc("GET /api/admin/logs", admin(s.adminLogs))
	mux.HandleFunc("GET /api/admin/overview", admin(s.adminOverview))
	mux.HandleFunc("GET /api/admin/server-logs", admin(s.adminServerLogs))

	// File statis hasil upload / AI
	files := http.StripPrefix("/uploads/", http.FileServer(http.Dir(filepath.Clean(s.Cfg.UploadDir))))
	mux.Handle("GET /uploads/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		files.ServeHTTP(w, r)
	}))

	return httpx.Logger(httpx.CORS(s.Cfg.CORSOrigins, mux))
}
