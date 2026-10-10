// Package billing mengelola saldo user (Rupiah) dan tarif fitur AI.
//
// Setiap perubahan saldo tercatat di wallet_transactions. Pemotongan dilakukan atomik
// (UPDATE ... WHERE balance >= biaya) sehingga request paralel tidak bisa membuat saldo minus.
package billing

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
)

const (
	KindTopup  = "topup"
	KindCharge = "charge"
	KindRefund = "refund"
	KindAdjust = "adjust"
)

var ErrInsufficient = errors.New("saldo tidak cukup")

// Settings: tarif global, diatur admin di menu Konfigurasi Harga.
type Settings struct {
	MarkupPercent float64 `json:"markupPercent"` // harga user = harga vendor + markup %
	FreeHitPrice  int64   `json:"freeHitPrice"`  // tarif per hit untuk model gratis di vendor
}

// ModelPrice: harga vendor satu model per hit (1 panggilan teks / 1 gambar).
type ModelPrice struct {
	Model         string   `json:"model"`
	VendorPrice   float64  `json:"vendorPrice"`
	MarkupPercent *float64 `json:"markupPercent"` // nil = markup global
}

// Pricing: tarif lengkap yang dipakai saat menghitung biaya.
type Pricing struct {
	Settings
	Models map[string]ModelPrice
}

// UserPrice: harga per hit yang dibebankan ke user untuk satu model (Rupiah, dibulatkan ke
// atas). Model gratis (harga vendor 0 / belum diatur) dikenai tarif flat FreeHitPrice.
func (p Pricing) UserPrice(model string) int64 {
	mp, ok := p.Models[model]
	if !ok || mp.VendorPrice <= 0 {
		return max(p.FreeHitPrice, 0)
	}
	markup := p.MarkupPercent
	if mp.MarkupPercent != nil {
		markup = *mp.MarkupPercent
	}
	// Dibulatkan 2 desimal dulu supaya galat float (mis. 100*1.05 = 105.0000001) tidak
	// menaikkan harga 1 rupiah.
	v := math.Round(mp.VendorPrice*(1+markup/100)*100) / 100
	return int64(math.Ceil(v))
}

// IsSet: harga vendor model sudah diatur admin.
func (p Pricing) IsSet(model string) bool {
	_, ok := p.Models[model]
	return ok
}

// Load membaca tarif dari database; nilai default dipakai kalau admin belum mengatur.
func Load(ctx context.Context, db *sql.DB, def Settings) (Pricing, error) {
	p := Pricing{Settings: def, Models: map[string]ModelPrice{}}
	var raw []byte
	err := db.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key='pricing'`).Scan(&raw)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return p, err
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &p.Settings)
	}
	rows, err := db.QueryContext(ctx, `SELECT model, vendor_price, markup_percent FROM model_prices`)
	if err != nil {
		return p, err
	}
	defer rows.Close()
	for rows.Next() {
		var mp ModelPrice
		if err := rows.Scan(&mp.Model, &mp.VendorPrice, &mp.MarkupPercent); err == nil {
			p.Models[mp.Model] = mp
		}
	}
	return p, rows.Err()
}

// Tx: satu baris buku besar saldo.
type Tx struct {
	UserID      string
	Amount      int64 // + masuk, - keluar
	Kind        string
	Feature     string
	Description string
	RequestID   string
	ActorID     string // admin yang melakukan top up / adjust
}

// Apply mengubah saldo secara atomik dan mencatat transaksinya. Untuk amount negatif,
// gagal dengan ErrInsufficient kalau saldo tidak cukup. Mengembalikan saldo setelahnya.
func Apply(ctx context.Context, db *sql.DB, t Tx) (int64, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var bal int64
	err = tx.QueryRowContext(ctx,
		`UPDATE users SET balance = balance + $2 WHERE id=$1 AND balance + $2 >= 0 RETURNING balance`,
		t.UserID, t.Amount).Scan(&bal)
	if errors.Is(err, sql.ErrNoRows) {
		var exists bool
		if db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1)`, t.UserID).Scan(&exists); !exists {
			return 0, errors.New("user tidak ditemukan")
		}
		return 0, ErrInsufficient
	}
	if err != nil {
		return 0, err
	}
	var actor any
	if t.ActorID != "" {
		actor = t.ActorID
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO wallet_transactions
		(user_id, amount, balance_after, kind, feature, description, request_id, actor_id)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8)`,
		t.UserID, t.Amount, bal, t.Kind, t.Feature, t.Description, t.RequestID, actor); err != nil {
		return 0, err
	}
	return bal, tx.Commit()
}

func Balance(ctx context.Context, db *sql.DB, userID string) (int64, error) {
	var b int64
	err := db.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1`, userID).Scan(&b)
	return b, err
}
