package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/kikiraihan/racik/backend/internal/applog"
	"github.com/kikiraihan/racik/backend/internal/httpx"
)

// ---------- Ringkasan (dashboard admin) ----------

type overviewDay struct {
	Date    string `json:"date"`    // YYYY-MM-DD
	AIHits  int64  `json:"aiHits"`  // jumlah generate AI
	Revenue int64  `json:"revenue"` // pemakaian AI (charge - refund), Rupiah
	Signups int64  `json:"signups"` // akun permanen baru
}

type overviewTx struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Amount    int64     `json:"amount"`
	Kind      string    `json:"kind"`
	Feature   string    `json:"feature"`
	CreatedAt time.Time `json:"createdAt"`
}

// GET /api/admin/overview — angka ringkas untuk halaman Ringkasan panel admin.
func (s *Server) adminOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	out := map[string]any{}

	var users, guests, admins, newUsers7d, designs, assets int64
	var balance int64
	if err := s.DB.QueryRowContext(ctx, `SELECT
			count(*) FILTER (WHERE NOT is_guest),
			count(*) FILTER (WHERE is_guest),
			count(*) FILTER (WHERE role='admin'),
			count(*) FILTER (WHERE NOT is_guest AND created_at > now() - interval '7 days'),
			COALESCE(sum(balance), 0)
		FROM users`).Scan(&users, &guests, &admins, &newUsers7d, &balance); err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	_ = s.DB.QueryRowContext(ctx, `SELECT count(*) FROM designs`).Scan(&designs)
	_ = s.DB.QueryRowContext(ctx, `SELECT count(*) FROM assets`).Scan(&assets)

	var hitsToday, hits7d, liked, disliked int64
	_ = s.DB.QueryRowContext(ctx, `SELECT
			count(*) FILTER (WHERE created_at >= date_trunc('day', now())),
			count(*) FILTER (WHERE created_at > now() - interval '7 days'),
			count(*) FILTER (WHERE rating = 1),
			count(*) FILTER (WHERE rating = -1)
		FROM ai_generations`).Scan(&hitsToday, &hits7d, &liked, &disliked)

	// Pemakaian AI = charge (negatif) dikurangi refund (positif).
	var revenueToday, revenue30d, topup30d int64
	_ = s.DB.QueryRowContext(ctx, `SELECT
			COALESCE(-sum(amount) FILTER (WHERE kind IN ('charge','refund') AND created_at >= date_trunc('day', now())), 0),
			COALESCE(-sum(amount) FILTER (WHERE kind IN ('charge','refund') AND created_at > now() - interval '30 days'), 0),
			COALESCE(sum(amount) FILTER (WHERE kind IN ('topup','adjust') AND created_at > now() - interval '30 days'), 0)
		FROM wallet_transactions`).Scan(&revenueToday, &revenue30d, &topup30d)

	out["users"] = map[string]any{"total": users, "guests": guests, "admins": admins, "new7d": newUsers7d, "balance": balance}
	out["content"] = map[string]any{"designs": designs, "assets": assets}
	// Model aktif ditampilkan dengan label yang mudah dibaca (hanya untuk admin).
	active := s.activeModels(ctx)
	labels := s.AI.Labels()
	name := func(m string) string {
		if l, ok := labels[m]; ok {
			return l
		}
		return m
	}
	out["ai"] = map[string]any{"hitsToday": hitsToday, "hits7d": hits7d, "liked": liked, "disliked": disliked,
		"configured": s.AI.Configured(), "textModel": name(active.TextModel), "imageModel": name(active.ImageModel), "editModel": name(active.EditModel)}
	out["wallet"] = map[string]any{"revenueToday": revenueToday, "revenue30d": revenue30d, "topup30d": topup30d}

	out["daily"] = s.overviewDaily(ctx, 14)
	out["byFeature"] = s.overviewByFeature(ctx)
	out["recentTx"] = s.overviewRecentTx(ctx)
	out["system"] = map[string]any{
		"elasticsearch": s.Log.Enabled(),
		"googleLogin":   len(s.Cfg.GoogleClientIDs) > 0,
	}
	if s.ServerLog != nil {
		out["serverLog"] = s.ServerLog.Stats()
	}
	httpx.JSON(w, 200, out)
}

