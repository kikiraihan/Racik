package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/kikiraihan/racik/backend/internal/ai"
	"github.com/kikiraihan/racik/backend/internal/applog"
	"github.com/kikiraihan/racik/backend/internal/httpx"
)

// track membungkus endpoint yang aktivitasnya dicatat ke activity log (Elasticsearch).
// Hanya fitur yang riskan: auth, hit model AI, dan aksi admin.
func (s *Server) track(action string, h http.HandlerFunc) http.HandlerFunc {
	return httpx.Track(action, s.logActivity, h)
}

func (s *Server) logActivity(r *http.Request, res httpx.ActivityResult) {
	userID, email, details := res.Activity.Snapshot()
	e := applog.Event{
		"request_id":  httpx.RequestID(r.Context()),
		"action":      res.Activity.Action,
		"method":      r.Method,
		"path":        r.URL.Path,
		"status":      res.Status,
		"success":     res.Status < 400,
		"duration_ms": res.Duration.Milliseconds(),
		"ip":          httpx.ClientIP(r),
		"user_agent":  applog.Truncate(r.UserAgent(), 300),
	}
	if userID != "" {
		e["user_id"] = userID
	}
	if email != "" {
		e["email"] = email
	}
	if res.Error != "" {
		e["error"] = applog.Truncate(res.Error, 500)
	}
	if len(details) > 0 {
		e["details"] = details
	}
	s.Log.Activity(e)
}

// activity menambah detail ke activity log request ini (kalau endpoint-nya di-track).
func activity(r *http.Request, k string, v any) {
	if a := httpx.ActivityFrom(r.Context()); a != nil {
		a.Set(k, v)
	}
}

// logVendorCall dipasang sebagai observer router AI: setiap request ke vendor dicatat
// lengkap (provider & model asli, durasi, error) — info yang tidak pernah dikirim ke user.
func (s *Server) logVendorCall(ctx context.Context, c ai.Call) {
	e := applog.Event{
		"request_id":   httpx.RequestID(ctx),
		"provider":     c.Provider,
		"model":        c.Model,
		"model_id":     c.Choice,
		"operation":    c.Operation,
		"duration_ms":  c.Duration.Milliseconds(),
		"success":      c.Err == nil,
		"prompt":       applog.Truncate(c.Prompt, 1000),
		"output_bytes": c.OutBytes,
	}
	if u := httpx.UserFrom(ctx); u.ID != "" {
		e["user_id"] = u.ID
		e["email"] = u.Email
	}
	if a := httpx.ActivityFrom(ctx); a != nil {
		e["feature"] = a.Action
	}
	if c.Err != nil {
		e["error"] = applog.Truncate(c.Err.Error(), 1000)
		var ae *ai.APIError
		if errors.As(c.Err, &ae) {
			e["status"] = ae.Status
			if ae.Code != "" {
				e["error_code"] = ae.Code
			}
		}
		if errors.Is(c.Err, context.Canceled) || errors.Is(c.Err, context.DeadlineExceeded) {
			e["error_code"] = "timeout_or_canceled"
		}
	}
	// Harga vendor per hit (dari menu Konfigurasi Harga) untuk rekap biaya vendor.
	if p, err := s.pricing(context.WithoutCancel(ctx)); err == nil {
		if mp, ok := p.Models[c.Choice]; ok {
			e["vendor_price"] = mp.VendorPrice
		}
		if c.Err == nil {
			e["user_price"] = p.UserPrice(c.Choice)
		}
	}
	s.Log.Vendor(e)
}
