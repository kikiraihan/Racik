package ai

import (
	"strings"
	"time"
)

// Preset provider OpenAI-compatible, mengikuti konektor di katilidev (app/Ai/*Client.php,
// app/Ai/AiModels.php). Default & cadangan model di sini sama dengan katilidev.

const (
	GroqBaseURL       = "https://api.groq.com/openai/v1"
	NvidiaTextBaseURL = "https://integrate.api.nvidia.com/v1"
	AgnesBaseURL      = "https://apihub.agnes-ai.com/v1"
	OpenRouterBaseURL = "https://openrouter.ai/api/v1"

	GroqTextDefault    = "openai/gpt-oss-120b"
	NvidiaTextDefault  = "meta/llama-3.3-70b-instruct"
	AgnesTextFallback  = "agnes-2.5-flash"
	AgnesImageFallback = "agnes-image-2.1-flash"
)

// NewGroq: teks saja. GPT-OSS diturunkan usaha reasoning-nya supaya batas token tidak
// habis untuk berpikir. Model yang dicabut Groq (400/404) jatuh ke GroqTextDefault.
func NewGroq(apiKey string, timeout time.Duration) *Client {
	c := NewOpenAI("groq", GroqBaseURL, apiKey, timeout)
	c.TextFallback = GroqTextDefault
	c.TextFallbackStatuses = []int{400, 404}
	c.ChatExtra = func(model string) map[string]any {
		if strings.HasPrefix(model, "openai/gpt-oss") {
			return map[string]any{"reasoning_effort": "low"}
		}
		return nil
	}
	return c
}

// nvidiaTextExtra: field per model untuk mematikan mode berpikir (katilidev NVIDIA_TEXT).
var nvidiaTextExtra = map[string]map[string]any{
	"nvidia/nemotron-3-super-120b-a12b": {"chat_template_kwargs": map[string]any{"enable_thinking": false}},
	"deepseek-ai/deepseek-v3.2":         {"chat_template_kwargs": map[string]any{"enable_thinking": false}},
	"deepseek-ai/deepseek-v4-flash":     {"chat_template_kwargs": map[string]any{"thinking": false}},
	"qwen/qwen3.5-122b-a10b":            {"chat_template_kwargs": map[string]any{"enable_thinking": false}},
	"google/gemma-4-31b-it":             {"chat_template_kwargs": map[string]any{"enable_thinking": false}},
}

// NewNvidiaText: teks lewat endpoint OpenAI-compatible integrate.api.nvidia.com.
// Gambar NVIDIA memakai endpoint GenAI yang berbeda — lihat nvidia.go.
func NewNvidiaText(apiKey string, timeout time.Duration) *Client {
	c := NewOpenAI("nvidia", NvidiaTextBaseURL, apiKey, timeout)
	c.TextFallback = NvidiaTextDefault
	c.ChatExtra = func(model string) map[string]any { return nvidiaTextExtra[model] }
	c.PrepareMessages = func(model string, msgs []Message) []Message {
		// Nemotron Super 49B mematikan mode berpikir lewat baris "/no_think" di pesan system.
		if model == "nvidia/llama-3.3-nemotron-super-49b-v1.5" {
			msgs = withSystemPrefix(msgs, "/no_think")
		}
		// Template chat Gemma 3 di NIM tidak mengenal role "system".
		if strings.HasPrefix(model, "google/gemma-3") {
			msgs = mergeSystem(msgs)
		}
		return msgs
	}
	return c
}

// NewAgnes: teks + gambar. Gambar diminta sebagai kelas ukuran + rasio (paket gratis
// membatasi RPM per kelas), `return_base64: true`; jangan kirim response_format di
// tingkat atas (ditolak). Model yang ditolak jatuh ke model 2.x dari katalog publik Agnes.
func NewAgnes(baseURL, apiKey string, timeout time.Duration) *Client {
	c := NewOpenAI("agnes", baseURL, apiKey, timeout)
	c.TextFallback = AgnesTextFallback
	c.ImageFallback = AgnesImageFallback
	c.NoChatImageFallback = true
	c.ChatExtra = func(model string) map[string]any {
		if model == "agnes-3.0-flash" {
			return map[string]any{"chat_template_kwargs": map[string]any{"enable_thinking": false}}
		}
		return nil
	}
	c.ImagePayload = func(model, prompt string, w, h int) map[string]any {
		return map[string]any{
			"model":         model,
			"prompt":        strings.Join(strings.Fields(prompt), " "),
			"size":          "1K",
			"ratio":         nearestRatio(w, h, agnesRatios),
			"return_base64": true,
		}
	}
	return c
}

var agnesRatios = map[string][2]int{
	"1:1": {1024, 1024}, "3:4": {864, 1152}, "4:3": {1152, 864}, "16:9": {1312, 736},
	"9:16": {736, 1312}, "2:3": {832, 1248}, "3:2": {1248, 832}, "21:9": {1568, 672},
}

// NewOpenRouter: teks + gambar. Model gambar OpenRouter hanya lewat chat completions
// (modalities image+text); edit gambar juga lewat chat dengan gambar sebagai input.
func NewOpenRouter(baseURL, apiKey, referer, title string, timeout time.Duration) *Client {
	c := NewOpenAI("openrouter", baseURL, apiKey, timeout)
	c.ImagesViaChat = true
	c.Headers = map[string]string{}
	if referer != "" {
		c.Headers["HTTP-Referer"] = referer
	}
	if title != "" {
		c.Headers["X-Title"] = title
	}
	return c
}

func withSystemPrefix(msgs []Message, prefix string) []Message {
	out := slicesClone(msgs)
	for i, m := range out {
		if s, ok := m.Content.(string); ok && m.Role == "system" {
			out[i].Content = prefix + "\n" + s
			return out
		}
	}
	return append([]Message{{Role: "system", Content: prefix}}, out...)
}

// mergeSystem menggabungkan isi pesan system ke awal pesan user pertama.
func mergeSystem(msgs []Message) []Message {
	var system []string
	var rest []Message
	for _, m := range msgs {
		if s, ok := m.Content.(string); ok && m.Role == "system" {
			system = append(system, s)
		} else {
			rest = append(rest, m)
		}
	}
	if len(system) == 0 {
		return msgs
	}
	sys := strings.Join(system, "\n\n")
	for i, m := range rest {
		if s, ok := m.Content.(string); ok && m.Role == "user" {
			rest[i].Content = sys + "\n\n" + s
			return rest
		}
	}
	return append([]Message{{Role: "user", Content: sys}}, rest...)
}

func slicesClone(msgs []Message) []Message { return append([]Message(nil), msgs...) }
