package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// AI API Call Log (Panel admin → AI API Call Log, pola sama dengan tiara): setiap request
// HTTP ke vendor AI — termasuk percobaan ulang SDK dan model cadangan — dicatat dengan
// provider, model, status HTTP, latensi, token, biaya dari vendor, serta ringkasan body
// request & response. Pencatatan dipasang di http.RoundTripper (callLogTransport) supaya
// berlaku untuk semua provider tanpa mengubah kode tiap provider. Request hanya dicatat
// kalau berasal dari Router.Chat / GenerateImage / EditImage (ada callScope di context),
// jadi daftar /models, cek model saat start, dan unduhan gambar hasil tidak ikut tercatat.

// CallRecord: satu request HTTP ke vendor AI.
type CallRecord struct {
	Provider         string
	Endpoint         string // path URL, mis. /v1/chat/completions
	Model            string
	Operation        string // chat | image | edit
	HTTPStatus       int    // 0 = tidak sampai mendapat respons HTTP
	OK               bool
	Latency          time.Duration
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	Cost             *float64 // USD, kalau dilaporkan vendor (mis. OpenRouter usage.cost)
	Error            string
	Request          any // sudah diringkas (gambar base64 diganti ukuran); nil kalau body tidak disimpan
	Response         any
}

// CallRecorder: tujuan CallRecord (implementasi: handlers → applog.APICalls). RecordCall
// tidak boleh memblokir; ctx dipakai untuk membaca fitur / user / request id.
type CallRecorder interface {
	RecordCall(ctx context.Context, c CallRecord)
}

type callScopeKey struct{}

type callScope struct {
	rec       CallRecorder
	bodies    bool
	provider  string
	model     string
	operation string
}

// withCallScope menandai context panggilan router supaya request HTTP di bawahnya dicatat.
func (r *Router) withCallScope(ctx context.Context, choice, operation string) context.Context {
	if r.Recorder == nil {
		return ctx
	}
	provider, model := SplitChoice(choice)
	return context.WithValue(ctx, callScopeKey{}, &callScope{rec: r.Recorder, bodies: r.LogBodies, provider: provider, model: model, operation: operation})
}

// withoutCallLog: request di bawah ctx ini tidak dicatat (mis. mengunduh gambar hasil).
func withoutCallLog(ctx context.Context) context.Context {
	return context.WithValue(ctx, callScopeKey{}, (*callScope)(nil))
}

func scopeFrom(ctx context.Context) *callScope {
	s, _ := ctx.Value(callScopeKey{}).(*callScope)
	return s
}

// newHTTPClient: http.Client untuk provider AI, dengan pencatat AI API Call Log.
func newHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, Transport: callLogTransport{}}
}

type callLogTransport struct {
	base http.RoundTripper // nil = http.DefaultTransport
}

const (
	maxLoggedString = 4000     // string lebih panjang dari ini (selain prompt) dipotong
	maxLoggedPrompt = 20000    // isi pesan / prompt dipotong di sini
	maxBodyRead     = 60 << 20 // sama dengan batas baca jawaban di klien provider
)

func (t callLogTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	scope := scopeFrom(req.Context())
	if scope == nil {
		return base.RoundTrip(req)
	}
	// Salin body request tanpa mengubah req (kontrak RoundTripper).
	var reqBody []byte
	if req.Body != nil && req.Body != http.NoBody {
		if req.GetBody != nil {
			if rc, err := req.GetBody(); err == nil {
				reqBody, _ = io.ReadAll(rc)
				rc.Close()
			}
		} else {
			b, err := io.ReadAll(req.Body)
			req.Body.Close()
			if err != nil {
				return nil, err
			}
			reqBody = b
			req = req.Clone(req.Context())
			req.Body = io.NopCloser(bytes.NewReader(b))
		}
	}

	start := time.Now()
	resp, err := base.RoundTrip(req)
	rec := CallRecord{Provider: scope.provider, Endpoint: req.URL.Path, Model: scope.model, Operation: scope.operation}
	if m := requestModel(req, reqBody); m != "" {
		rec.Model = m
	}
	if scope.bodies {
		rec.Request = summarizeRequest(req, reqBody)
	}
	if err != nil {
		rec.Latency = time.Since(start)
		rec.Error = err.Error()
		scope.rec.RecordCall(req.Context(), rec)
		return nil, err
	}
	// Body jawaban dibaca di sini (latensi = sampai jawaban selesai), lalu diganti salinannya.
	raw, rerr := io.ReadAll(io.LimitReader(resp.Body, maxBodyRead))
	resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(raw))
	rec.Latency = time.Since(start)
	// 202 = NVIDIA NVCF masih memproses (dilanjutkan polling); yang dicatat jawaban akhirnya.
	if resp.StatusCode == http.StatusAccepted {
		return resp, nil
	}
	rec.HTTPStatus = resp.StatusCode
	rec.OK = resp.StatusCode < 300 && rerr == nil
	if rerr != nil {
		rec.Error = rerr.Error()
	}
	parsed := parseResponse(resp, raw, &rec)
	if resp.StatusCode >= 300 {
		rec.Error = responseError(scope.provider, req.URL.Path, resp, raw).Message
	}
	if scope.bodies {
		rec.Response = parsed
	}
	scope.rec.RecordCall(req.Context(), rec)
	return resp, nil
}

