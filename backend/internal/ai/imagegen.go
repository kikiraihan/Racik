package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Pollinations: generate gambar gratis tanpa API key lewat
// GET image.pollinations.ai/prompt/{prompt}?width&height&nologo=true&model=flux
// (sama dengan katilidev GeneratePostThumbnailJob::generateViaPollinations).
type Pollinations struct {
	BaseURL string // default https://image.pollinations.ai/prompt/
	Enabled bool
	HTTP    *http.Client
}

func NewPollinations(enabled bool, timeout time.Duration) *Pollinations {
	return &Pollinations{BaseURL: "https://image.pollinations.ai/prompt/", Enabled: enabled, HTTP: &http.Client{Timeout: timeout}}
}

func (p *Pollinations) Configured() bool { return p.Enabled }

func (p *Pollinations) GenerateImage(ctx context.Context, model, prompt string, width, height int) (*Image, error) {
	if width <= 0 || height <= 0 {
		width, height = 1024, 1024
	}
	q := url.Values{"width": {strconv.Itoa(width)}, "height": {strconv.Itoa(height)}, "nologo": {"true"}}
	if model != "" {
		q.Set("model", model)
	}
	u := p.BaseURL + url.PathEscape(strings.Join(strings.Fields(prompt), " ")) + "?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	return readImageResponse(p.HTTP, req, "pollinations", "/prompt")
}

// HuggingFace: Inference API text-to-image, POST {base}/{model} dengan
// {"inputs", "parameters": {width, height}}; jawabannya langsung biner gambar.
// 503 = model sedang dimuat (cold start).
type HuggingFace struct {
	BaseURL string // default https://router.huggingface.co/hf-inference/models
	APIKey  string
	HTTP    *http.Client
}

func NewHuggingFace(baseURL, apiKey string, timeout time.Duration) *HuggingFace {
	return &HuggingFace{BaseURL: strings.TrimRight(baseURL, "/"), APIKey: apiKey, HTTP: &http.Client{Timeout: timeout}}
}

func (h *HuggingFace) Configured() bool { return h.APIKey != "" }

func (h *HuggingFace) GenerateImage(ctx context.Context, model, prompt string, width, height int) (*Image, error) {
	if !h.Configured() {
		return nil, notConfigured("huggingface")
	}
	params := map[string]any{}
	if width > 0 && height > 0 {
		// Model diffusion umumnya butuh sisi kelipatan 8.
		params["width"], params["height"] = width/8*8, height/8*8
	}
	b, _ := json.Marshal(map[string]any{"inputs": prompt, "parameters": params})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.BaseURL+"/"+model, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+h.APIKey)
	req.Header.Set("Accept", "image/png")
	img, err := readImageResponse(h.HTTP, req, "huggingface", "/"+model)
	var ae *APIError
	if err != nil && errors.As(err, &ae) && ae.Status == http.StatusServiceUnavailable {
		return nil, fmt.Errorf("model Hugging Face %s sedang dimuat, coba lagi sebentar: %w", model, err)
	}
	return img, err
}

// readImageResponse menjalankan request yang jawabannya biner gambar.
func readImageResponse(client *http.Client, req *http.Request, provider, path string) (*Image, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 30<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, responseError(provider, path, resp, raw)
	}
	mime := http.DetectContentType(raw)
	if !strings.HasPrefix(mime, "image/") {
		return nil, fmt.Errorf("jawaban %s bukan gambar: %s", provider, truncate(string(raw), 300))
	}
	return &Image{Data: raw, Mime: mime}, nil
}
