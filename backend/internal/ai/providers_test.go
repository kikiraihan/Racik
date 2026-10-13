package ai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kikiraihan/racik/backend/internal/config"
)

// recorder menyimpan body request terakhir per path.
type recorder struct {
	bodies  map[string]map[string]any
	headers map[string]http.Header
	paths   []string
}

func newRecorder() *recorder {
	return &recorder{bodies: map[string]map[string]any{}, headers: map[string]http.Header{}}
}

func (rec *recorder) record(r *http.Request) map[string]any {
	var body map[string]any
	raw, _ := io.ReadAll(r.Body)
	json.Unmarshal(raw, &body)
	rec.bodies[r.URL.Path] = body
	rec.headers[r.URL.Path] = r.Header.Clone()
	rec.paths = append(rec.paths, r.URL.Path)
	return body
}

func TestGroqFallbackAndExtra(t *testing.T) {
	rec := newRecorder()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := rec.record(r)
		if body["model"] == "retired" {
			w.WriteHeader(400)
			w.Write([]byte(`{"error":{"message":"model decommissioned","code":"model_decommissioned"}}`))
			return
		}
		w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()
	g := NewGroq("k", 5*time.Second)
	g.BaseURL = srv.URL
	txt, err := g.Chat(context.Background(), "retired", []Message{{Role: "user", Content: "hi"}}, 50)
	if err != nil || txt != "ok" {
		t.Fatalf("fallback: %q %v", txt, err)
	}
	body := rec.bodies["/chat/completions"]
	if body["model"] != GroqTextDefault || body["reasoning_effort"] != "low" {
		t.Fatalf("body cadangan: %v", body)
	}
}

func TestNvidiaTextPrepare(t *testing.T) {
	msgs := []Message{{Role: "system", Content: "SYS"}, {Role: "user", Content: "Q"}}
	merged := NewNvidiaText("k", time.Second).PrepareMessages("google/gemma-3-27b-it", msgs)
	if len(merged) != 1 || merged[0].Role != "user" || merged[0].Content != "SYS\n\nQ" {
		t.Fatalf("merge system: %+v", merged)
	}
	pre := NewNvidiaText("k", time.Second).PrepareMessages("nvidia/llama-3.3-nemotron-super-49b-v1.5", msgs)
	if pre[0].Content != "/no_think\nSYS" || msgs[0].Content != "SYS" {
		t.Fatalf("no_think: %+v (asli %+v)", pre, msgs)
	}
}

func TestNvidiaTextExtra(t *testing.T) {
	c := NewNvidiaText("k", time.Second)
	if e := c.ChatExtra("openai/gpt-oss-20b"); e["reasoning_effort"] != "low" {
		t.Fatalf("gpt-oss: %v", e)
	}
	kw, _ := c.ChatExtra("nvidia/nemotron-3.5-lightning-30b-a3b")["chat_template_kwargs"].(map[string]any)
	if kw["enable_thinking"] != false {
		t.Fatalf("lightning: %v", kw)
	}
	if c.ChatExtra("model/lain") != nil {
		t.Fatal("model tanpa extra harus nil")
	}
}

