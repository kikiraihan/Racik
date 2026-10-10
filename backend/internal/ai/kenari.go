// Package ai adalah konektor ke gateway AI kenari.id.
//
// Gateway diasumsikan OpenAI-compatible:
//   - POST {base}/chat/completions      -> model teks (Claude, GPT, Gemini)
//   - POST {base}/images/generations    -> model gambar (gpt-image-2, Nano Banana)
//   - POST {base}/images/edits          -> edit gambar dengan prompt
//   - GET  {base}/models                -> daftar model (publik, tanpa key)
//
// Untuk model gambar yang di gateway hanya tersedia lewat chat completions
// (mis. Gemini "Nano Banana"), client otomatis fallback dan mengambil gambar
// dari field message.images / data URL di content.
//
// Perilaku kenari yang ditangani (docs kenari.id, sama dengan konektor di katilidev):
//   - Error berbentuk {"error":{"message","type","code"}}; 401/413 bisa berupa teks biasa,
//     429 bisa membawa header Retry-After.
//   - Request yang lewat 45 dtk dijawab 200 + spasi penjaga koneksi, lalu JSON. Kegagalan
//     setelah titik itu datang sebagai HTTP 200 dengan objek "error" (bukan "data").
//   - Field "url" di jawaban gambar berupa data: URI yang memuat gambarnya sendiri.
//   - response_format sengaja tidak dikirim ke endpoint gambar (dukungannya beda per model).
//   - Semua model gambar berbayar per gambar, jadi n selalu 1 dan fallback ke chat hanya
//     dicoba kalau model ditolak (400/404/410/422), bukan saat saldo habis / kuota / timeout.
package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"
)

type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

func New(baseURL, apiKey string, timeout time.Duration) *Client {
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), APIKey: apiKey, HTTP: &http.Client{Timeout: timeout}}
}

var ErrNotConfigured = errors.New("KENARI_API_KEY belum diisi, konektor AI belum aktif")

func (c *Client) Configured() bool { return c.APIKey != "" }

// APIError adalah error dari gateway kenari. Status 200 berarti request berjalan lama
// (> 45 dtk) lalu gagal di tengah jalan.
type APIError struct {
	Path       string
	Status     int
	Code       string
	Message    string
	RetryAfter string
}

func (e *APIError) Error() string {
	msg := fmt.Sprintf("kenari %s HTTP %d: %s", e.Path, e.Status, e.Message)
	if e.Code != "" {
		msg += " (" + e.Code + ")"
	}
	if e.RetryAfter != "" {
		msg += " (Retry-After: " + e.RetryAfter + " dtk)"
	}
	return msg
}

// modelRejected: model tidak bisa dipakai di endpoint ini (id salah / tidak mendukung
// endpoint) — layak dicoba lewat jalur lain. Bukan 401 (key), 402 (saldo), 429 (kuota),
// 5xx, timeout, atau 200+error (request lama; mengulang berisiko tagihan ganda).
func modelRejected(err error) bool {
	var ae *APIError
	if !errors.As(err, &ae) {
		return false
	}
	switch ae.Status {
	case 400, 404, 410, 422:
		return true
	}
	return false
}

// ---------- Chat ----------

type Message struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string atau []ContentPart
}

type ContentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

type ImageURL struct {
	URL string `json:"url"`
}

type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Modalities  []string  `json:"modalities,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content          json.RawMessage `json:"content"`
			ReasoningContent string          `json:"reasoning_content"`
			Reasoning        string          `json:"reasoning"`
			Images           []struct {
				ImageURL ImageURL `json:"image_url"`
			} `json:"images"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *apiError `json:"error"`
}

type apiError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    any    `json:"code"` // kenari: string, provider lain kadang angka
}

func (e *apiError) toError(path string) *APIError {
	code := ""
	if e.Code != nil {
		code = fmt.Sprint(e.Code)
	}
	return &APIError{Path: path, Status: 200, Code: code, Message: e.Message}
}

