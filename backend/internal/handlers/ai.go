package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/kikiraihan/racik/backend/internal/ai"
	"github.com/kikiraihan/racik/backend/internal/billing"
	"github.com/kikiraihan/racik/backend/internal/httpx"
)

const maxImagesPerDesign = 4

// GET /api/ai/models — info AI untuk user. Provider & model sengaja tidak dikirim: user tidak
// perlu tahu dalamannya; model diatur admin (lihat /api/admin/ai-config). Yang dikirim hanya
// status layanan dan tarif per fitur.
func (s *Server) aiModels(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, 200, map[string]any{
		"configured":         s.AI.Configured(),
		"prices":             s.featurePrices(r.Context()),
		"fonts":              ai.Fonts,
		"maxImagesPerDesign": maxImagesPerDesign,
	})
}

func nonNil(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

// aiError menjawab error vendor AI dengan pesan umum: detail (provider, model, pesan asli
// vendor) hanya masuk log vendor & log server, tidak dikirim ke user. Kode request dipakai
// admin untuk mencari detailnya di system log.
func (s *Server) aiError(w http.ResponseWriter, r *http.Request, err error) {
	reqID := httpx.RequestID(r.Context())
	log.Printf("ai error [%s]: %v", reqID, err)
	activity(r, "ai_error", truncate(err.Error(), 500))
	if errors.Is(err, ai.ErrNotConfigured) {
		httpx.Error(w, 503, "Layanan AI belum tersedia. Hubungi admin. (kode: "+reqID+")")
		return
	}
	httpx.Error(w, 502, "Layanan AI sedang gangguan, coba lagi beberapa saat. Saldo kamu tidak terpotong. (kode: "+reqID+")")
}

// loadExamples mengambil desain AI yang pernah diberi rating oleh user (personalisasi).
func (s *Server) loadExamples(ctx context.Context, userID string) []ai.Example {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT prompt, result, rating, feedback FROM ai_generations
		WHERE user_id=$1 AND kind IN ('design','refine') AND (rating IS NOT NULL OR feedback <> '')
		ORDER BY created_at DESC LIMIT 12`, userID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []ai.Example
	for rows.Next() {
		var ex ai.Example
		var rating *int
		if rows.Scan(&ex.Prompt, &ex.Spec, &rating, &ex.Feedback) == nil {
			ex.Liked = rating != nil && *rating > 0
			out = append(out, ex)
		}
	}
	return out
}

func (s *Server) logGeneration(ctx context.Context, userID, kind, prompt, model string, result any) string {
	b, _ := json.Marshal(result)
	var id string
	if err := s.DB.QueryRowContext(ctx, `INSERT INTO ai_generations(user_id, kind, prompt, model, result) VALUES($1,$2,$3,$4,$5) RETURNING id`,
		userID, kind, prompt, model, b).Scan(&id); err != nil {
		log.Printf("log generation: %v", err)
	}
	return id
}

// POST /api/ai/design — inti fitur: auto-bikin desain dari prompt.
func (s *Server) aiDesign(w http.ResponseWriter, r *http.Request) {
	var req ai.DesignRequest
	if err := httpx.Decode(r, &req, 1<<20); err != nil {
		httpx.Error(w, 400, "body tidak valid")
		return
	}
	req.Prompt = strings.TrimSpace(req.Prompt)
	if req.Prompt == "" {
		httpx.Error(w, 400, "prompt wajib diisi")
		return
	}
	if req.Width <= 0 || req.Height <= 0 {
		req.Width, req.Height = 1080, 1080
	}
	req.Pages = min(max(req.Pages, 1), 15)
	user := httpx.CurrentUser(r)
	models := s.activeModels(r.Context())
	model := models.TextModel
	prices, _ := s.pricing(r.Context())
	activity(r, "prompt", truncate(req.Prompt, 300))

	// Biaya: 1x susun layout (model teks) + per gambar AI yang berhasil dibuat.
	bill := s.newCharge(r, "design")
	textPrice := prices.UserPrice(model)
	if err := bill.reserve(r.Context(), textPrice, "Magic Design — susun layout & copy"); err != nil {
		s.chargeError(w, r, err, textPrice)
		return
	}
	defer bill.settle(r.Context(), r)

	msgs := ai.BuildDesignMessages(req, s.brandContext(r.Context(), user.ID), s.loadExamples(r.Context(), user.ID), maxImagesPerDesign)
	spec, err := s.chatJSON(r.Context(), model, msgs, 8000)
	if err != nil {
		s.aiError(w, r, err)
		return
	}
	bill.use(textPrice)
	spec["width"], spec["height"] = req.Width, req.Height
	warnings := []string{}
	if req.GenerateImages {
		warnings = s.fillImages(r.Context(), user.ID, models.ImageModel, spec, req.Width, req.Height, bill, prices.UserPrice(models.ImageModel), "Magic Design")
	}
	id := s.logGeneration(r.Context(), user.ID, "design", req.Prompt, model, spec)
	httpx.JSON(w, 200, map[string]any{"generationId": id, "spec": spec, "warnings": warnings})
}

// POST /api/ai/refine — "Magic redesign": perbaiki desain yang sedang dibuka.
func (s *Server) aiRefine(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Spec           json.RawMessage `json:"spec"`
		Instruction    string          `json:"instruction"`
		Width          int             `json:"width"`
		Height         int             `json:"height"`
		TextModel      string          `json:"textModel"`
		ImageModel     string          `json:"imageModel"`
		GenerateImages bool            `json:"generateImages"`
	}
	if err := httpx.Decode(r, &req, 30<<20); err != nil {
		httpx.Error(w, 400, "body tidak valid")
		return
	}
	if strings.TrimSpace(req.Instruction) == "" || len(req.Spec) == 0 {
		httpx.Error(w, 400, "spec dan instruction wajib diisi")
		return
	}
	user := httpx.CurrentUser(r)
	models := s.activeModels(r.Context())
	model := models.TextModel
	prices, _ := s.pricing(r.Context())
	activity(r, "prompt", truncate(req.Instruction, 300))

	bill := s.newCharge(r, "refine")
	textPrice := prices.UserPrice(model)
	if err := bill.reserve(r.Context(), textPrice, "Perbaiki desain — susun ulang layout"); err != nil {
		s.chargeError(w, r, err, textPrice)
		return
	}
	defer bill.settle(r.Context(), r)

	spec, err := s.chatJSON(r.Context(), model, ai.BuildRefineMessages(req.Spec, req.Instruction, req.Width, req.Height, req.GenerateImages), 8000)
	if err != nil {
		s.aiError(w, r, err)
		return
	}
	bill.use(textPrice)
	spec["width"], spec["height"] = req.Width, req.Height
	warnings := []string{}
	if req.GenerateImages {
		warnings = s.fillImages(r.Context(), user.ID, models.ImageModel, spec, req.Width, req.Height, bill, prices.UserPrice(models.ImageModel), "Perbaiki desain")
	}
	id := s.logGeneration(r.Context(), user.ID, "refine", req.Instruction, model, spec)
	httpx.JSON(w, 200, map[string]any{"generationId": id, "spec": spec, "warnings": warnings})
}

func (s *Server) chatJSON(ctx context.Context, model string, msgs []ai.Message, maxTokens int) (map[string]any, error) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		text, err := s.AI.Chat(ctx, model, msgs, maxTokens)
		if err != nil {
			return nil, err
		}
		raw, err := ai.ExtractJSON(text)
		if err == nil {
			var spec map[string]any
			if err = json.Unmarshal(raw, &spec); err == nil {
				if pages, ok := spec["pages"].([]any); ok && len(pages) > 0 {
					return spec, nil
				}
				err = errors.New("spec tidak memiliki halaman")
			}
		}
		lastErr = err
		msgs = append(msgs, ai.Message{Role: "assistant", Content: text},
			ai.Message{Role: "user", Content: "Your previous answer was not valid (" + err.Error() + "). Reply again with ONLY the complete valid JSON object."})
	}
	return nil, lastErr
}

// fillImages men-generate semua imagePrompt di spec secara paralel lalu mengisi field "src".
// Saldo dipesan untuk sebanyak mungkin gambar; gambar yang gagal dikembalikan saldonya.
func (s *Server) fillImages(ctx context.Context, userID, model string, spec map[string]any, pw, ph int, bill *charge, unit int64, label string) []string {
	type job struct {
		target map[string]any
		prompt string
		w, h   int
	}
	var jobs []job
	pages, _ := spec["pages"].([]any)
	for _, p := range pages {
		page, _ := p.(map[string]any)
		if page == nil {
			continue
		}
		if bg, _ := page["background"].(map[string]any); bg != nil && bg["type"] == "image" {
			if pr, _ := bg["imagePrompt"].(string); pr != "" && bg["src"] == nil {
				jobs = append(jobs, job{bg, pr, pw, ph})
			}
		}
		els, _ := page["elements"].([]any)
		for _, e := range els {
			el, _ := e.(map[string]any)
			if el == nil || el["type"] != "image" || el["src"] != nil {
				continue
			}
			if pr, _ := el["imagePrompt"].(string); pr != "" {
				jobs = append(jobs, job{el, pr, num(el["w"], pw), num(el["h"], ph)})
			}
		}
	}
	if len(jobs) > maxImagesPerDesign {
		jobs = jobs[:maxImagesPerDesign]
	}
	var warnings []string
	if len(jobs) > 0 {
		k := bill.reserveUpTo(ctx, len(jobs), unit, func(k int) string { return fmt.Sprintf("%s — %d gambar AI", label, k) })
		if k < len(jobs) {
			warnings = append(warnings, fmt.Sprintf("Saldo tidak cukup untuk %d gambar AI (%s/gambar), placeholder dibiarkan kosong. Silakan top up saldo.", len(jobs)-k, rupiah(unit)))
			jobs = jobs[:k]
		}
	}
	var (
		wg  sync.WaitGroup
		mu  sync.Mutex
		sem = make(chan struct{}, 3)
	)
	for _, j := range jobs {
		wg.Add(1)
		go func(j job) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			prompt := j.prompt + ". High quality, professional, no text, no watermark."
			img, err := s.AI.GenerateImage(ctx, model, prompt, j.w, j.h)
			var a *assetDTO
			if err == nil {
				a, err = s.saveAsset(ctx, userID, img.Data, img.Mime, "ai-"+time.Now().Format("20060102-150405")+".png", "ai")
			}
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				// Detail error vendor ada di log vendor; ke user cukup pesan umum.
				log.Printf("gambar ai [%s]: %v", httpx.RequestID(ctx), err)
				warnings = append(warnings, fmt.Sprintf("Gambar %q gagal dibuat, saldonya dikembalikan.", truncate(j.prompt, 40)))
				return
			}
			bill.use(unit)
			j.target["src"] = a.URL
		}(j)
	}
	wg.Wait()
	return warnings
}

// POST /api/ai/image — generate gambar tunggal (Nano Banana / GPT Image).
func (s *Server) aiImage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Prompt string `json:"prompt"`
		Model  string `json:"model"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
		Style  string `json:"style"`
	}
	if err := httpx.Decode(r, &req, 1<<20); err != nil || strings.TrimSpace(req.Prompt) == "" {
		httpx.Error(w, 400, "prompt wajib diisi")
		return
	}
	user := httpx.CurrentUser(r)
	model := s.activeModels(r.Context()).ImageModel
	prompt := req.Prompt
	if req.Style != "" {
		prompt += ". Style: " + req.Style
	}
	activity(r, "prompt", truncate(prompt, 300))
	prices, _ := s.pricing(r.Context())
	bill := s.newCharge(r, "image")
	price := prices.UserPrice(model)
	if err := bill.reserve(r.Context(), price, "Gambar AI"); err != nil {
		s.chargeError(w, r, err, price)
		return
	}
	defer bill.settle(r.Context(), r)
	img, err := s.AI.GenerateImage(r.Context(), model, prompt, req.Width, req.Height)
	if err != nil {
		s.aiError(w, r, err)
		return
	}
	a, err := s.saveAsset(r.Context(), user.ID, img.Data, img.Mime, "ai-image.png", "ai")
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	bill.use(price)
	id := s.logGeneration(r.Context(), user.ID, "image", req.Prompt, model, map[string]any{"url": a.URL})
	httpx.JSON(w, 200, map[string]any{"generationId": id, "asset": a})
}

