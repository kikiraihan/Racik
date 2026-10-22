package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kikiraihan/racik/backend/internal/ai"
	"github.com/kikiraihan/racik/backend/internal/applog"
	"github.com/kikiraihan/racik/backend/internal/httpx"
)

// Panel admin → AI API Call Log (pola sama dengan tiara): setiap request HTTP ke vendor AI
// tersimpan di tabel ai_api_calls. Ditulis lewat recordAICall (dipasang sebagai
// ai.Router.Recorder), di sini hanya baca, ringkasan, dan hapus.

// RecordCall (ai.CallRecorder): lengkapi record dengan fitur, user, dan request id dari
// context lalu masukkan ke antrean penulisan.
func (s *Server) RecordCall(ctx context.Context, c ai.CallRecord) {
	if s.AICalls == nil {
		return
	}
	row := applog.APICall{
		Provider: c.Provider, Endpoint: c.Endpoint, Model: c.Model, Operation: c.Operation,
		RequestID: httpx.RequestID(ctx), HTTPStatus: c.HTTPStatus, OK: c.OK, Latency: c.Latency,
		PromptTokens: c.PromptTokens, CompletionTokens: c.CompletionTokens, TotalTokens: c.TotalTokens,
		Cost: c.Cost, Error: c.Error, Request: c.Request, Response: c.Response,
	}
	if a := httpx.ActivityFrom(ctx); a != nil {
		row.Feature = a.Action
	}
	if u := httpx.UserFrom(ctx); u.ID != "" {
		row.UserID = u.ID
	}
	s.AICalls.Push(row)
}

type aiCallRow struct {
	ID               int64           `json:"id"`
	Provider         string          `json:"provider"`
	Endpoint         string          `json:"endpoint"`
	Model            string          `json:"model"`
	Operation        string          `json:"operation"`
	Feature          *string         `json:"feature"`
	UserID           *string         `json:"userId"`
	Email            *string         `json:"email"`
	RequestID        *string         `json:"requestId"`
	HTTPStatus       *int            `json:"httpStatus"`
	IsSuccess        bool            `json:"isSuccess"`
	LatencyMS        *int            `json:"latencyMs"`
	PromptTokens     *int            `json:"promptTokens"`
	CompletionTokens *int            `json:"completionTokens"`
	TotalTokens      *int            `json:"totalTokens"`
	Cost             *float64        `json:"cost"`
	VendorPrice      *float64        `json:"vendorPrice"`
	ErrorMessage     *string         `json:"errorMessage"`
	RequestBody      json.RawMessage `json:"requestBody,omitempty"`
	ResponseBody     json.RawMessage `json:"responseBody,omitempty"`
	CreatedAt        time.Time       `json:"createdAt"`
}

const aiCallCols = `c.id, c.provider, c.endpoint, c.model, c.operation, c.feature, c.user_id::text, u.email, c.request_id,
	c.http_status, c.is_success, c.latency_ms, c.prompt_tokens, c.completion_tokens, c.total_tokens, c.cost, c.vendor_price`

func (x *aiCallRow) scanArgs() []any {
	return []any{&x.ID, &x.Provider, &x.Endpoint, &x.Model, &x.Operation, &x.Feature, &x.UserID, &x.Email, &x.RequestID,
		&x.HTTPStatus, &x.IsSuccess, &x.LatencyMS, &x.PromptTokens, &x.CompletionTokens, &x.TotalTokens, &x.Cost, &x.VendorPrice}
}

// aiCallSort: kolom yang boleh dipakai mengurutkan (whitelist, bukan input mentah).
var aiCallSort = map[string]string{
	"id": "c.id", "createdAt": "c.created_at", "model": "c.model", "feature": "c.feature", "provider": "c.provider",
	"latencyMs": "c.latency_ms", "totalTokens": "c.total_tokens", "cost": "c.cost", "vendorPrice": "c.vendor_price", "httpStatus": "c.http_status",
}

