package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

// NvidiaImage men-generate gambar lewat endpoint GenAI NVIDIA
// (POST ai.api.nvidia.com/v1/genai/{model}), sama dengan katilidev NvidiaClient::image():
//   - tiap keluarga model menerima body berbeda (FLUX.2, FLUX.1 dev/schnell, SD3);
//   - FLUX hanya menerima sisi kelipatan 64 dan luas <= 1.062.400 px;
//   - prompt > 800 karakter ditolak, jadi dipotong di batas kalimat (kecuali prompt JSON);
//   - model berat bisa menjawab 202 → hasil diambil dengan polling NVCF;
//   - model yang ditolak (400/404/410/422) dicoba ulang sekali dengan model default.
type NvidiaImage struct {
	APIKey   string
	HTTP     *http.Client
	Fallback string
	Steps    int // langkah FLUX.2 (default 4)

	BaseURL       string // default https://ai.api.nvidia.com/v1/genai/
	NVCFStatusURL string // default https://api.nvcf.nvidia.com/v2/nvcf/pexec/status/
	PollEvery     time.Duration
	PollTimeout   time.Duration
}

const (
	NvidiaImageDefault     = "black-forest-labs/flux.2-klein-4b"
	nvidiaImageMaxPixels   = 1062400
	nvidiaImagePromptChars = 800
)

func NewNvidiaImage(apiKey string, steps int, timeout time.Duration) *NvidiaImage {
	if steps <= 0 {
		steps = 4
	}
	return &NvidiaImage{
		APIKey: apiKey, HTTP: newHTTPClient(timeout), Fallback: NvidiaImageDefault, Steps: steps,
		BaseURL:       "https://ai.api.nvidia.com/v1/genai/",
		NVCFStatusURL: "https://api.nvcf.nvidia.com/v2/nvcf/pexec/status/",
		PollEvery:     2 * time.Second, PollTimeout: 150 * time.Second,
	}
}

func (n *NvidiaImage) Configured() bool { return n.APIKey != "" }

func (n *NvidiaImage) GenerateImage(ctx context.Context, model, prompt string, width, height int) (*Image, error) {
	if !n.Configured() {
		return nil, notConfigured("nvidia")
	}
	img, err := n.generate(ctx, model, prompt, width, height)
	if err == nil || n.Fallback == "" || n.Fallback == model || !modelRejected(err) {
		return img, err
	}
	img, fbErr := n.generate(ctx, n.Fallback, prompt, width, height)
	if fbErr != nil {
		return nil, fmt.Errorf("model gambar pilihan (%s) ditolak: %v — model cadangan (%s) juga gagal: %w", model, err, n.Fallback, fbErr)
	}
	return img, nil
}