// POST /api/ai/image-edit — edit gambar yang ada dengan instruksi (hapus objek, ganti gaya, dsb).
func (s *Server) aiImageEdit(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Prompt string `json:"prompt"`
		Model  string `json:"model"`
		URL    string `json:"url"`     // URL /uploads/... milik user
		Data   string `json:"dataUrl"` // atau data URL (mis. hasil export objek dari canvas)
	}
	if err := httpx.Decode(r, &req, 40<<20); err != nil || strings.TrimSpace(req.Prompt) == "" {
		httpx.Error(w, 400, "prompt wajib diisi")
		return
	}
	user := httpx.CurrentUser(r)
	var src ai.Image
	switch {
	case req.Data != "":
		_, b64, ok := strings.Cut(req.Data, ";base64,")
		data, err := base64.StdEncoding.DecodeString(b64)
		if !ok || err != nil {
			httpx.Error(w, 400, "dataUrl tidak valid")
			return
		}
		src = ai.Image{Data: data, Mime: http.DetectContentType(data)}
	case req.URL != "":
		data, mime, err := s.readOwnAsset(user.ID, req.URL)
		if err != nil {
			httpx.Error(w, 400, err.Error())
			return
		}
		src = ai.Image{Data: data, Mime: mime}
	default:
		httpx.Error(w, 400, "url atau dataUrl wajib diisi")
		return
	}
	model := s.activeModels(r.Context()).EditModel
	activity(r, "prompt", truncate(req.Prompt, 300))
	prices, _ := s.pricing(r.Context())
	bill := s.newCharge(r, "edit")
	price := prices.UserPrice(model)
	if err := bill.reserve(r.Context(), price, "Edit gambar AI"); err != nil {
		s.chargeError(w, r, err, price)
		return
	}
	defer bill.settle(r.Context(), r)
	img, err := s.AI.EditImage(r.Context(), model, req.Prompt, &src)
	if err != nil {
		s.aiError(w, r, err)
		return
	}
	a, err := s.saveAsset(r.Context(), user.ID, img.Data, img.Mime, "ai-edit.png", "ai")
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	bill.use(price)
	id := s.logGeneration(r.Context(), user.ID, "edit", req.Prompt, model, map[string]any{"url": a.URL})
	httpx.JSON(w, 200, map[string]any{"generationId": id, "asset": a})
}