// aiCallWhere menyusun filter dari query string; semua nilai lewat parameter $n.
func aiCallWhere(r *http.Request) (string, []any) {
	v := r.URL.Query()
	var conds []string
	var args []any
	add := func(cond string, val any) {
		args = append(args, val)
		conds = append(conds, strings.ReplaceAll(cond, "?", "$"+strconv.Itoa(len(args))))
	}
	if q := strings.TrimSpace(v.Get("q")); q != "" {
		add(`(c.model ILIKE '%' || ? || '%' OR c.feature ILIKE '%' || ? || '%' OR c.request_id = ? OR c.error_message ILIKE '%' || ? || '%' OR u.email ILIKE '%' || ? || '%')`, q)
	}
	for param, col := range map[string]string{"feature": "c.feature", "operation": "c.operation", "provider": "c.provider", "model": "c.model", "requestId": "c.request_id"} {
		if val := strings.TrimSpace(v.Get(param)); val != "" {
			add(col+` = ?`, val)
		}
	}
	if val := strings.TrimSpace(v.Get("userId")); val != "" {
		add(`c.user_id::text = ?`, val)
	}
	if b, err := strconv.ParseBool(v.Get("success")); err == nil {
		add(`c.is_success = ?`, b)
	}
	// from / to: tanggal YYYY-MM-DD (to inklusif sampai akhir hari) atau RFC3339
	if t, ok := parseDay(v.Get("from")); ok {
		add(`c.created_at >= ?`, t)
	}
	if t, ok := parseDay(v.Get("to")); ok {
		if len(v.Get("to")) == len("2006-01-02") {
			t = t.AddDate(0, 0, 1)
		}
		add(`c.created_at < ?`, t)
	}
	if len(conds) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

func parseDay(s string) (time.Time, bool) {
	if t, err := time.ParseInLocation("2006-01-02", s, time.Local); err == nil {
		return t, true
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// GET /api/admin/ai-calls — daftar log, terbaru dulu (bisa diurutkan lewat sort & dir).
func (s *Server) adminListAICalls(w http.ResponseWriter, r *http.Request) {
	v := r.URL.Query()
	page := max(atoi(v.Get("page")), 1)
	size := min(max(atoi(v.Get("size")), 1), 200)
	if v.Get("size") == "" {
		size = 50
	}
	order := "c.id DESC"
	if col, ok := aiCallSort[v.Get("sort")]; ok {
		dir := "DESC"
		if v.Get("dir") == "asc" {
			dir = "ASC"
		}
		order = col + " " + dir + " NULLS LAST, c.id DESC"
	}
	where, args := aiCallWhere(r)
	from := ` FROM ai_api_calls c LEFT JOIN users u ON u.id = c.user_id`
	var total int64
	if err := s.DB.QueryRowContext(r.Context(), `SELECT count(*)`+from+where, args...).Scan(&total); err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	n := len(args)
	rows, err := s.DB.QueryContext(r.Context(), `SELECT `+aiCallCols+`, LEFT(c.error_message, 300), c.created_at`+from+where+
		fmt.Sprintf(` ORDER BY %s LIMIT $%d OFFSET $%d`, order, n+1, n+2), append(args, size, (page-1)*size)...)
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	defer rows.Close()
	items := []aiCallRow{}
	for rows.Next() {
		var x aiCallRow
		if err := rows.Scan(append(x.scanArgs(), &x.ErrorMessage, &x.CreatedAt)...); err != nil {
			httpx.Error(w, 500, err.Error())
			return
		}
		items = append(items, x)
	}
	if err := rows.Err(); err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"items": items, "total": total, "page": page, "size": size,
		"bodies": s.Cfg.AICallLogBodies, "retentionDays": s.Cfg.AICallLogRetentionDays})
}

// GET /api/admin/ai-calls/{id} — detail satu panggilan beserta body request & response.
func (s *Server) adminGetAICall(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.Error(w, 404, "log tidak ditemukan")
		return
	}
	var x aiCallRow
	var req, resp *string
	err = s.DB.QueryRowContext(r.Context(), `SELECT `+aiCallCols+`, c.error_message, c.created_at, c.request_body::text, c.response_body::text
		FROM ai_api_calls c LEFT JOIN users u ON u.id = c.user_id WHERE c.id=$1`, id).
		Scan(append(x.scanArgs(), &x.ErrorMessage, &x.CreatedAt, &req, &resp)...)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, 404, "log tidak ditemukan")
		return
	}
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	if req != nil {
		x.RequestBody = json.RawMessage(*req)
	}
	if resp != nil {
		x.ResponseBody = json.RawMessage(*resp)
	}
	httpx.JSON(w, 200, x)
}