func (n *NvidiaImage) generate(ctx context.Context, model, prompt string, width, height int) (*Image, error) {
	b, _ := json.Marshal(nvidiaImagePayload(model, prompt, width, height, n.Steps))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.BaseURL+model, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, raw, err := n.do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusAccepted {
		resp, raw, err = n.waitNVCF(ctx, resp.Header.Get("NVCF-REQID"))
		if err != nil {
			return nil, err
		}
	}
	if resp.StatusCode >= 300 {
		return nil, responseError("nvidia", "/genai/"+model, resp, raw)
	}
	var out struct {
		Artifacts []struct {
			Base64       string `json:"base64"`
			FinishReason string `json:"finishReason"`
		} `json:"artifacts"`
		Image        string   `json:"image"`
		Images       []string `json:"images"`
		FinishReason string   `json:"finish_reason"`
		Data         []struct {
			B64JSON string `json:"b64_json"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("jawaban nvidia bukan JSON: %s", truncate(string(raw), 300))
	}
	reason := out.FinishReason
	candidates := []string{out.Image}
	if len(out.Artifacts) > 0 {
		reason = out.Artifacts[0].FinishReason
		candidates = append([]string{out.Artifacts[0].Base64}, candidates...)
	}
	if reason != "" && !strings.EqualFold(reason, "SUCCESS") {
		return nil, fmt.Errorf("nvidia berhenti membuat gambar: %s", reason)
	}
	if len(out.Images) > 0 {
		candidates = append(candidates, out.Images[0])
	}
	if len(out.Data) > 0 {
		candidates = append(candidates, out.Data[0].B64JSON)
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if strings.HasPrefix(c, "data:") {
			_, c, _ = strings.Cut(c, ",")
		}
		data, err := base64.StdEncoding.DecodeString(c)
		if err != nil || len(data) == 0 {
			return nil, fmt.Errorf("base64 gambar nvidia tidak valid")
		}
		return &Image{Data: data, Mime: http.DetectContentType(data)}, nil
	}
	return nil, fmt.Errorf("jawaban nvidia tidak berisi gambar: %s", truncate(string(raw), 300))
}

func (n *NvidiaImage) do(req *http.Request) (*http.Response, []byte, error) {
	req.Header.Set("Authorization", "Bearer "+n.APIKey)
	req.Header.Set("Accept", "application/json")
	resp, err := n.HTTP.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 60<<20))
	return resp, raw, err
}

// waitNVCF: model berat menjawab 202 + header NVCF-REQID; hasilnya diambil dengan polling.
func (n *NvidiaImage) waitNVCF(ctx context.Context, reqID string) (*http.Response, []byte, error) {
	if reqID == "" {
		return nil, nil, fmt.Errorf("nvidia membalas 202 tanpa header NVCF-REQID")
	}
	deadline := time.Now().Add(n.PollTimeout)
	for {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-time.After(n.PollEvery):
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, n.NVCFStatusURL+reqID, nil)
		if err != nil {
			return nil, nil, err
		}
		resp, raw, err := n.do(req)
		if err != nil {
			return nil, nil, err
		}
		if resp.StatusCode != http.StatusAccepted {
			return resp, raw, nil
		}
		if time.Now().After(deadline) {
			return nil, nil, fmt.Errorf("nvidia belum selesai membuat gambar setelah %s", n.PollTimeout)
		}
	}
}

// nvidiaImagePayload: body per keluarga model (katilidev NvidiaClient::imagePayload).
func nvidiaImagePayload(model, prompt string, width, height, steps int) map[string]any {
	prompt = fitNvidiaPrompt(prompt)
	switch {
	case strings.Contains(model, "stable-diffusion-3"):
		return map[string]any{"prompt": prompt, "aspect_ratio": nearestRatio(width, height, sd3Ratios), "cfg_scale": 5, "seed": 0, "steps": 50}
	case strings.Contains(model, "flux.1-dev"):
		w, h := fitFlux1Size(width, height)
		return map[string]any{"prompt": prompt, "mode": "base", "cfg_scale": 3.5, "width": w, "height": h, "seed": 0, "steps": 30}
	case strings.Contains(model, "flux.1-schnell"):
		w, h := fitFlux1Size(width, height)
		return map[string]any{"prompt": prompt, "width": w, "height": h, "seed": 0, "steps": 4}
	default: // FLUX.2 dan model yang tidak dikenal: bentuk paling sederhana
		w, h := fitNvidiaImageSize(width, height)
		return map[string]any{"prompt": prompt, "width": w, "height": h, "seed": 0, "steps": steps}
	}
}

var sd3Ratios = map[string][2]int{
	"1:1": {1, 1}, "16:9": {16, 9}, "9:16": {9, 16}, "5:4": {5, 4}, "4:5": {4, 5}, "3:2": {3, 2}, "2:3": {2, 3},
}

// fitNvidiaImageSize: sisi kelipatan 64, luas <= nvidiaImageMaxPixels, rasio dipertahankan.
func fitNvidiaImageSize(w, h int) (int, int) {
	w, h = max(64, w), max(64, h)
	scale := math.Min(1, math.Sqrt(float64(nvidiaImageMaxPixels)/float64(w*h)))
	return max(64, int(math.Floor(float64(w)*scale))/64*64), max(64, int(math.Floor(float64(h)*scale))/64*64)
}

// fitFlux1Size: FLUX.1 menerima sisi 768–1344, kelipatan 64.
func fitFlux1Size(w, h int) (int, int) {
	clamp := func(side int) int { return min(1344, max(768, side/64*64)) }
	return clamp(w), clamp(h)
}

// fitNvidiaPrompt memotong prompt > 800 karakter di akhir kalimat (atau spasi);
// prompt JSON dibiarkan utuh karena memotongnya merusak JSON.
func fitNvidiaPrompt(prompt string) string {
	prompt = strings.Join(strings.Fields(prompt), " ")
	if utf8.RuneCountInString(prompt) <= nvidiaImagePromptChars || (strings.HasPrefix(prompt, "{") && json.Valid([]byte(prompt))) {
		return prompt
	}
	cut := string([]rune(prompt)[:nvidiaImagePromptChars])
	if i := strings.LastIndexAny(cut, ".!?;"); i >= nvidiaImagePromptChars/2 {
		return cut[:i+1]
	}
	if i := strings.LastIndex(cut, " "); i >= nvidiaImagePromptChars/2 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " ,;:-")
}

// nearestRatio memilih nama rasio yang paling dekat (jarak log) dengan lebar × tinggi.
func nearestRatio(w, h int, ratios map[string][2]int) string {
	target := math.Log(float64(max(1, w)) / float64(max(1, h)))
	best, diff := "1:1", math.Inf(1)
	for name, r := range ratios {
		d := math.Abs(math.Log(float64(r[0])/float64(r[1])) - target)
		if d < diff || (d == diff && name < best) {
			best, diff = name, d
		}
	}
	return best
}
