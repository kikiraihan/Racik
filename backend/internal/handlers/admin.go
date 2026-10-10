package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kikiraihan/racik/backend/internal/applog"
	"github.com/kikiraihan/racik/backend/internal/auth"
	"github.com/kikiraihan/racik/backend/internal/billing"
	"github.com/kikiraihan/racik/backend/internal/config"
	"github.com/kikiraihan/racik/backend/internal/httpx"
)

const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

// requireAdmin: endpoint khusus admin. Role dibaca dari database (bukan dari token) supaya
// pencabutan akses admin langsung berlaku.
func (s *Server) requireAdmin(h http.HandlerFunc) http.HandlerFunc {
	return s.auth(func(w http.ResponseWriter, r *http.Request) {
		var role string
		err := s.DB.QueryRowContext(r.Context(), `SELECT role FROM users WHERE id=$1`, httpx.CurrentUser(r).ID).Scan(&role)
		if err != nil || role != RoleAdmin {
			httpx.Error(w, http.StatusForbidden, "khusus admin")
			return
		}
		h(w, r)
	})
}

// EnsureAdmins menjadikan email di ADMIN_EMAILS sebagai admin. Kalau ADMIN_PASSWORD diisi
// dan email pertama belum terdaftar, akun admin dibuat.
func EnsureAdmins(ctx context.Context, db *sql.DB, cfg config.Config) error {
	if len(cfg.AdminEmails) == 0 {
		return nil
	}
	if cfg.AdminPassword != "" {
		hash, err := auth.HashPassword(cfg.AdminPassword)
		if err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO users(email, name, password_hash, role) VALUES($1,'Admin',$2,'admin')
			ON CONFLICT (email) DO NOTHING`, cfg.AdminEmails[0], hash); err != nil {
			return err
		}
	}
	_, err := db.ExecContext(ctx, `UPDATE users SET role='admin' WHERE email = ANY($1) AND role <> 'admin'`, cfg.AdminEmails)
	return err
}

func (s *Server) isAdminEmail(email string) bool {
	return slices.Contains(s.Cfg.AdminEmails, strings.ToLower(email))
}

// ---------- Users ----------

type adminUser struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	IsGuest   bool      `json:"isGuest"`
	Balance   int64     `json:"balance"`
	Spent     int64     `json:"spent"`  // total pemakaian AI (setelah refund)
	TopUp     int64     `json:"topUp"`  // total top up
	AIHits    int64     `json:"aiHits"` // jumlah generate AI
	CreatedAt time.Time `json:"createdAt"`
}

// GET /api/admin/users?q=&guests=1&page=1 — daftar semua user beserta saldonya.
func (s *Server) adminListUsers(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	withGuests := r.URL.Query().Get("guests") == "1"
	page := max(atoi(r.URL.Query().Get("page")), 1)
	const size = 50
	where := `WHERE ($1 = '' OR u.email ILIKE '%' || $1 || '%' OR u.name ILIKE '%' || $1 || '%') AND ($2 OR NOT u.is_guest)`
	var total int64
	if err := s.DB.QueryRowContext(r.Context(), `SELECT count(*) FROM users u `+where, q, withGuests).Scan(&total); err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), `
		SELECT u.id, u.email, u.name, u.role, u.is_guest, u.balance, u.created_at,
			COALESCE((SELECT -sum(amount) FROM wallet_transactions t WHERE t.user_id=u.id AND t.kind IN ('charge','refund')), 0),
			COALESCE((SELECT sum(amount) FROM wallet_transactions t WHERE t.user_id=u.id AND t.kind='topup'), 0),
			(SELECT count(*) FROM ai_generations g WHERE g.user_id=u.id)
		FROM users u `+where+`
		ORDER BY u.created_at DESC LIMIT $3 OFFSET $4`, q, withGuests, size, (page-1)*size)
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	defer rows.Close()
	out := []adminUser{}
	for rows.Next() {
		var u adminUser
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.IsGuest, &u.Balance, &u.CreatedAt, &u.Spent, &u.TopUp, &u.AIHits); err == nil {
			out = append(out, u)
		}
	}
	httpx.JSON(w, 200, map[string]any{"items": out, "total": total, "page": page, "size": size})
}

// POST /api/admin/users/{id}/balance — tambah saldo user (tahap testing, sebelum ada
// payment gateway). amount negatif = koreksi saldo (tidak bisa membuat saldo minus).
func (s *Server) adminAdjustBalance(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Amount int64  `json:"amount"`
		Note   string `json:"note"`
	}
	if err := httpx.Decode(r, &req, 1<<14); err != nil || req.Amount == 0 {
		httpx.Error(w, 400, "jumlah saldo wajib diisi (bukan 0)")
		return
	}
	if req.Amount > 100_000_000 || req.Amount < -100_000_000 {
		httpx.Error(w, 400, "jumlah maksimal Rp 100.000.000 per transaksi")
		return
	}
	userID := r.PathValue("id")
	kind, desc := billing.KindTopup, "Top up saldo oleh admin"
	if req.Amount < 0 {
		kind, desc = billing.KindAdjust, "Koreksi saldo oleh admin"
	}
	if note := strings.TrimSpace(req.Note); note != "" {
		desc += ": " + truncate(note, 200)
	}
	admin := httpx.CurrentUser(r)
	activity(r, "target_user_id", userID)
	activity(r, "amount", req.Amount)
	bal, err := billing.Apply(r.Context(), s.DB, billing.Tx{
		UserID: userID, Amount: req.Amount, Kind: kind, Description: desc,
		RequestID: httpx.RequestID(r.Context()), ActorID: admin.ID,
	})
	if errors.Is(err, billing.ErrInsufficient) {
		httpx.Error(w, 400, "koreksi melebihi saldo user")
		return
	}
	if err != nil {
		httpx.Error(w, 400, err.Error())
		return
	}
	activity(r, "balance_after", bal)
	httpx.JSON(w, 200, map[string]any{"balance": bal})
}

// PATCH /api/admin/users/{id} — ubah role (user/admin).
func (s *Server) adminUpdateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Role string `json:"role"`
	}
	if err := httpx.Decode(r, &req, 1<<12); err != nil || (req.Role != RoleUser && req.Role != RoleAdmin) {
		httpx.Error(w, 400, "role harus user atau admin")
		return
	}
	id := r.PathValue("id")
	if id == httpx.CurrentUser(r).ID && req.Role != RoleAdmin {
		httpx.Error(w, 400, "tidak bisa mencabut akses admin akun sendiri")
		return
	}
	activity(r, "target_user_id", id)
	activity(r, "role", req.Role)
	res, err := s.DB.ExecContext(r.Context(), `UPDATE users SET role=$2 WHERE id=$1 AND NOT is_guest`, id, req.Role)
	if n, _ := rowsAffected(res); err != nil || n == 0 {
		httpx.Error(w, 404, "user tidak ditemukan (akun tamu tidak bisa jadi admin)")
		return
	}
	httpx.JSON(w, 200, map[string]bool{"ok": true})
}

// GET /api/admin/users/{id}/transactions — riwayat saldo satu user.
func (s *Server) adminUserTx(w http.ResponseWriter, r *http.Request) {
	txs, err := s.listWalletTx(r.Context(), r.PathValue("id"), 200)
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, txs)
}

// ---------- Konfigurasi AI ----------

func (s *Server) aiConfigResponse(ctx context.Context) map[string]any {
	return map[string]any{
		"configured":  s.AI.Configured(),
		"saved":       s.loadAIConfig(ctx), // tersimpan di database
		"active":      s.activeModels(ctx), // yang benar-benar dipakai (fallback ke default env)
		"textModels":  nonNil(s.AI.TextChoices()),
		"imageModels": nonNil(s.AI.ImageChoices()),
		"editModels":  nonNil(s.AI.EditChoices()),
		"labels":      s.AI.Labels(),
		"defaults":    map[string]string{"textModel": s.AI.DefaultText(), "imageModel": s.AI.DefaultImage()},
	}
}

// GET /api/admin/ai-config
func (s *Server) adminGetAIConfig(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, 200, s.aiConfigResponse(r.Context()))
}

// PUT /api/admin/ai-config — pilih model untuk teks, gambar, dan edit gambar.
// API key provider tetap di env (tidak disimpan di database / tidak lewat browser).
func (s *Server) adminPutAIConfig(w http.ResponseWriter, r *http.Request) {
	var c aiConfig
	if err := httpx.Decode(r, &c, 1<<14); err != nil {
		httpx.Error(w, 400, "body tidak valid")
		return
	}
	check := func(v string, choices []string, name string) error {
		if v != "" && !slices.Contains(choices, v) {
			return fmt.Errorf("model %s %q tidak tersedia", name, v)
		}
		return nil
	}
	for _, err := range []error{
		check(c.TextModel, s.AI.TextChoices(), "teks"),
		check(c.ImageModel, s.AI.ImageChoices(), "gambar"),
		check(c.EditModel, s.AI.EditChoices(), "edit gambar"),
	} {
		if err != nil {
			httpx.Error(w, 400, err.Error())
			return
		}
	}
	activity(r, "config", c)
	if err := s.saveSetting(r.Context(), "ai", c, httpx.CurrentUser(r).ID); err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, s.aiConfigResponse(r.Context()))
}

// ---------- Konfigurasi harga ----------

type priceRow struct {
	Model         string   `json:"model"`
	Label         string   `json:"label"`
	Kinds         []string `json:"kinds"` // text | image | edit
	Active        []string `json:"active"`
	VendorPrice   float64  `json:"vendorPrice"`
	MarkupPercent *float64 `json:"markupPercent"`
	UserPrice     int64    `json:"userPrice"`
	IsSet         bool     `json:"isSet"`     // harga vendor sudah diatur admin
	Available     bool     `json:"available"` // provider model ini aktif
}

func (s *Server) pricingResponse(ctx context.Context) (map[string]any, error) {
	p, err := s.pricing(ctx)
	if err != nil {
		return nil, err
	}
	labels := s.AI.Labels()
	active := s.activeModels(ctx)
	rows := map[string]*priceRow{}
	var order []string
	add := func(model, kind string, available bool) {
		row := rows[model]
		if row == nil {
			label := labels[model]
			if label == "" {
				label = model
			}
			row = &priceRow{Model: model, Label: label, Kinds: []string{}, Active: []string{}, Available: available}
			rows[model] = row
			order = append(order, model)
		}
		if kind != "" && !slices.Contains(row.Kinds, kind) {
			row.Kinds = append(row.Kinds, kind)
		}
	}
	for _, m := range s.AI.TextChoices() {
		add(m, "text", true)
	}
	for _, m := range s.AI.ImageChoices() {
		add(m, "image", true)
	}
	for _, m := range s.AI.EditChoices() {
		add(m, "edit", true)
	}
	// Harga model yang providernya sudah tidak aktif tetap ditampilkan.
	for m := range p.Models {
		add(m, "", false)
	}
	for kind, m := range map[string]string{"text": active.TextModel, "image": active.ImageModel, "edit": active.EditModel} {
		if row := rows[m]; row != nil {
			row.Active = append(row.Active, kind)
			slices.Sort(row.Active)
		}
	}
	out := make([]priceRow, 0, len(order))
	for _, m := range order {
		row := rows[m]
		if mp, ok := p.Models[m]; ok {
			row.VendorPrice, row.MarkupPercent = mp.VendorPrice, mp.MarkupPercent
		}
		row.IsSet = p.IsSet(m)
		row.UserPrice = p.UserPrice(m)
		out = append(out, *row)
	}
	return map[string]any{
		"markupPercent": p.MarkupPercent,
		"freeHitPrice":  p.FreeHitPrice,
		"models":        out,
		"features":      s.featurePrices(ctx),
	}, nil
}

// GET /api/admin/pricing
func (s *Server) adminGetPricing(w http.ResponseWriter, r *http.Request) {
	res, err := s.pricingResponse(r.Context())
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, res)
}

// PUT /api/admin/pricing — atur markup global, tarif model gratis, dan harga vendor per model.
// Model dengan "remove": true dihapus dari daftar harga (kembali ke tarif model gratis).
func (s *Server) adminPutPricing(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MarkupPercent *float64 `json:"markupPercent"`
		FreeHitPrice  *int64   `json:"freeHitPrice"`
		Models        []struct {
			Model         string   `json:"model"`
			VendorPrice   float64  `json:"vendorPrice"`
			MarkupPercent *float64 `json:"markupPercent"`
			Remove        bool     `json:"remove"`
		} `json:"models"`
	}
	if err := httpx.Decode(r, &req, 1<<18); err != nil {
		httpx.Error(w, 400, "body tidak valid")
		return
	}
	ctx := r.Context()
	cur, err := s.pricing(ctx)
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	set := cur.Settings
	if req.MarkupPercent != nil {
		set.MarkupPercent = *req.MarkupPercent
	}
	if req.FreeHitPrice != nil {
		set.FreeHitPrice = *req.FreeHitPrice
	}
	if set.MarkupPercent < -100 || set.MarkupPercent > 1000 || set.FreeHitPrice < 0 {
		httpx.Error(w, 400, "markup harus -100..1000% dan tarif model gratis tidak boleh negatif")
		return
	}
	adminID := httpx.CurrentUser(r).ID
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	defer tx.Rollback()
	b, _ := json.Marshal(set)
	if _, err := tx.ExecContext(ctx, `INSERT INTO app_settings(key, value, updated_by, updated_at) VALUES('pricing',$1,$2,now())
		ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value, updated_by=EXCLUDED.updated_by, updated_at=now()`, b, adminID); err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	for _, m := range req.Models {
		m.Model = strings.TrimSpace(m.Model)
		if m.Model == "" {
			continue
		}
		if m.Remove {
			if _, err := tx.ExecContext(ctx, `DELETE FROM model_prices WHERE model=$1`, m.Model); err != nil {
				httpx.Error(w, 500, err.Error())
				return
			}
			continue
		}
		if m.VendorPrice < 0 || (m.MarkupPercent != nil && (*m.MarkupPercent < -100 || *m.MarkupPercent > 1000)) {
			httpx.Error(w, 400, fmt.Sprintf("harga %s tidak valid", m.Model))
			return
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO model_prices(model, vendor_price, markup_percent, updated_by, updated_at)
			VALUES($1,$2,$3,$4,now())
			ON CONFLICT (model) DO UPDATE SET vendor_price=EXCLUDED.vendor_price, markup_percent=EXCLUDED.markup_percent,
				updated_by=EXCLUDED.updated_by, updated_at=now()`, m.Model, m.VendorPrice, m.MarkupPercent, adminID); err != nil {
			httpx.Error(w, 500, err.Error())
			return
		}
	}
	if err := tx.Commit(); err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	activity(r, "pricing", req)
	res, err := s.pricingResponse(ctx)
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, res)
}