// POST /api/ai/copy — AI copywriting.
func (s *Server) aiCopy(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Prompt string `json:"prompt"`
		Kind   string `json:"kind"`
		Model  string `json:"model"`
	}
	if err := httpx.Decode(r, &req, 1<<20); err != nil || strings.TrimSpace(req.Prompt) == "" {
		httpx.Error(w, 400, "prompt wajib diisi")
		return
	}
	user := httpx.CurrentUser(r)
	model := s.activeModels(r.Context()).TextModel
	activity(r, "prompt", truncate(req.Prompt, 300))
	prices, _ := s.pricing(r.Context())
	bill := s.newCharge(r, "copy")
	price := prices.UserPrice(model)
	if err := bill.reserve(r.Context(), price, "Copywriting AI"); err != nil {
		s.chargeError(w, r, err, price)
		return
	}
	defer bill.settle(r.Context(), r)
	bk := s.loadBrandKit(r.Context(), user.ID)
	text, err := s.AI.Chat(r.Context(), model, ai.BuildCopyMessages(req.Prompt, req.Kind, bk.Voice), 1200)
	if err != nil {
		s.aiError(w, r, err)
		return
	}
	bill.use(price)
	var options []string
	if raw, err := ai.ExtractJSON(text); err == nil {
		json.Unmarshal(raw, &options)
	}
	if len(options) == 0 {
		for _, line := range strings.Split(text, "\n") {
			if line = strings.Trim(strings.TrimSpace(line), "-•*\"0123456789. "); line != "" {
				options = append(options, line)
			}
		}
	}
	s.logGeneration(r.Context(), user.ID, "copy", req.Prompt, model, options)
	httpx.JSON(w, 200, map[string]any{"options": options})
}

