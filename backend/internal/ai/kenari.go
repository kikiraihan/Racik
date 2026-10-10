// Package ai adalah konektor ke gateway AI kenari.id.
//
// Gateway diasumsikan OpenAI-compatible:
//   - POST {base}/chat/completions      -> model teks (Claude, GPT, Gemini)
//   - POST {base}/images/generations    -> model gambar (gpt-image-1, Nano Banana)
//   - POST {base}/images/edits          -> edit gambar dengan prompt
//
// Untuk model gambar yang di gateway hanya tersedia lewat chat completions
// (mis. Gemini "Nano Banana"), client otomatis fallback dan mengambil gambar
// dari field message.images / data URL di content.
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
			Content json.RawMessage `json:"content"`
			Images  []struct {
				ImageURL ImageURL `json:"image_url"`
			} `json:"images"`
		} `json:"message"`
	} `json:"choices"`
	Error *apiError `json:"error"`
}

type apiError struct {
	Message string `json:"message"`
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
	return contentText(resp.Choices[0].Message.Content), nil
}

func (c *Client) chatRaw(ctx context.Context, body chatRequest) (*chatResponse, error) {
	var out chatResponse
	if err := c.postJSON(ctx, "/chat/completions", body, &out); err != nil {
		return nil, err
	}
	if out.Error != nil {
		return nil, errors.New(out.Error.Message)
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
	if err == nil && out.Error == nil && len(out.Data) > 0 {
		return c.decodeImage(ctx, out.Data[0].B64JSON, out.Data[0].URL)
	}
	// Fallback: model gambar via chat completions (gaya Gemini / OpenRouter).
	img, chatErr := c.imageViaChat(ctx, model, []ContentPart{{Type: "text", Text: prompt + fmt.Sprintf("\n\nAspect ratio approx %s.", ratioLabel(width, height))}})
	if chatErr == nil {
		return img, nil
	}
	if err == nil && out.Error != nil {
		err = errors.New(out.Error.Message)
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
	if err == nil {
		defer resp.Body.Close()
		var out imageResponse
		raw, _ := io.ReadAll(resp.Body)
		if resp.StatusCode < 300 && json.Unmarshal(raw, &out) == nil && len(out.Data) > 0 {
			return c.decodeImage(ctx, out.Data[0].B64JSON, out.Data[0].URL)
		}
		err = fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(raw), 300))
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
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
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
		return fmt.Errorf("kenari %s HTTP %d: %s", path, resp.StatusCode, truncate(string(raw), 400))
	}
	return json.Unmarshal(raw, out)
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
