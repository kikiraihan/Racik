package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kikiraihan/racik/backend/internal/ai"
	"github.com/kikiraihan/racik/backend/internal/httpx"
)

// Panel admin → Katalog model (seperti admin → Model AI di tiara/katilidev): daftar model yang
// tersedia di sistem, terpisah dari Konfigurasi AI. Katalog tersimpan di tabel ai_models;
// Konfigurasi AI hanya menawarkan model katalog yang aktif dari provider yang API key-nya diisi.
// Model env (KENARI_*_MODELS, OPENROUTER_*_MODELS, ...) masuk otomatis sebagai model bawaan;
// model lain ditambahkan dari daftar GET /models OpenRouter dan kenari.

const (
	modelActive   = "active"
	modelDisabled = "disabled"

	maxCatalogModels = 100 // per provider & jenis, supaya dropdown Konfigurasi AI tetap terbaca
)

type catalogRow struct {
	ID              int64     `json:"id"`
	Provider        string    `json:"provider"`
	ModelID         string    `json:"modelId"`
	Choice          string    `json:"choice"` // "<provider>:<model>", nilai yang disimpan Konfigurasi AI
	Kind            string    `json:"kind"`
	Name            string    `json:"name"`
	Maker           string    `json:"maker"`
	Status          string    `json:"status"`
	IsBuiltin       bool      `json:"isBuiltin"`
	IsFree          bool      `json:"isFree"`
	ContextLength   *int      `json:"contextLength"`
	PromptPrice     *float64  `json:"promptPrice"`
	CompletionPrice *float64  `json:"completionPrice"`
	ImagePrice      *float64  `json:"imagePrice"`
	Notes           string    `json:"notes"`
	CreatedAt       time.Time `json:"createdAt"`
	// InUse: fitur di Konfigurasi AI yang memakai model ini (text | image | edit).
	InUse []string `json:"inUse"`
}

const catalogCols = `id, provider, model_id, kind, name, maker, status, is_builtin, is_free, context_length,
	prompt_price, completion_price, image_price, notes, created_at`

func scanCatalogRow(sc interface{ Scan(...any) error }) (catalogRow, error) {
	var x catalogRow
	err := sc.Scan(&x.ID, &x.Provider, &x.ModelID, &x.Kind, &x.Name, &x.Maker, &x.Status, &x.IsBuiltin, &x.IsFree,
		&x.ContextLength, &x.PromptPrice, &x.CompletionPrice, &x.ImagePrice, &x.Notes, &x.CreatedAt)
	x.Choice = ai.Choice(x.Provider, x.ModelID)
	x.InUse = []string{}
	return x, err
}

// SyncCatalog memasukkan model env sebagai model bawaan lalu memasang model katalog yang aktif
// ke router. Dipanggil sekali saat start (setelah migrasi). Gagal = router tetap memakai model env.
func (s *Server) SyncCatalog(ctx context.Context) {
	if err := s.ensureBuiltinModels(ctx); err != nil {
		log.Printf("PERINGATAN katalog model: model bawaan env gagal disimpan: %v", err)
	}
	if err := s.applyCatalog(ctx); err != nil {
		log.Printf("PERINGATAN katalog model: gagal dibaca, memakai model env: %v", err)
	}
}

// ensureBuiltinModels: model env tiap provider selalu ada di katalog sebagai model bawaan (tidak
// bisa dihapus, tapi boleh dinonaktifkan). Baris yang sudah ada hanya ditandai bawaan — status
// pilihan admin tidak ditimpa; model bawaan lama (env diganti) menjadi model biasa.
func (s *Server) ensureBuiltinModels(ctx context.Context) error {
	keys := []string{}
	for _, p := range s.AI.Providers() {
		for kind, ids := range map[string][]string{ai.KindText: p.EnvText, ai.KindImage: p.EnvImage} {
			for _, id := range ids {
				id = strings.TrimSpace(id)
				if id == "" {
					continue
				}
				keys = append(keys, p.Name+"|"+kind+"|"+id)
				maker, _, _ := strings.Cut(id, "/")
				if _, err := s.DB.ExecContext(ctx, `INSERT INTO ai_models (provider, kind, model_id, name, maker, is_builtin, is_free)
					VALUES ($1, $2, $3, $3, $4, true, $5)
					ON CONFLICT (provider, kind, model_id) DO UPDATE SET is_builtin = true`,
					p.Name, kind, id, maker, strings.HasSuffix(id, ":free")); err != nil {
					return err
				}
			}
		}
	}
	_, err := s.DB.ExecContext(ctx, `UPDATE ai_models SET is_builtin = false, updated_at = now()
		WHERE is_builtin AND NOT (provider || '|' || kind || '|' || model_id = ANY($1))`, keys)
	return err
}