func TestNvidiaImage(t *testing.T) {
	b64 := base64.StdEncoding.EncodeToString(pngBytes)
	rec := newRecorder()
	polls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := rec.record(r)
		switch {
		case strings.HasPrefix(r.URL.Path, "/status/"):
			if polls++; polls < 2 {
				w.WriteHeader(202)
				return
			}
			w.Write([]byte(`{"artifacts":[{"base64":"` + b64 + `","finishReason":"SUCCESS"}]}`))
		case r.URL.Path == "/genai/black-forest-labs/flux.1-dev":
			w.Header().Set("NVCF-REQID", "abc")
			w.WriteHeader(202)
		case r.URL.Path == "/genai/gone":
			w.WriteHeader(404)
			w.Write([]byte(`{"detail":"not found"}`))
		default:
			_ = body
			w.Write([]byte(`{"artifacts":[{"base64":"` + b64 + `","finishReason":"SUCCESS"}]}`))
		}
	}))
	defer srv.Close()
	n := NewNvidiaImage("k", 4, 5*time.Second)
	n.BaseURL, n.NVCFStatusURL, n.PollEvery = srv.URL+"/genai/", srv.URL+"/status/", time.Millisecond

	if _, err := n.GenerateImage(context.Background(), "black-forest-labs/flux.1-dev", "kucing", 1080, 1920); err != nil {
		t.Fatalf("202 polling: %v", err)
	}
	body := rec.bodies["/genai/black-forest-labs/flux.1-dev"]
	if body["width"] != float64(1024) || body["height"] != float64(1344) || body["mode"] != "base" {
		t.Fatalf("payload flux.1-dev: %v", body)
	}
	if _, err := n.GenerateImage(context.Background(), "gone", strings.Repeat("kata ", 300), 1216, 832); err != nil {
		t.Fatalf("fallback: %v", err)
	}
	fb := rec.bodies["/genai/"+NvidiaImageDefault]
	if fb["width"] != float64(1216) || fb["height"] != float64(832) || len(fb["prompt"].(string)) > 800 {
		t.Fatalf("payload flux.2: %v", fb)
	}
	if w, h := fitNvidiaImageSize(1344, 896); w*h > nvidiaImageMaxPixels || w%64 != 0 || h%64 != 0 {
		t.Fatalf("fit size: %dx%d", w, h)
	}
}

func TestAgnesImagePayload(t *testing.T) {
	b64 := base64.StdEncoding.EncodeToString(pngBytes)
	rec := newRecorder()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := rec.record(r)
		if body["model"] == "old" {
			w.WriteHeader(422)
			w.Write([]byte(`{"message":"model tidak ada","error_code":"MODEL_NOT_FOUND"}`))
			return
		}
		w.Write([]byte(`{"data":[{"base64":"data:image/png;base64,` + b64 + `"}]}`))
	}))
	defer srv.Close()
	a := NewAgnes(srv.URL, "k", 5*time.Second)
	if _, err := a.GenerateImage(context.Background(), "old", "x", 1080, 1350); err != nil {
		t.Fatalf("agnes: %v", err)
	}
	body := rec.bodies["/images/generations"]
	if body["model"] != AgnesImageFallback || body["ratio"] != "3:4" || body["size"] != "1K" || body["return_base64"] != true || body["response_format"] != nil {
		t.Fatalf("payload agnes: %v", body)
	}
	for _, p := range rec.paths {
		if p == "/chat/completions" {
			t.Fatal("agnes tidak boleh jatuh ke chat")
		}
	}
}

func TestOpenRouterImageViaChat(t *testing.T) {
	b64 := base64.StdEncoding.EncodeToString(pngBytes)
	rec := newRecorder()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.record(r)
		w.Write([]byte(`{"choices":[{"message":{"content":"","images":[{"image_url":{"url":"data:image/png;base64,` + b64 + `"}}]}}]}`))
	}))
	defer srv.Close()
	o := NewOpenRouter(srv.URL, "k", "https://racik.test", "Racik", 5*time.Second)
	if _, err := o.GenerateImage(context.Background(), "flux", "x", 1024, 1024); err != nil {
		t.Fatal(err)
	}
	if len(rec.paths) != 1 || rec.paths[0] != "/chat/completions" || rec.headers["/chat/completions"].Get("HTTP-Referer") != "https://racik.test" {
		t.Fatalf("openrouter: %v %v", rec.paths, rec.headers)
	}
	if m := rec.bodies["/chat/completions"]["modalities"]; m == nil {
		t.Fatal("modalities tidak dikirim")
	}
}

func TestHuggingFaceAndPollinations(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/prompt/"):
			gotQuery = r.URL.RawQuery
			w.Write(pngBytes)
		case strings.HasSuffix(r.URL.Path, "/loading"):
			w.WriteHeader(503)
			w.Write([]byte(`{"error":"Model is currently loading"}`))
		default:
			w.Write(pngBytes)
		}
	}))
	defer srv.Close()
	hf := NewHuggingFace(srv.URL, "k", 5*time.Second)
	if img, err := hf.GenerateImage(context.Background(), "org/model", "x", 1080, 1080); err != nil || img.Mime != "image/png" {
		t.Fatalf("hf: %v", err)
	}
	if _, err := hf.GenerateImage(context.Background(), "org/loading", "x", 1080, 1080); err == nil || !strings.Contains(err.Error(), "sedang dimuat") {
		t.Fatalf("hf 503: %v", err)
	}
	p := NewPollinations(true, 5*time.Second)
	p.BaseURL = srv.URL + "/prompt/"
	if _, err := p.GenerateImage(context.Background(), "flux", "kucing lucu", 1200, 800); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotQuery, "width=1200") || !strings.Contains(gotQuery, "model=flux") || !strings.Contains(gotQuery, "nologo=true") {
		t.Fatalf("query pollinations: %s", gotQuery)
	}
}

