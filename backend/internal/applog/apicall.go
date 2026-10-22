package applog

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"strings"
	"sync/atomic"
	"time"
)

// APICalls menyimpan AI API Call Log (setiap request HTTP ke vendor AI) ke tabel
// ai_api_calls di PostgreSQL, pola sama dengan tiara: penulisan selalu asinkron lewat
// antrean berkapasitas tetap — request user tidak pernah menunggu database, dan kalau
// antrean penuh baris log dibuang (dicatat di log server). Berbeda dengan activity/vendor
// log di Elasticsearch, log ini selalu tersedia tanpa konfigurasi tambahan.
type APICalls struct {
	db      *sql.DB
	ch      chan APICall
	dropped atomic.Int64
	done    chan struct{}
}

// APICall: satu baris ai_api_calls.
type APICall struct {
	Provider         string
	Endpoint         string
	Model            string
	Operation        string
	Feature          string
	UserID           string
	RequestID        string
	HTTPStatus       int
	OK               bool
	Latency          time.Duration
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	Cost             *float64
	Error            string
	Request          any
	Response         any
	CreatedAt        time.Time
}

const maxBodyBytes = 64 << 10

func NewAPICalls(db *sql.DB) *APICalls {
	return &APICalls{db: db, ch: make(chan APICall, 1000), done: make(chan struct{})}
}

// Push memasukkan baris ke antrean; tidak pernah memblokir.
func (a *APICalls) Push(c APICall) {
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now()
	}
	select {
	case a.ch <- c:
	default:
		if n := a.dropped.Add(1); n%100 == 1 {
			log.Printf("applog: antrean AI API Call Log penuh, %d baris dibuang", n)
		}
	}
}

// Run menulis antrean ke database sampai ctx selesai, lalu menulis sisanya. Panggil di goroutine.
func (a *APICalls) Run(ctx context.Context) {
	defer close(a.done)
	for {
		select {
		case c := <-a.ch:
			a.insert(c)
		case <-ctx.Done():
			for {
				select {
				case c := <-a.ch:
					a.insert(c)
				default:
					return
				}
			}
		}
	}
}

// Wait menunggu Run selesai menulis sisa antrean (dipakai saat shutdown).
func (a *APICalls) Wait(timeout time.Duration) {
	select {
	case <-a.done:
	case <-time.After(timeout):
	}
}

func (a *APICalls) insert(c APICall) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var status, prompt, completion, total, userID any
	if c.HTTPStatus > 0 {
		status = c.HTTPStatus
	}
	if c.PromptTokens > 0 || c.CompletionTokens > 0 || c.TotalTokens > 0 {
		prompt, completion, total = c.PromptTokens, c.CompletionTokens, c.TotalTokens
	}
	if c.UserID != "" {
		userID = c.UserID
	}
	// Harga vendor (Rupiah per hit, menu Konfigurasi harga) diambil langsung saat insert,
	// hanya untuk panggilan yang sukses.
	_, err := a.db.ExecContext(ctx, `INSERT INTO ai_api_calls (provider, endpoint, model, operation, feature, user_id, request_id,
			http_status, is_success, latency_ms, prompt_tokens, completion_tokens, total_tokens, cost, vendor_price,
			error_message, request_body, response_body, created_at)
		VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,NULLIF($7,''),$8,$9,$10,$11,$12,$13,$14,
			CASE WHEN $9 THEN (SELECT vendor_price FROM model_prices WHERE model = $1::text || ':' || $3::text) END,
			NULLIF($15,''),$16::jsonb,$17::jsonb,$18)`,
		c.Provider, c.Endpoint, c.Model, c.Operation, c.Feature, userID, c.RequestID,
		status, c.OK, c.Latency.Milliseconds(), prompt, completion, total, c.Cost,
		truncate(c.Error, 4000), jsonParam(c.Request), jsonParam(c.Response), c.CreatedAt)
	if err != nil {
		// Sengaja hanya ke log server, bukan ke tabel ini lagi.
		log.Printf("applog: gagal menyimpan AI API Call Log: %v", err)
	}
}

// jsonParam: nilai untuk kolom JSONB; nil → NULL. Body yang terlalu besar diganti ringkasan.
func jsonParam(v any) any {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		b, _ = json.Marshal(map[string]string{"marshal_error": err.Error()})
	}
	if len(b) > maxBodyBytes {
		b, _ = json.Marshal(map[string]any{"truncated": true, "bytes": len(b), "preview": strings.ToValidUTF8(string(b[:maxBodyBytes]), "")})
	}
	// JSONB menolak karakter NUL
	return strings.ReplaceAll(string(b), `\u0000`, "")
}

// StartRetention menghapus baris AI API Call Log yang lebih tua dari days hari, saat start
// lalu setiap 6 jam (days <= 0 = simpan selamanya).
func (a *APICalls) StartRetention(ctx context.Context, days int) {
	if days <= 0 {
		return
	}
	clean := func() {
		cctx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		if _, err := a.db.ExecContext(cctx, `DELETE FROM ai_api_calls WHERE created_at < now() - make_interval(days => $1)`, days); err != nil {
			log.Printf("applog: retensi AI API Call Log gagal: %v", err)
		}
	}
	go func() {
		clean()
		t := time.NewTicker(6 * time.Hour)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				clean()
			}
		}
	}()
}
