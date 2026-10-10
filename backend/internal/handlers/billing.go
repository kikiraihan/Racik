package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kikiraihan/racik/backend/internal/billing"
	"github.com/kikiraihan/racik/backend/internal/httpx"
)

// ---------- Konfigurasi AI (diatur admin) ----------

// aiConfig: model yang dipakai fitur AI. User tidak bisa memilih model dan tidak pernah
// melihat provider/model apa yang dipakai — semuanya diatur admin.
type aiConfig struct {
	TextModel  string `json:"textModel"`  // layout desain, perbaiki desain, copywriting
	ImageModel string `json:"imageModel"` // generate gambar
	EditModel  string `json:"editModel"`  // edit gambar
}

func (s *Server) loadAIConfig(ctx context.Context) aiConfig {
	var c aiConfig
	var raw []byte
	if err := s.DB.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key='ai'`).Scan(&raw); err == nil {
		_ = json.Unmarshal(raw, &c)
	}
	return c
}

// activeModels: konfigurasi admin yang sudah divalidasi terhadap provider aktif. Model yang
// tidak (lagi) tersedia diganti default dari env.
func (s *Server) activeModels(ctx context.Context) aiConfig {
	c := s.loadAIConfig(ctx)
	if !slices.Contains(s.AI.TextChoices(), c.TextModel) {
		c.TextModel = s.AI.DefaultText()
	}
	if !slices.Contains(s.AI.ImageChoices(), c.ImageModel) {
		c.ImageModel = s.AI.DefaultImage()
	}
	if edits := s.AI.EditChoices(); !slices.Contains(edits, c.EditModel) {
		// Model gambar kalau providernya bisa edit (kenari, OpenRouter); kalau tidak, model
		// edit pertama yang tersedia.
		c.EditModel = c.ImageModel
		if !slices.Contains(edits, c.EditModel) && len(edits) > 0 {
			c.EditModel = edits[0]
		}
	}
	return c
}

func (s *Server) saveSetting(ctx context.Context, key string, v any, actorID string) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx, `INSERT INTO app_settings(key, value, updated_by, updated_at) VALUES($1,$2,$3,now())
		ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value, updated_by=EXCLUDED.updated_by, updated_at=now()`, key, b, actorID)
	return err
}

// ---------- Tarif ----------

func (s *Server) pricing(ctx context.Context) (billing.Pricing, error) {
	return billing.Load(ctx, s.DB, billing.Settings{MarkupPercent: s.Cfg.DefaultMarkupPercent, FreeHitPrice: s.Cfg.DefaultFreeHitPrice})
}

// featurePrices: harga per hit tiap jenis fitur AI untuk ditampilkan ke user (tanpa nama model).
func (s *Server) featurePrices(ctx context.Context) map[string]int64 {
	p, err := s.pricing(ctx)
	if err != nil {
		log.Printf("load pricing: %v", err)
	}
	m := s.activeModels(ctx)
	return map[string]int64{
		"text":  p.UserPrice(m.TextModel),  // per desain/perbaikan/copywriting (1x susun layout/teks)
		"image": p.UserPrice(m.ImageModel), // per gambar yang berhasil dibuat
		"edit":  p.UserPrice(m.EditModel),  // per gambar yang diedit
	}
}

func rupiah(n int64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	s := strconv.FormatInt(n, 10)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	if neg {
		return "-Rp " + b.String()
	}
	return "Rp " + b.String()
}

// ---------- Pemotongan saldo per request AI ----------

// charge mengelola saldo satu request AI: saldo dipesan (dipotong) di depan sebelum
// memanggil vendor, lalu bagian yang tidak terpakai (vendor gagal, gambar gagal dibuat)
// dikembalikan saat settle. User hanya membayar hasil yang benar-benar diterima.
type charge struct {
	s        *Server
	userID   string
	feature  string
	reqID    string
	mu       sync.Mutex
	reserved int64
	used     int64
	balance  int64
}

func (s *Server) newCharge(r *http.Request, feature string) *charge {
	return &charge{s: s, userID: httpx.CurrentUser(r).ID, feature: feature, reqID: httpx.RequestID(r.Context())}
}

// reserve memotong saldo sebesar amount. Gagal dengan billing.ErrInsufficient.
func (c *charge) reserve(ctx context.Context, amount int64, desc string) error {
	if amount <= 0 {
		return nil
	}
	bal, err := billing.Apply(context.WithoutCancel(ctx), c.s.DB, billing.Tx{
		UserID: c.userID, Amount: -amount, Kind: billing.KindCharge, Feature: c.feature, Description: desc, RequestID: c.reqID,
	})
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.reserved += amount
	c.balance = bal
	c.mu.Unlock()
	return nil
}

// reserveUpTo memesan saldo untuk sebanyak mungkin dari n item seharga unit (mis. gambar
// AI dalam satu desain) dan mengembalikan jumlah yang berhasil dipesan.
func (c *charge) reserveUpTo(ctx context.Context, n int, unit int64, desc func(k int) string) int {
	if unit <= 0 {
		return n
	}
	bal, err := billing.Balance(ctx, c.s.DB, c.userID)
	if err != nil {
		return 0
	}
	for k := min(n, int(bal/unit)); k > 0; k-- {
		err := c.reserve(ctx, int64(k)*unit, desc(k))
		if err == nil {
			return k
		}
		if !errors.Is(err, billing.ErrInsufficient) {
			log.Printf("reserve saldo: %v", err)
			return 0
		}
	}
	return 0
}

func (c *charge) use(amount int64) {
	c.mu.Lock()
	c.used += amount
	c.mu.Unlock()
}

// settle mengembalikan saldo yang dipesan tapi tidak terpakai. Dipanggil dengan defer.
func (c *charge) settle(ctx context.Context, r *http.Request) {
	c.mu.Lock()
	refund := c.reserved - c.used
	used := c.used
	c.mu.Unlock()
	if refund > 0 {
		bal, err := billing.Apply(context.WithoutCancel(ctx), c.s.DB, billing.Tx{
			UserID: c.userID, Amount: refund, Kind: billing.KindRefund, Feature: c.feature,
			Description: "Pengembalian saldo: proses AI gagal / tidak terpakai", RequestID: c.reqID,
		})
		if err != nil {
			// Saldo tetap terpotong — tercatat di log supaya admin bisa mengembalikan manual.
			log.Printf("REFUND GAGAL user=%s request=%s jumlah=%d: %v", c.userID, c.reqID, refund, err)
		} else {
			c.balance = bal
		}
	}
	activity(r, "charged", used)
	if refund > 0 {
		activity(r, "refunded", refund)
	}
	activity(r, "balance_after", c.balance)
}

// insufficient menjawab 402 dengan info biaya & saldo supaya UI bisa mengarahkan ke top up.
func (s *Server) insufficient(w http.ResponseWriter, r *http.Request, need int64) {
	bal, _ := billing.Balance(r.Context(), s.DB, httpx.CurrentUser(r).ID)
	activity(r, "need", need)
	httpx.JSON(w, http.StatusPaymentRequired, map[string]any{
		"error":   fmt.Sprintf("Saldo tidak cukup: biaya %s, saldo kamu %s. Silakan top up saldo.", rupiah(need), rupiah(bal)),
		"code":    "insufficient_balance",
		"need":    need,
		"balance": bal,
	})
}

// ---------- Endpoint user: saldo ----------

type walletTx struct {
	ID           string    `json:"id"`
	Amount       int64     `json:"amount"`
	BalanceAfter int64     `json:"balanceAfter"`
	Kind         string    `json:"kind"`
	Feature      string    `json:"feature"`
	Description  string    `json:"description"`
	CreatedAt    time.Time `json:"createdAt"`
}

func (s *Server) listWalletTx(ctx context.Context, userID string, limit int) ([]walletTx, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, amount, balance_after, kind, feature, description, created_at
		FROM wallet_transactions WHERE user_id=$1 ORDER BY created_at DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []walletTx{}
	for rows.Next() {
		var t walletTx
		if err := rows.Scan(&t.ID, &t.Amount, &t.BalanceAfter, &t.Kind, &t.Feature, &t.Description, &t.CreatedAt); err == nil {
			out = append(out, t)
		}
	}
	return out, rows.Err()
}

// GET /api/wallet — saldo, tarif fitur AI, dan riwayat transaksi user.
func (s *Server) wallet(w http.ResponseWriter, r *http.Request) {
	u := httpx.CurrentUser(r)
	bal, err := billing.Balance(r.Context(), s.DB, u.ID)
	if err != nil {
		httpx.Error(w, 401, "user tidak ditemukan")
		return
	}
	txs, err := s.listWalletTx(r.Context(), u.ID, 100)
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{
		"balance":            bal,
		"prices":             s.featurePrices(r.Context()),
		"maxImagesPerDesign": maxImagesPerDesign,
		"transactions":       txs,
	})
}