// requestModel: model dari body JSON / form multipart (bisa beda dengan pilihan router
// saat model cadangan dipakai).
func requestModel(req *http.Request, body []byte) string {
	var v struct {
		Model string `json:"model"`
	}
	if json.Unmarshal(body, &v) == nil && v.Model != "" {
		return v.Model
	}
	if fields, _ := multipartFields(req, body); fields["model"] != "" {
		return fields["model"]
	}
	return ""
}

func summarizeRequest(req *http.Request, body []byte) any {
	if len(body) == 0 {
		// GET tanpa body (mis. Pollinations: prompt ada di path)
		q := map[string]any{"method": req.Method, "path": unescapePath(req.URL)}
		if len(req.URL.RawQuery) > 0 {
			q["query"] = req.URL.Query()
		}
		return q
	}
	var v any
	if json.Unmarshal(body, &v) == nil {
		return sanitize(v, "")
	}
	if fields, files := multipartFields(req, body); fields != nil {
		out := map[string]any{"multipart": true, "fields": fields}
		if len(files) > 0 {
			out["files"] = files
		}
		return out
	}
	return map[string]any{"bytes": len(body), "preview": shorten(string(body), 500)}
}

func unescapePath(u *url.URL) string {
	if p, err := url.PathUnescape(u.EscapedPath()); err == nil {
		return shorten(p, maxLoggedPrompt)
	}
	return u.Path
}

// multipartFields: field teks & ringkasan file dari body multipart/form-data (edit gambar).
func multipartFields(req *http.Request, body []byte) (map[string]string, []map[string]any) {
	mt, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
	if err != nil || !strings.HasPrefix(mt, "multipart/") || params["boundary"] == "" {
		return nil, nil
	}
	mr := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	fields := map[string]string{}
	var files []map[string]any
	for {
		p, err := mr.NextPart()
		if err != nil {
			break
		}
		data, _ := io.ReadAll(p)
		if p.FileName() != "" {
			files = append(files, map[string]any{"field": p.FormName(), "filename": p.FileName(), "bytes": len(data), "mime": http.DetectContentType(data)})
		} else {
			fields[p.FormName()] = shorten(string(data), maxLoggedPrompt)
		}
		p.Close()
	}
	return fields, files
}

// parseResponse meringkas body jawaban dan mengisi token, biaya, dan error (200 + objek
// "error" dari kenari setelah request lama) ke rec.
func parseResponse(resp *http.Response, raw []byte, rec *CallRecord) any {
	ct := resp.Header.Get("Content-Type")
	if mt := http.DetectContentType(raw); strings.HasPrefix(mt, "image/") || strings.HasPrefix(ct, "image/") {
		return map[string]any{"content_type": mt, "bytes": len(raw)}
	}
	trimmed := bytes.TrimSpace(raw) // kenari: spasi penjaga koneksi sebelum JSON
	var v any
	if json.Unmarshal(trimmed, &v) != nil {
		if len(trimmed) == 0 {
			return nil
		}
		return shorten(string(trimmed), maxLoggedString)
	}
	if obj, ok := v.(map[string]any); ok {
		if u, ok := obj["usage"].(map[string]any); ok {
			readUsage(u, rec)
		}
		if rec.OK {
			if e, ok := obj["error"].(map[string]any); ok {
				rec.OK = false
				rec.Error, _ = e["message"].(string)
				if rec.Error == "" {
					b, _ := json.Marshal(e)
					rec.Error = string(b)
				}
			}
		}
	}
	return sanitize(v, "")
}

// readUsage: gaya OpenAI (prompt/completion/total_tokens, cost di OpenRouter) dan gaya
// Anthropic / OpenAI Images (input/output_tokens).
func readUsage(u map[string]any, rec *CallRecord) {
	num := func(keys ...string) int {
		for _, k := range keys {
			if f, ok := u[k].(float64); ok {
				return int(f)
			}
		}
		return 0
	}
	rec.PromptTokens = num("prompt_tokens", "input_tokens")
	rec.CompletionTokens = num("completion_tokens", "output_tokens")
	rec.TotalTokens = num("total_tokens")
	if rec.TotalTokens == 0 {
		rec.TotalTokens = rec.PromptTokens + rec.CompletionTokens
	}
	if c, ok := u["cost"].(float64); ok {
		rec.Cost = &c
	}
}

// sanitize mengganti data base64 (gambar di request edit / jawaban gambar) dengan ukurannya
// dan memotong string panjang, supaya log tidak menyimpan megabyte gambar.
func sanitize(v any, key string) any {
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			x[k] = sanitize(val, k)
		}
		return x
	case []any:
		for i, val := range x {
			x[i] = sanitize(val, key)
		}
		return x
	case string:
		if strings.HasPrefix(x, "data:") && len(x) > 64 {
			head, _, _ := strings.Cut(x, ",")
			return head + ",… (" + strconv.Itoa(len(x)) + " karakter base64 tidak disimpan)"
		}
		switch key {
		case "b64_json", "base64", "image", "images", "data":
			if len(x) > 64 {
				return "… (" + strconv.Itoa(len(x)) + " karakter base64 tidak disimpan)"
			}
		case "content", "text", "prompt", "inputs":
			return shorten(x, maxLoggedPrompt)
		}
		return shorten(x, maxLoggedString)
	}
	return v
}

func shorten(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