// overviewDaily: generate AI, pemakaian saldo, dan pendaftar per hari (n hari terakhir).
func (s *Server) overviewDaily(ctx context.Context, n int) []overviewDay {
	days := make([]overviewDay, n)
	idx := map[string]int{}
	today := time.Now()
	for i := 0; i < n; i++ {
		d := today.AddDate(0, 0, i-n+1).Format("2006-01-02")
		days[i] = overviewDay{Date: d}
		idx[d] = i
	}
	from := today.AddDate(0, 0, -n+1).Format("2006-01-02")
	scan := func(query string, set func(*overviewDay, int64)) {
		rows, err := s.DB.QueryContext(ctx, query, from)
		if err != nil {
			return
		}
		defer rows.Close()
		for rows.Next() {
			var d time.Time
			var v int64
			if rows.Scan(&d, &v) == nil {
				if i, ok := idx[d.Format("2006-01-02")]; ok {
					set(&days[i], v)
				}
			}
		}
	}
	scan(`SELECT date_trunc('day', created_at)::date, count(*) FROM ai_generations WHERE created_at >= $1::date GROUP BY 1`,
		func(d *overviewDay, v int64) { d.AIHits = v })
	scan(`SELECT date_trunc('day', created_at)::date, -sum(amount) FROM wallet_transactions
		WHERE kind IN ('charge','refund') AND created_at >= $1::date GROUP BY 1`,
		func(d *overviewDay, v int64) { d.Revenue = v })
	scan(`SELECT date_trunc('day', created_at)::date, count(*) FROM users WHERE NOT is_guest AND created_at >= $1::date GROUP BY 1`,
		func(d *overviewDay, v int64) { d.Signups = v })
	return days
}

// overviewByFeature: jumlah generate AI per fitur dalam 30 hari terakhir.
func (s *Server) overviewByFeature(ctx context.Context) []map[string]any {
	out := []map[string]any{}
	rows, err := s.DB.QueryContext(ctx, `SELECT kind, count(*) FROM ai_generations
		WHERE created_at > now() - interval '30 days' GROUP BY kind ORDER BY 2 DESC`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		var n int64
		if rows.Scan(&k, &n) == nil {
			out = append(out, map[string]any{"kind": k, "count": n})
		}
	}
	return out
}

// overviewRecentTx: 8 transaksi saldo terakhir semua user.
func (s *Server) overviewRecentTx(ctx context.Context) []overviewTx {
	out := []overviewTx{}
	rows, err := s.DB.QueryContext(ctx, `SELECT t.id, u.email, t.amount, t.kind, t.feature, t.created_at
		FROM wallet_transactions t JOIN users u ON u.id = t.user_id ORDER BY t.created_at DESC LIMIT 8`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var t overviewTx
		if rows.Scan(&t.ID, &t.Email, &t.Amount, &t.Kind, &t.Feature, &t.CreatedAt) == nil {
			out = append(out, t)
		}
	}
	return out
}

// ---------- Log server (in-memory) ----------

// GET /api/admin/server-logs?q=&level=info|warn|error&source=http|app|log&after=<seq>&limit=
// Baris log server terakhir dari memori (access log, peringatan, error) — tetap bisa dilihat
// walaupun Elasticsearch belum diatur.
func (s *Server) adminServerLogs(w http.ResponseWriter, r *http.Request) {
	if s.ServerLog == nil {
		httpx.JSON(w, 200, map[string]any{"enabled": false, "items": []any{}, "lastSeq": 0})
		return
	}
	v := r.URL.Query()
	after, _ := strconv.ParseInt(v.Get("after"), 10, 64)
	items, last := s.ServerLog.Lines(applog.RingQuery{
		Text: v.Get("q"), Level: v.Get("level"), Source: v.Get("source"), After: after, Limit: atoi(v.Get("limit")),
	})
	httpx.JSON(w, 200, map[string]any{"enabled": true, "items": items, "lastSeq": last, "stats": s.ServerLog.Stats()})
}