// Chat mengirim percakapan dan mengembalikan teks jawaban.
func (c *Client) Chat(ctx context.Context, model string, msgs []Message, maxTokens int) (string, error) {
	resp, err := c.chatRaw(ctx, chatRequest{Model: model, Messages: msgs, Temperature: 0.7, MaxTokens: maxTokens})
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", errors.New("AI tidak mengembalikan jawaban")
	}
	choice := resp.Choices[0]
	text := stripThinking(contentText(choice.Message.Content))
	// Model reasoning bisa menghabiskan seluruh max_tokens untuk berpikir tanpa sempat menjawab.
	if text == "" && (choice.FinishReason == "length" || choice.Message.ReasoningContent != "" || choice.Message.Reasoning != "") {
		return "", fmt.Errorf("model %s menghabiskan batas token untuk reasoning tanpa menulis jawaban, pilih model lain", model)
	}
	return text, nil
}

var thinkRe = regexp.MustCompile(`(?s)<think>.*?</think>`)

// stripThinking membuang blok <think>…</think> yang ditulis sebagian model reasoning di content,
// supaya ExtractJSON tidak tersangkut kurung kurawal di dalam penalarannya.
func stripThinking(s string) string {
	s = thinkRe.ReplaceAllString(s, "")
	// Blok yang tidak ditutup (jawaban terpotong) → buang dari <think> sampai akhir.
	if i := strings.Index(s, "<think>"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

func (c *Client) chatRaw(ctx context.Context, body chatRequest) (*chatResponse, error) {
	var out chatResponse
	if err := c.postJSON(ctx, "/chat/completions", body, &out); err != nil {
		return nil, err
	}
	if out.Error != nil {
		return nil, out.Error.toError("/chat/completions")
	}
	return &out, nil
}

// contentText menangani content berupa string maupun array part.
func contentText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var parts []ContentPart
	if json.Unmarshal(raw, &parts) == nil {
		var b strings.Builder
		for _, p := range parts {
			if p.Type == "text" {
				b.WriteString(p.Text)
			}
		}
		return b.String()
	}
	return string(raw)
}

// ---------- Image ----------

type Image struct {
	Data []byte
	Mime string
}

type imageResponse struct {
	Data []struct {
		B64JSON string `json:"b64_json"`
		URL     string `json:"url"`
	} `json:"data"`
	Error *apiError `json:"error"`
}

// GenerateImage membuat gambar dari prompt. width/height dipakai untuk memilih rasio terdekat.
func (c *Client) GenerateImage(ctx context.Context, model, prompt string, width, height int) (*Image, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	size := pickSize(width, height)
	var out imageResponse
	err := c.postJSON(ctx, "/images/generations", map[string]any{
		"model": model, "prompt": prompt, "n": 1, "size": size,
	}, &out)
	if err == nil && out.Error != nil {
		// 200 + error: request berjalan lama lalu gagal — jangan diulang lewat chat.
		return nil, out.Error.toError("/images/generations")
	}
	if err == nil && len(out.Data) > 0 {
		return c.decodeImage(ctx, out.Data[0].B64JSON, out.Data[0].URL)
	}
	if err == nil {
		return nil, errors.New("jawaban kenari tidak berisi gambar")
	}
	if !modelRejected(err) {
		return nil, err
	}
	// Fallback: model gambar via chat completions (gaya Gemini / OpenRouter).
	img, chatErr := c.imageViaChat(ctx, model, []ContentPart{{Type: "text", Text: prompt + fmt.Sprintf("\n\nAspect ratio approx %s.", ratioLabel(width, height))}})
	if chatErr == nil {
		return img, nil
	}
	return nil, fmt.Errorf("gagal generate gambar: %v; fallback chat: %v", err, chatErr)
}

// EditImage mengubah gambar sesuai instruksi (inpainting global / restyle).
func (c *Client) EditImage(ctx context.Context, model, prompt string, src *Image) (*Image, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("model", model)
	_ = mw.WriteField("prompt", prompt)
	ext := "png"
	if strings.Contains(src.Mime, "jpeg") {
		ext = "jpg"
	}
	fw, _ := mw.CreateFormFile("image", "image."+ext)
	fw.Write(src.Data)
	mw.Close()

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/images/edits", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 60<<20))
	if resp.StatusCode >= 300 {
		err = responseError("/images/edits", resp, raw)
	} else {
		var out imageResponse
		if jerr := json.Unmarshal(raw, &out); jerr != nil {
			return nil, fmt.Errorf("jawaban kenari /images/edits bukan JSON: %s", truncate(string(raw), 300))
		}
		if out.Error != nil {
			return nil, out.Error.toError("/images/edits")
		}
		if len(out.Data) == 0 {
			return nil, errors.New("jawaban kenari tidak berisi gambar")
		}
		return c.decodeImage(ctx, out.Data[0].B64JSON, out.Data[0].URL)
	}
	if !modelRejected(err) {
		return nil, err
	}
	dataURL := "data:" + src.Mime + ";base64," + base64.StdEncoding.EncodeToString(src.Data)
	img, chatErr := c.imageViaChat(ctx, model, []ContentPart{
		{Type: "text", Text: prompt},
		{Type: "image_url", ImageURL: &ImageURL{URL: dataURL}},
	})
	if chatErr == nil {
		return img, nil
	}
	return nil, fmt.Errorf("gagal edit gambar: %v; fallback chat: %v", err, chatErr)
}

var dataURLRe = regexp.MustCompile(`data:(image/[a-zA-Z+]+);base64,([A-Za-z0-9+/=]+)`)
var httpImgRe = regexp.MustCompile(`https?://[^\s)"']+\.(?:png|jpe?g|webp)`)

func (c *Client) imageViaChat(ctx context.Context, model string, parts []ContentPart) (*Image, error) {
	resp, err := c.chatRaw(ctx, chatRequest{
		Model:      model,
		Messages:   []Message{{Role: "user", Content: parts}},
		Modalities: []string{"image", "text"},
	})
	if err != nil {
		return nil, err
	}
	if len(resp.Choices) == 0 {
		return nil, errors.New("tidak ada gambar")
	}
	msg := resp.Choices[0].Message
	for _, im := range msg.Images {
		if im.ImageURL.URL != "" {
			return c.fromURLOrData(ctx, im.ImageURL.URL)
		}
	}
	// Cari gambar di dalam content (array part image_url, data URL, atau link http).
	var parts2 []ContentPart
	if json.Unmarshal(msg.Content, &parts2) == nil {
		for _, p := range parts2 {
			if p.ImageURL != nil && p.ImageURL.URL != "" {
				return c.fromURLOrData(ctx, p.ImageURL.URL)
			}
		}
	}
	text := contentText(msg.Content)
	if m := dataURLRe.FindString(text); m != "" {
		return c.fromURLOrData(ctx, m)
	}
	if m := httpImgRe.FindString(text); m != "" {
		return c.fromURLOrData(ctx, m)
	}
	return nil, errors.New("respon model tidak berisi gambar")
}

func (c *Client) fromURLOrData(ctx context.Context, u string) (*Image, error) {
	if m := dataURLRe.FindStringSubmatch(u); m != nil {
		data, err := base64.StdEncoding.DecodeString(m[2])
		if err != nil {
			return nil, err
		}
		return &Image{Data: data, Mime: m[1]}, nil
	}
	return c.decodeImage(ctx, "", u)
}

func (c *Client) decodeImage(ctx context.Context, b64, url string) (*Image, error) {
	if b64 != "" {
		data, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return nil, err
		}
		return &Image{Data: data, Mime: http.DetectContentType(data)}, nil
	}
	if url == "" {
		return nil, errors.New("gambar kosong")
	}
	// kenari mengisi "url" dengan data: URI yang memuat gambarnya, bukan tautan.
	if strings.HasPrefix(url, "data:") {
		_, b64, ok := strings.Cut(url, ",")
		if !ok {
			return nil, errors.New("data URI gambar tidak valid")
		}
		return c.decodeImage(ctx, b64, "")
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gagal mengunduh gambar (HTTP %d): %s", resp.StatusCode, truncate(url, 200))
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 30<<20))
	if err != nil {
		return nil, err
	}
	return &Image{Data: data, Mime: http.DetectContentType(data)}, nil
}