func (s *Server) listGenerations(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.QueryContext(r.Context(), `SELECT id, kind, prompt, model, rating, feedback, created_at
		FROM ai_generations WHERE user_id=$1 ORDER BY created_at DESC LIMIT 50`, httpx.CurrentUser(r).ID)
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	defer rows.Close()
	type gen struct {
		ID        string    `json:"id"`
		Kind      string    `json:"kind"`
		Prompt    string    `json:"prompt"`
		Model     string    `json:"-"` // model internal tidak dikirim ke user
		Rating    *int      `json:"rating"`
		Feedback  string    `json:"feedback"`
		CreatedAt time.Time `json:"createdAt"`
	}
	out := []gen{}
	for rows.Next() {
		var g gen
		if rows.Scan(&g.ID, &g.Kind, &g.Prompt, &g.Model, &g.Rating, &g.Feedback, &g.CreatedAt) == nil {
			out = append(out, g)
		}
	}
	httpx.JSON(w, 200, out)
}

// POST /api/ai/generations/{id}/rate — rating & masukan, dipakai agar hasil berikutnya makin bagus.
func (s *Server) rateGeneration(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Rating   int    `json:"rating"`
		Feedback string `json:"feedback"`
	}
	if err := httpx.Decode(r, &req, 1<<16); err != nil || (req.Rating != 1 && req.Rating != -1) {
		httpx.Error(w, 400, "rating harus 1 atau -1")
		return
	}
	res, err := s.DB.ExecContext(r.Context(), `UPDATE ai_generations SET rating=$3, feedback=$4 WHERE id=$1 AND user_id=$2`,
		r.PathValue("id"), httpx.CurrentUser(r).ID, req.Rating, strings.TrimSpace(req.Feedback))
	if n, _ := rowsAffected(res); err != nil || n == 0 {
		httpx.Error(w, 404, "generation tidak ditemukan")
		return
	}
	httpx.JSON(w, 200, map[string]bool{"ok": true})
}

// chargeError: gagal memotong saldo sebelum memanggil AI.
func (s *Server) chargeError(w http.ResponseWriter, r *http.Request, err error, need int64) {
	if errors.Is(err, billing.ErrInsufficient) {
		s.insufficient(w, r, need)
		return
	}
	log.Printf("potong saldo: %v", err)
	httpx.Error(w, 500, "gagal memproses saldo")
}

func num(v any, def int) int {
	if f, ok := v.(float64); ok && f > 0 {
		return int(f)
	}
	return def
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