func TestAnthropicChat(t *testing.T) {
	rec := newRecorder()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.record(r)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-5-5",
			"content":[{"type":"thinking","thinking":"","signature":"s"},{"type":"text","text":"{\"pages\":[1]}"}],
			"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
	defer srv.Close()
	a := NewAnthropic("k", srv.URL, "medium", 5*time.Second)
	msgs := []Message{{Role: "system", Content: "SYS"}, {Role: "user", Content: []ContentPart{
		{Type: "text", Text: "lihat"}, {Type: "image_url", ImageURL: &ImageURL{URL: "data:image/png;base64,AAAA"}}}}}
	txt, err := a.Chat(context.Background(), "claude-sonnet-5-5", msgs, 8000)
	if err != nil || txt != `{"pages":[1]}` {
		t.Fatalf("anthropic: %q %v", txt, err)
	}
	body := rec.bodies["/v1/messages"]
	if body["fallbacks"] != "default" || body["max_tokens"] != float64(16000) || body["system"] == nil {
		t.Fatalf("body anthropic: %v", body)
	}
	if !strings.Contains(rec.headers["/v1/messages"].Get("anthropic-beta"), "server-side-fallback-2026-07-01") {
		t.Fatal("beta fallback tidak dikirim")
	}
	content := body["messages"].([]any)[0].(map[string]any)["content"].([]any)
	if content[1].(map[string]any)["type"] != "image" {
		t.Fatalf("gambar tidak dikirim sebagai blok image: %v", content)
	}
}

func TestRouterFromConfig(t *testing.T) {
	cfg := config.Config{AITimeoutSec: 5, KenariBaseURL: "http://x", KenariTextModel: "claude-sonnet-5-5",
		KenariTextModels: []string{"claude-sonnet-5-5"}, KenariImageModel: "gpt-image-2", KenariImageModels: []string{"gpt-image-2"},
		Groq:         config.Provider{APIKey: "g", TextModels: []string{"openai/gpt-oss-120b"}},
		HuggingFace:  config.Provider{APIKey: "h", BaseURL: "http://hf", ImageModels: []string{"org/m"}},
		Pollinations: config.Provider{Enabled: false, ImageModels: []string{"flux"}},
	}
	// Tanpa kenari: default jatuh ke provider aktif pertama.
	r := FromConfig(cfg)
	if r.DefaultText() != "groq:openai/gpt-oss-120b" || r.DefaultImage() != "huggingface:org/m" {
		t.Fatalf("default tanpa kenari: %s %s", r.DefaultText(), r.DefaultImage())
	}
	if len(r.EditChoices()) != 0 {
		t.Fatalf("hf tidak bisa edit: %v", r.EditChoices())
	}
	cfg.KenariAPIKey = "k"
	r = FromConfig(cfg)
	if r.DefaultText() != "kenari:claude-sonnet-5-5" || r.TextChoices()[1] != "groq:openai/gpt-oss-120b" {
		t.Fatalf("default kenari: %s %v", r.DefaultText(), r.TextChoices())
	}
	if r.Labels()["groq:openai/gpt-oss-120b"] != "openai/gpt-oss-120b · Groq" {
		t.Fatalf("label: %v", r.Labels())
	}
	if p, m := SplitChoice("kenari:step-3-7-flash:free"); p != "kenari" || m != "step-3-7-flash:free" {
		t.Fatalf("split: %s %s", p, m)
	}
	// Tanpa provider sama sekali: error tetap menyebut KENARI_API_KEY.
	empty := FromConfig(config.Config{AITimeoutSec: 5, KenariTextModel: "x"})
	if _, err := empty.Chat(context.Background(), empty.DefaultText(), nil, 10); err == nil || !strings.Contains(err.Error(), "KENARI_API_KEY") {
		t.Fatalf("tanpa provider: %v", err)
	}
}