func (c *Client) postJSON(ctx context.Context, path string, body any, out any) error {
	if !c.Configured() {
		return ErrNotConfigured
	}
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 60<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return responseError(path, resp, raw)
	}
	// Request > 45 dtk diawali spasi penjaga koneksi; json.Unmarshal menerima spasi di depan.
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("jawaban kenari %s bukan JSON: %s", path, truncate(strings.TrimSpace(string(raw)), 300))
	}
	return nil
}

// responseError membaca error gaya OpenAI {"error":{"message","type","code"}};
// kalau body berupa teks biasa (401, 413), teksnya dipakai apa adanya.
func responseError(path string, resp *http.Response, raw []byte) *APIError {
	e := &APIError{Path: path, Status: resp.StatusCode, RetryAfter: resp.Header.Get("Retry-After")}
	var body struct {
		Error   json.RawMessage `json:"error"`
		Message string          `json:"message"`
	}
	if json.Unmarshal(raw, &body) == nil {
		var obj apiError
		var str string
		switch {
		case json.Unmarshal(body.Error, &obj) == nil && obj.Message != "":
			e.Message = obj.Message
			if obj.Code != nil {
				e.Code = fmt.Sprint(obj.Code)
			}
		case json.Unmarshal(body.Error, &str) == nil && str != "":
			e.Message = str
		case body.Message != "":
			e.Message = body.Message
		}
	}
	if e.Message == "" {
		e.Message = truncate(strings.TrimSpace(string(raw)), 400)
	}
	return e
}

