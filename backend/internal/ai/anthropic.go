package ai

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// Anthropic: model Claude langsung lewat Claude API (SDK resmi), teks saja.
// Gambar di pesan (ContentPart image_url berupa data: URI atau URL) dikirim sebagai
// blok gambar Claude.
type Anthropic struct {
	APIKey string
	Effort string // low | medium | high | xhigh | max; kosong = default model
	client anthropic.Client
}

// Model yang menerima server-side fallback bentuk "default" saat Claude menolak
// (stop_reason "refusal"): permintaan dilayani ulang model lain di panggilan yang sama.
var anthropicFallbackModels = map[string]bool{
	"claude-sonnet-5-5": true, "claude-opus-5-5": true, "claude-opus-5": true, "claude-fable-5-1": true,
}

func NewAnthropic(apiKey, baseURL, effort string, timeout time.Duration) *Anthropic {
	// HTTP client dengan pencatat AI API Call Log (timeout tetap lewat WithRequestTimeout).
	opts := []option.RequestOption{option.WithAPIKey(apiKey), option.WithRequestTimeout(timeout),
		option.WithHTTPClient(&http.Client{Transport: callLogTransport{}})}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	return &Anthropic{APIKey: apiKey, Effort: effort, client: anthropic.NewClient(opts...)}
}

func (a *Anthropic) Configured() bool { return a.APIKey != "" }

func (a *Anthropic) Chat(ctx context.Context, model string, msgs []Message, maxTokens int) (string, error) {
	if !a.Configured() {
		return "", notConfigured("anthropic")
	}
	params := anthropic.MessageNewParams{
		Model: anthropic.Model(model),
		// Thinking adaptif ikut memakai max_tokens; jangan terlalu kecil supaya jawaban
		// (JSON layout) tidak terpotong.
		MaxTokens: int64(max(maxTokens, 16000)),
	}
	if a.Effort != "" {
		params.OutputConfig = anthropic.OutputConfigParam{Effort: anthropic.OutputConfigEffort(a.Effort)}
	}
	for _, m := range msgs {
		if m.Role == "system" {
			if s := contentString(m.Content); s != "" {
				params.System = append(params.System, anthropic.TextBlockParam{Text: s})
			}
			continue
		}
		blocks := anthropicBlocks(m.Content)
		if len(blocks) == 0 {
			continue
		}
		if m.Role == "assistant" {
			params.Messages = append(params.Messages, anthropic.NewAssistantMessage(blocks...))
		} else {
			params.Messages = append(params.Messages, anthropic.NewUserMessage(blocks...))
		}
	}
	var opts []option.RequestOption
	if anthropicFallbackModels[model] {
		opts = append(opts,
			option.WithHeaderAdd("anthropic-beta", "server-side-fallback-2026-07-01"),
			option.WithJSONSet("fallbacks", "default"))
	}
	resp, err := a.client.Messages.New(ctx, params, opts...)
	if err != nil {
		var apiErr *anthropic.Error
		if errors.As(err, &apiErr) {
			return "", &APIError{Provider: "anthropic", Path: "/v1/messages", Status: apiErr.StatusCode, Message: truncate(apiErr.Error(), 400)}
		}
		return "", fmt.Errorf("anthropic: %w", err)
	}
	if resp.StopReason == anthropic.StopReasonRefusal {
		return "", fmt.Errorf("Claude menolak permintaan ini (kategori: %s) %s", nz(string(resp.StopDetails.Category), "-"), resp.StopDetails.Explanation)
	}
	var b strings.Builder
	for _, block := range resp.Content {
		if t, ok := block.AsAny().(anthropic.TextBlock); ok {
			b.WriteString(t.Text)
		}
	}
	text := strings.TrimSpace(b.String())
	if text == "" && resp.StopReason == anthropic.StopReasonMaxTokens {
		return "", fmt.Errorf("model %s menghabiskan batas token sebelum menulis jawaban", model)
	}
	return text, nil
}

func contentString(c any) string {
	switch v := c.(type) {
	case string:
		return v
	case []ContentPart:
		var b strings.Builder
		for _, p := range v {
			b.WriteString(p.Text)
		}
		return b.String()
	}
	return ""
}

func anthropicBlocks(c any) []anthropic.ContentBlockParamUnion {
	switch v := c.(type) {
	case string:
		if v == "" {
			return nil
		}
		return []anthropic.ContentBlockParamUnion{anthropic.NewTextBlock(v)}
	case []ContentPart:
		var out []anthropic.ContentBlockParamUnion
		for _, p := range v {
			switch {
			case p.Type == "text" && p.Text != "":
				out = append(out, anthropic.NewTextBlock(p.Text))
			case p.ImageURL != nil && strings.HasPrefix(p.ImageURL.URL, "data:"):
				if m := dataURLRe.FindStringSubmatch(p.ImageURL.URL); m != nil {
					out = append(out, anthropic.NewImageBlockBase64(m[1], m[2]))
				}
			case p.ImageURL != nil && p.ImageURL.URL != "":
				out = append(out, anthropic.NewImageBlock(anthropic.URLImageSourceParam{URL: p.ImageURL.URL}))
			}
		}
		return out
	}
	return nil
}