type aiCallSummary struct {
	Feature     *string   `json:"feature"`
	Provider    string    `json:"provider"`
	Model       string    `json:"model"`
	Operation   string    `json:"operation"`
	Calls       int64     `json:"calls"`
	Errors      int64     `json:"errors"`
	AvgMS       *float64  `json:"avgMs"`
	P95MS       *float64  `json:"p95Ms"`
	Tokens      *int64    `json:"tokens"`
	Cost        *float64  `json:"cost"`
	VendorPrice *float64  `json:"vendorPrice"`
	LastCall    time.Time `json:"lastCall"`
}

// GET /api/admin/ai-calls/summary?days=7 — ringkasan per fitur & model dalam rentang hari
// tertentu, untuk memantau biaya dan model mana yang paling sering dipanggil / gagal.
func (s *Server) adminAICallSummary(w http.ResponseWriter, r *http.Request) {
	days := atoi(r.URL.Query().Get("days"))
	if days < 1 || days > 365 {
		days = 7
	}
	rows, err := s.DB.QueryContext(r.Context(), `SELECT feature, provider, model, operation, count(*), count(*) FILTER (WHERE NOT is_success),
			avg(latency_ms)::float8, (percentile_cont(0.95) WITHIN GROUP (ORDER BY latency_ms))::float8,
			sum(total_tokens)::bigint, sum(cost)::float8, sum(vendor_price)::float8, max(created_at)
		FROM ai_api_calls WHERE created_at >= now() - make_interval(days => $1)
		GROUP BY feature, provider, model, operation ORDER BY count(*) DESC LIMIT 100`, days)
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	defer rows.Close()
	items := []aiCallSummary{}
	var total struct {
		Calls       int64    `json:"calls"`
		Errors      int64    `json:"errors"`
		Tokens      *int64   `json:"tokens"`
		Cost        *float64 `json:"cost"`
		VendorPrice *float64 `json:"vendorPrice"`
	}
	addInt := func(dst **int64, v *int64) {
		if v != nil {
			sum := *v + deref(*dst)
			*dst = &sum
		}
	}
	addFloat := func(dst **float64, v *float64) {
		if v != nil {
			sum := *v + deref(*dst)
			*dst = &sum
		}
	}
	for rows.Next() {
		var x aiCallSummary
		if err := rows.Scan(&x.Feature, &x.Provider, &x.Model, &x.Operation, &x.Calls, &x.Errors, &x.AvgMS, &x.P95MS,
			&x.Tokens, &x.Cost, &x.VendorPrice, &x.LastCall); err != nil {
			httpx.Error(w, 500, err.Error())
			return
		}
		total.Calls += x.Calls
		total.Errors += x.Errors
		addInt(&total.Tokens, x.Tokens)
		addFloat(&total.Cost, x.Cost)
		addFloat(&total.VendorPrice, x.VendorPrice)
		items = append(items, x)
	}
	if err := rows.Err(); err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"days": days, "items": items, "total": total})
}

func deref[T int64 | float64](p *T) T {
	if p == nil {
		return 0
	}
	return *p
}

// POST /api/admin/ai-calls/delete — {"ids": [...]} hapus baris terpilih, {"all": true} kosongkan log.
func (s *Server) adminDeleteAICalls(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []int64 `json:"ids"`
		All bool    `json:"all"`
	}
	if err := httpx.Decode(r, &req, 1<<20); err != nil {
		httpx.Error(w, 400, "body tidak valid")
		return
	}
	var res sql.Result
	var err error
	switch {
	case req.All:
		res, err = s.DB.ExecContext(r.Context(), `DELETE FROM ai_api_calls`)
	case len(req.IDs) > 0 && len(req.IDs) <= 1000:
		res, err = s.DB.ExecContext(r.Context(), `DELETE FROM ai_api_calls WHERE id = ANY($1)`, req.IDs)
	default:
		httpx.Error(w, 400, "pilih minimal satu log (maksimal 1000)")
		return
	}
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	n, _ := res.RowsAffected()
	activity(r, "deleted", n)
	activity(r, "all", req.All)
	httpx.JSON(w, 200, map[string]any{"deleted": n, "message": fmt.Sprintf("%d baris log dihapus", n)})
}