// ListModels mengambil id model dari GET {base}/models (publik di kenari). endpoint
// ("chat" / "images") menyaring per kolom "endpoints"; entri tanpa kolom itu tetap ikut.
func (c *Client) ListModels(ctx context.Context, endpoint string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, responseError("/models", resp, raw)
	}
	var out struct {
		Data []struct {
			ID        string   `json:"id"`
			Endpoints []string `json:"endpoints"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	var ids []string
	for _, m := range out.Data {
		if endpoint == "" || m.Endpoints == nil || slices.Contains(m.Endpoints, endpoint) {
			ids = append(ids, m.ID)
		}
	}
	return ids, nil
}

// pickSize memilih ukuran yang umum didukung model gambar sesuai rasio canvas.
func pickSize(w, h int) string {
	if w <= 0 || h <= 0 {
		return "1024x1024"
	}
	r := float64(w) / float64(h)
	switch {
	case r > 1.2:
		return "1536x1024"
	case r < 0.83:
		return "1024x1536"
	default:
		return "1024x1024"
	}
}

func ratioLabel(w, h int) string {
	if w <= 0 || h <= 0 {
		return "1:1"
	}
	a, b := w, h
	for b != 0 {
		a, b = b, a%b
	}
	return fmt.Sprintf("%d:%d", w/a, h/a)
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

// ExtractJSON mengambil objek JSON pertama dari teks jawaban LLM (bisa dibungkus ```json).
func ExtractJSON(text string) (json.RawMessage, error) {
	t := strings.TrimSpace(text)
	if i := strings.Index(t, "```"); i >= 0 {
		rest := t[i+3:]
		rest = strings.TrimPrefix(rest, "json")
		if j := strings.Index(rest, "```"); j >= 0 {
			t = strings.TrimSpace(rest[:j])
		}
	}
	start := strings.IndexAny(t, "{[")
	if start < 0 {
		return nil, errors.New("jawaban AI bukan JSON")
	}
	// Cari penutup yang seimbang dengan memperhatikan string.
	depth, inStr, esc := 0, false, false
	for i := start; i < len(t); i++ {
		ch := t[i]
		if inStr {
			switch {
			case esc:
				esc = false
			case ch == '\\':
				esc = true
			case ch == '"':
				inStr = false
			}
			continue
		}
		switch ch {
		case '"':
			inStr = true
		case '{', '[':
			depth++
		case '}', ']':
			depth--
			if depth == 0 {
				raw := json.RawMessage(t[start : i+1])
				if !json.Valid(raw) {
					return nil, errors.New("JSON dari AI tidak valid")
				}
				return raw, nil
			}
		}
	}
	return nil, errors.New("JSON dari AI terpotong")
}