// applyCatalog memasang model katalog yang aktif ke router (langsung berlaku tanpa restart).
func (s *Server) applyCatalog(ctx context.Context) error {
	rows, err := s.DB.QueryContext(ctx, `SELECT provider, kind, model_id FROM ai_models WHERE status = $1
		ORDER BY is_builtin DESC, id`, modelActive)
	if err != nil {
		return err
	}
	defer rows.Close()
	models := map[string]ai.CatalogModels{}
	for rows.Next() {
		var provider, kind, id string
		if err := rows.Scan(&provider, &kind, &id); err != nil {
			return err
		}
		m := models[provider]
		if kind == ai.KindImage {
			m.Image = append(m.Image, id)
		} else {
			m.Text = append(m.Text, id)
		}
		models[provider] = m
	}
	if err := rows.Err(); err != nil {
		return err
	}
	s.AI.SetCatalog(models)
	return nil
}

func (s *Server) catalogRows(ctx context.Context) ([]catalogRow, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+catalogCols+` FROM ai_models ORDER BY is_builtin DESC, provider, kind, model_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	saved := s.loadAIConfig(ctx)
	active := s.activeModels(ctx)
	out := []catalogRow{}
	for rows.Next() {
		x, err := scanCatalogRow(rows)
		if err != nil {
			return nil, err
		}
		// Tanda "dipakai": pilihan admin, atau default env yang sedang berlaku.
		for use, pair := range map[string][2]string{
			"text":  {saved.TextModel, active.TextModel},
			"image": {saved.ImageModel, active.ImageModel},
			"edit":  {saved.EditModel, active.EditModel},
		} {
			wantKind := ai.KindText
			if use != "text" {
				wantKind = ai.KindImage
			}
			if x.Kind == wantKind && (pair[0] == x.Choice || pair[1] == x.Choice) {
				x.InUse = append(x.InUse, use)
			}
		}
		slices.Sort(x.InUse)
		out = append(out, x)
	}
	return out, rows.Err()
}

// savedUse: fitur Konfigurasi AI yang secara eksplisit memilih choice (model berjenis kind).
func (s *Server) savedUse(ctx context.Context, choice, kind string) []string {
	c := s.loadAIConfig(ctx)
	var out []string
	if kind == ai.KindText && c.TextModel == choice {
		out = append(out, "teks")
	}
	if kind == ai.KindImage && c.ImageModel == choice {
		out = append(out, "gambar")
	}
	if kind == ai.KindImage && c.EditModel == choice {
		out = append(out, "edit gambar")
	}
	return out
}

func (s *Server) catalogResponse(ctx context.Context) (map[string]any, error) {
	items, err := s.catalogRows(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]any{"providers": s.AI.Providers(), "items": items}, nil
}

// GET /api/admin/models — katalog model + daftar provider (aktif / punya daftar API).
func (s *Server) adminListModels(w http.ResponseWriter, r *http.Request) {
	res, err := s.catalogResponse(r.Context())
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, res)
}

type remoteRow struct {
	ai.CatalogModel
	InCatalog bool `json:"inCatalog"`
}

// GET /api/admin/models/remote?provider=openrouter|kenari&kind=text|image&refresh=1 — daftar
// model dari API provider (cache 10 menit; refresh=1 ambil ulang) untuk ditambahkan ke katalog.
func (s *Server) adminRemoteModels(w http.ResponseWriter, r *http.Request) {
	provider := r.URL.Query().Get("provider")
	cat := s.AI.Catalogs[provider]
	if cat == nil {
		httpx.Error(w, 400, "Daftar model "+provider+" tidak tersedia, isi API key provider itu di env backend lalu restart")
		return
	}
	kind := r.URL.Query().Get("kind")
	if kind != ai.KindImage {
		kind = ai.KindText
	}
	list, err := cat.Models(r.Context(), kind, r.URL.Query().Get("refresh") == "1")
	if err != nil {
		log.Printf("PERINGATAN %s: daftar model gagal dibaca: %v", provider, err)
		httpx.Error(w, 502, "Daftar model "+provider+" gagal dibaca: "+err.Error())
		return
	}
	have := map[string]bool{}
	rows, err := s.DB.QueryContext(r.Context(), `SELECT model_id FROM ai_models WHERE provider = $1 AND kind = $2`, provider, kind)
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			have[id] = true
		}
	}
	out := make([]remoteRow, 0, len(list))
	for _, m := range list {
		out = append(out, remoteRow{CatalogModel: m, InCatalog: have[m.ID]})
	}
	httpx.JSON(w, 200, map[string]any{"items": out})
}

// POST /api/admin/models — tambahkan model dari daftar API provider ke katalog:
// {"provider": "kenari", "kind": "text", "ids": ["step-3-7-flash:free"]}. Setiap id dicek ke
// daftar provider (harus ada dan jenisnya cocok), lalu langsung muncul di Konfigurasi AI.
func (s *Server) adminAddModels(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Provider string   `json:"provider"`
		Kind     string   `json:"kind"`
		IDs      []string `json:"ids"`
	}
	if err := httpx.Decode(r, &req, 1<<16); err != nil {
		httpx.Error(w, 400, "body tidak valid")
		return
	}
	cat := s.AI.Catalogs[req.Provider]
	if cat == nil {
		httpx.Error(w, 400, "Provider "+req.Provider+" belum aktif atau tidak punya daftar model")
		return
	}
	if req.Kind != ai.KindText && req.Kind != ai.KindImage {
		httpx.Error(w, 400, "jenis model harus text atau image")
		return
	}
	if len(req.IDs) == 0 || len(req.IDs) > maxCatalogModels {
		httpx.Error(w, 400, fmt.Sprintf("pilih 1–%d model", maxCatalogModels))
		return
	}
	var count int
	if err := s.DB.QueryRowContext(r.Context(), `SELECT count(*) FROM ai_models WHERE provider = $1 AND kind = $2`,
		req.Provider, req.Kind).Scan(&count); err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	activity(r, "models", req)
	added, skipped := 0, 0
	for _, id := range req.IDs {
		id = strings.TrimSpace(id)
		m, err := cat.Find(r.Context(), req.Kind, id)
		if err != nil {
			httpx.Error(w, 502, "Daftar model "+req.Provider+" gagal dibaca: "+err.Error())
			return
		}
		if m == nil {
			skipped++
			continue
		}
		if count >= maxCatalogModels {
			httpx.Error(w, 400, fmt.Sprintf("maksimal %d model per provider & jenis di katalog (%d ditambahkan)", maxCatalogModels, added))
			return
		}
		res, err := s.DB.ExecContext(r.Context(), `INSERT INTO ai_models (provider, kind, model_id, name, maker, is_free,
			context_length, prompt_price, completion_price, image_price)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) ON CONFLICT (provider, kind, model_id) DO NOTHING`,
			req.Provider, req.Kind, m.ID, m.Name, m.Maker, m.IsFree, m.ContextLength, m.PromptPrice, m.CompletionPrice, m.ImagePrice)
		if err != nil {
			httpx.Error(w, 500, err.Error())
			return
		}
		if n, _ := res.RowsAffected(); n == 1 {
			added++
			count++
		} else {
			skipped++
		}
	}
	if err := s.applyCatalog(r.Context()); err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	res, err := s.catalogResponse(r.Context())
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	res["added"], res["skipped"] = added, skipped
	httpx.JSON(w, 200, res)
}

func (s *Server) catalogRow(ctx context.Context, id string) (*catalogRow, error) {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil, nil
	}
	x, err := scanCatalogRow(s.DB.QueryRowContext(ctx, `SELECT `+catalogCols+` FROM ai_models WHERE id = $1`, n))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &x, nil
}

// PATCH /api/admin/models/{id} — ubah status (active | disabled), nama tampilan, atau catatan.
// Model yang sedang dipilih di Konfigurasi AI tidak bisa dinonaktifkan.
func (s *Server) adminUpdateModel(w http.ResponseWriter, r *http.Request) {
	cur, err := s.catalogRow(r.Context(), r.PathValue("id"))
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	if cur == nil {
		httpx.Error(w, 404, "model tidak ditemukan")
		return
	}
	var req struct {
		Status *string `json:"status"`
		Name   *string `json:"name"`
		Notes  *string `json:"notes"`
	}
	if err := httpx.Decode(r, &req, 1<<14); err != nil {
		httpx.Error(w, 400, "body tidak valid")
		return
	}
	status, name, notes := cur.Status, cur.Name, cur.Notes
	if req.Status != nil {
		status = *req.Status
		if status != modelActive && status != modelDisabled {
			httpx.Error(w, 400, "status harus active atau disabled")
			return
		}
		if status == modelDisabled && cur.Status != modelDisabled {
			if used := s.savedUse(r.Context(), cur.Choice, cur.Kind); len(used) > 0 {
				httpx.Error(w, 400, "Model ini sedang dipakai untuk "+strings.Join(used, ", ")+" di Konfigurasi AI — pilih model lain dulu")
				return
			}
		}
	}
	if req.Name != nil {
		if name = strings.TrimSpace(*req.Name); name == "" {
			name = cur.ModelID
		}
		if len([]rune(name)) > 200 {
			httpx.Error(w, 400, "nama maksimal 200 karakter")
			return
		}
	}
	if req.Notes != nil {
		if notes = strings.TrimSpace(*req.Notes); len([]rune(notes)) > 2000 {
			httpx.Error(w, 400, "catatan maksimal 2000 karakter")
			return
		}
	}
	activity(r, "model", map[string]any{"choice": cur.Choice, "kind": cur.Kind, "status": status})
	if _, err := s.DB.ExecContext(r.Context(), `UPDATE ai_models SET status = $2, name = $3, notes = $4, updated_at = now() WHERE id = $1`,
		cur.ID, status, name, notes); err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	if err := s.applyCatalog(r.Context()); err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	res, err := s.catalogResponse(r.Context())
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, res)
}

// DELETE /api/admin/models/{id} — hapus model dari katalog. Model bawaan (env) dan model yang
// sedang dipilih di Konfigurasi AI tidak bisa dihapus (model bawaan cukup dinonaktifkan).
func (s *Server) adminDeleteModel(w http.ResponseWriter, r *http.Request) {
	cur, err := s.catalogRow(r.Context(), r.PathValue("id"))
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	if cur == nil {
		httpx.Error(w, 404, "model tidak ditemukan")
		return
	}
	if cur.IsBuiltin {
		httpx.Error(w, 400, "Model bawaan dari env tidak bisa dihapus — nonaktifkan saja, atau hapus dari env backend")
		return
	}
	if used := s.savedUse(r.Context(), cur.Choice, cur.Kind); len(used) > 0 {
		httpx.Error(w, 400, "Model ini sedang dipakai untuk "+strings.Join(used, ", ")+" di Konfigurasi AI — pilih model lain dulu")
		return
	}
	activity(r, "model", map[string]any{"choice": cur.Choice, "kind": cur.Kind})
	if _, err := s.DB.ExecContext(r.Context(), `DELETE FROM ai_models WHERE id = $1`, cur.ID); err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	if err := s.applyCatalog(r.Context()); err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	res, err := s.catalogResponse(r.Context())
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, res)
}