// ---------- System log ----------

// GET /api/admin/logs?type=activity|vendor&q=&action=&userId=&success=true|false&from=&to=&page=
func (s *Server) adminLogs(w http.ResponseWriter, r *http.Request) {
	v := r.URL.Query()
	q := applog.Query{
		Kind:   v.Get("type"),
		Text:   v.Get("q"),
		Action: v.Get("action"),
		UserID: v.Get("userId"),
		Page:   atoi(v.Get("page")),
		Size:   50,
	}
	if b, err := strconv.ParseBool(v.Get("success")); err == nil {
		q.Success = &b
	}
	if t, err := time.Parse(time.RFC3339, v.Get("from")); err == nil {
		q.From = t
	}
	if t, err := time.Parse(time.RFC3339, v.Get("to")); err == nil {
		q.To = t
	}
	res, err := s.Log.Search(r.Context(), q)
	if errors.Is(err, applog.ErrDisabled) {
		httpx.JSON(w, 200, map[string]any{"enabled": false, "error": err.Error(), "total": 0, "items": []any{}})
		return
	}
	if err != nil {
		log.Printf("cari log: %v", err)
		httpx.Error(w, 502, "gagal membaca log dari Elasticsearch: "+err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"enabled": true, "total": res.Total, "items": res.Items})
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
