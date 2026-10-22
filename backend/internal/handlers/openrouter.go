package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strings"

	"github.com/kikiraihan/racik/backend/internal/ai"
	"github.com/kikiraihan/racik/backend/internal/httpx"
)

// Model OpenRouter tambahan yang dipilih admin dari katalog (seperti admin → Model AI di
// tiara). Tersimpan di app_settings key "openrouter_models"; model dari env
// (OPENROUTER_TEXT_MODELS / OPENROUTER_IMAGE_MODELS) tetap ada dan tidak bisa dihapus dari sini.

const (
	settingOpenRouterModels = "openrouter_models"
	maxOpenRouterModels     = 50 // per jenis, supaya dropdown tetap terbaca
)

type openRouterModels struct {
	Text  []string `json:"text"`
	Image []string `json:"image"`
}

func (s *Server) loadOpenRouterModels(ctx context.Context) openRouterModels {
	var m openRouterModels
	var raw []byte
	if err := s.DB.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key=$1`, settingOpenRouterModels).Scan(&raw); err == nil {
		_ = json.Unmarshal(raw, &m)
	}
	m.Text, m.Image = nonNil(m.Text), nonNil(m.Image)
	return m
}

// ApplyOpenRouterModels memasang model OpenRouter tambahan dari database ke router.
// Dipanggil sekali saat start (setelah migrasi).
func (s *Server) ApplyOpenRouterModels(ctx context.Context) {
	if !s.AI.Has("openrouter") {
		return
	}
	m := s.loadOpenRouterModels(ctx)
	s.AI.SetExtraModels("openrouter", m.Text, m.Image)
	if n := len(m.Text) + len(m.Image); n > 0 {
		log.Printf("OpenRouter: %d model tambahan dari Panel admin dipasang", n)
	}
}

func (s *Server) openRouterResponse(ctx context.Context) map[string]any {
	return map[string]any{
		"enabled": s.AI.Catalog != nil,
		"added":   s.loadOpenRouterModels(ctx),
		"env":     openRouterModels{Text: nonNil(s.Cfg.OpenRouter.TextModels), Image: nonNil(s.Cfg.OpenRouter.ImageModels)},
	}
}

// GET /api/admin/openrouter/models?kind=text|image&refresh=1 — katalog model OpenRouter
// (cache 10 menit; refresh=1 ambil ulang) + model yang sudah ditambahkan admin.
func (s *Server) adminOpenRouterCatalog(w http.ResponseWriter, r *http.Request) {
	out := s.openRouterResponse(r.Context())
	out["items"] = []ai.CatalogModel{}
	if s.AI.Catalog == nil {
		httpx.JSON(w, 200, out)
		return
	}
	kind := r.URL.Query().Get("kind")
	if kind != ai.KindText && kind != ai.KindImage {
		kind = ""
	}
	items, err := s.AI.Catalog.Models(r.Context(), kind, r.URL.Query().Get("refresh") == "1")
	if err != nil {
		log.Printf("PERINGATAN openrouter: katalog model gagal dibaca: %v", err)
		httpx.Error(w, 502, "Katalog model OpenRouter gagal dibaca: "+err.Error())
		return
	}
	out["items"] = items
	httpx.JSON(w, 200, out)
}

// PUT /api/admin/openrouter/models — simpan daftar model OpenRouter tambahan {text, image}.
// Setiap id dicek ke katalog (harus ada dan jenisnya cocok), lalu langsung berlaku tanpa restart.
func (s *Server) adminPutOpenRouterModels(w http.ResponseWriter, r *http.Request) {
	if s.AI.Catalog == nil {
		httpx.Error(w, 400, "OpenRouter belum aktif, isi OPENROUTER_API_KEY di env backend lalu restart")
		return
	}
	var req openRouterModels
	if err := httpx.Decode(r, &req, 1<<16); err != nil {
		httpx.Error(w, 400, "body tidak valid")
		return
	}
	var err error
	if req.Text, err = s.checkCatalog(r.Context(), req.Text, ai.KindText, s.Cfg.OpenRouter.TextModels); err == nil {
		req.Image, err = s.checkCatalog(r.Context(), req.Image, ai.KindImage, s.Cfg.OpenRouter.ImageModels)
	}
	if err != nil {
		httpx.Error(w, 400, err.Error())
		return
	}
	activity(r, "models", req)
	if err := s.saveSetting(r.Context(), settingOpenRouterModels, req, httpx.CurrentUser(r).ID); err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	s.AI.SetExtraModels("openrouter", req.Text, req.Image)
	httpx.JSON(w, 200, s.openRouterResponse(r.Context()))
}

// checkCatalog merapikan daftar id (trim, buang duplikat & yang sudah ada di env) dan
// memastikan setiap id ada di katalog OpenRouter dengan jenis kind.
func (s *Server) checkCatalog(ctx context.Context, ids []string, kind string, env []string) ([]string, error) {
	name := map[string]string{ai.KindText: "teks", ai.KindImage: "gambar"}[kind]
	out := []string{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || slices.Contains(out, id) || slices.Contains(env, id) {
			continue
		}
		m, err := s.AI.Catalog.Find(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("katalog model OpenRouter gagal dibaca: %v", err)
		}
		if m == nil {
			return nil, fmt.Errorf("model %s %q tidak ada di katalog OpenRouter", name, id)
		}
		if m.Kind != kind {
			return nil, fmt.Errorf("model %q bukan model %s", id, name)
		}
		out = append(out, id)
	}
	if len(out) > maxOpenRouterModels {
		return nil, fmt.Errorf("maksimal %d model %s tambahan", maxOpenRouterModels, name)
	}
	return out, nil
}
