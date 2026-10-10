package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/kikiraihan/racik/backend/internal/ai"
	"github.com/kikiraihan/racik/backend/internal/httpx"
)

const maxImagesPerDesign = 4

func (s *Server) aiModels(w http.ResponseWriter, r *http.Request) {
	// Id model berformat "<provider>:<model>"; "labels" berisi teks tampilannya.
	httpx.JSON(w, 200, map[string]any{
		"configured":         s.AI.Configured(),
		"provider":           "kenari.id",
		"textModel":          s.AI.DefaultText(),
		"textModels":         nonNil(s.AI.TextChoices()),
		"imageModel":         s.AI.DefaultImage(),
		"imageModels":        nonNil(s.AI.ImageChoices()),
		"editModels":         nonNil(s.AI.EditChoices()),
		"labels":             s.AI.Labels(),
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

func (s *Server) aiError(w http.ResponseWriter, err error) {
	if errors.Is(err, ai.ErrNotConfigured) {
		httpx.Error(w, 503, err.Error())
		return
	}
	log.Printf("ai error: %v", err)
	httpx.Error(w, 502, "AI gagal: "+err.Error())
}

func (s *Server) pickTextModel(m string) string {
	if m != "" && slices.Contains(s.AI.TextChoices(), m) {
		return m
	}
	return s.AI.DefaultText()
}

func (s *Server) pickImageModel(m string) string {
	if m != "" && slices.Contains(s.AI.ImageChoices(), m) {
		return m
	}
	return s.AI.DefaultImage()
}

// pickEditModel: model gambar pilihan kalau providernya bisa edit gambar (kenari,
// OpenRouter); kalau tidak, model edit pertama yang tersedia.
func (s *Server) pickEditModel(m string) string {
	m = s.pickImageModel(m)
	edits := s.AI.EditChoices()
	if slices.Contains(edits, m) || len(edits) == 0 {
		return m
	}
	return edits[0]
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
	model := s.pickTextModel(req.TextModel)

	msgs := ai.BuildDesignMessages(req, s.brandContext(r.Context(), user.ID), s.loadExamples(r.Context(), user.ID), maxImagesPerDesign)
	spec, err := s.chatJSON(r.Context(), model, msgs, 8000)
	if err != nil {
		s.aiError(w, err)
		return
	}
	spec["width"], spec["height"] = req.Width, req.Height
	warnings := []string{}
	if req.GenerateImages {
		warnings = s.fillImages(r.Context(), user.ID, s.pickImageModel(req.ImageModel), spec, req.Width, req.Height)
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
	model := s.pickTextModel(req.TextModel)
	spec, err := s.chatJSON(r.Context(), model, ai.BuildRefineMessages(req.Spec, req.Instruction, req.Width, req.Height, req.GenerateImages), 8000)
	if err != nil {
		s.aiError(w, err)
		return
	}
	spec["width"], spec["height"] = req.Width, req.Height
	warnings := []string{}
	if req.GenerateImages {
		warnings = s.fillImages(r.Context(), user.ID, s.pickImageModel(req.ImageModel), spec, req.Width, req.Height)
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
func (s *Server) fillImages(ctx context.Context, userID, model string, spec map[string]any, pw, ph int) []string {
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
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		warnings []string
		sem      = make(chan struct{}, 3)
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
				warnings = append(warnings, fmt.Sprintf("gambar %q gagal: %v", truncate(j.prompt, 40), err))
				return
			}
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
	model := s.pickImageModel(req.Model)
	prompt := req.Prompt
	if req.Style != "" {
		prompt += ". Style: " + req.Style
	}
	img, err := s.AI.GenerateImage(r.Context(), model, prompt, req.Width, req.Height)
	if err != nil {
		s.aiError(w, err)
		return
	}
	a, err := s.saveAsset(r.Context(), user.ID, img.Data, img.Mime, "ai-image.png", "ai")
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
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
	model := s.pickEditModel(req.Model)
	img, err := s.AI.EditImage(r.Context(), model, req.Prompt, &src)
	if err != nil {
		s.aiError(w, err)
		return
	}
	a, err := s.saveAsset(r.Context(), user.ID, img.Data, img.Mime, "ai-edit.png", "ai")
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
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
	model := s.pickTextModel(req.Model)
	bk := s.loadBrandKit(r.Context(), user.ID)
	text, err := s.AI.Chat(r.Context(), model, ai.BuildCopyMessages(req.Prompt, req.Kind, bk.Voice), 1200)
	if err != nil {
		s.aiError(w, err)
		return
	}
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
		Model     string    `json:"model"`
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
