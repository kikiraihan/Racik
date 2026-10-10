package ai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
)

var pngBytes, _ = base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==")

func TestExtractJSON(t *testing.T) {
	cases := []string{
		"```json\n{\"pages\":[{\"a\":\"}\"}]}\n```",
		"Berikut desainnya: {\"pages\":[1]} semoga suka",
		`{"pages":[]}`,
	}
	for _, c := range cases {
		raw, err := ExtractJSON(c)
		if err != nil || !json.Valid(raw) {
			t.Fatalf("ExtractJSON(%q) = %s, %v", c, raw, err)
		}
	}
	if _, err := ExtractJSON("tidak ada json"); err == nil {
		t.Fatal("harusnya error")
	}
}

func TestChatAndImages(t *testing.T) {
	b64 := base64.StdEncoding.EncodeToString(pngBytes)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer k" {
			w.WriteHeader(401)
			return
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		switch r.URL.Path {
		case "/chat/completions":
			if body["model"] == "nano-banana" { // model gambar via chat
				json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{
					"content": "ok", "images": []any{map[string]any{"image_url": map[string]any{"url": "data:image/png;base64," + b64}}}}}}})
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": "halo"}}}})
		case "/images/generations":
			if body["model"] == "nano-banana" {
				w.WriteHeader(404)
				w.Write([]byte(`{"error":{"message":"not supported"}}`))
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"b64_json": b64}}})
		}
	}))
	defer srv.Close()
	c := New(srv.URL, "k", 5*time.Second)
	ctx := context.Background()

	if txt, err := c.Chat(ctx, "claude", []Message{{Role: "user", Content: "hi"}}, 10); err != nil || txt != "halo" {
		t.Fatalf("chat: %q %v", txt, err)
	}
	for _, m := range []string{"gpt-image-1", "nano-banana"} {
		img, err := c.GenerateImage(ctx, m, "kucing", 1080, 1920)
		if err != nil || img.Mime != "image/png" {
			t.Fatalf("%s: %v", m, err)
		}
	}
}

// Perilaku khas kenari.id: data: URI di "url", 200 + spasi lalu objek error untuk
// request lama, error JSON {error:{message,code}}, dan tidak ada fallback chat saat
// saldo habis (402).
func TestKenariQuirks(t *testing.T) {
	b64 := base64.StdEncoding.EncodeToString(pngBytes)
	chatCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		switch {
		case r.URL.Path == "/models":
			w.Write([]byte(`{"data":[{"id":"gpt-image-2","endpoints":["images"]},{"id":"step:free","endpoints":["chat"]},{"id":"kenari/auto"}]}`))
		case r.URL.Path == "/chat/completions":
			chatCalls++
			if body["model"] == "thinker" {
				w.Write([]byte(`{"choices":[{"message":{"content":"<think>{bukan json}</think>\n{\"pages\":[1]}"}}]}`))
				return
			}
			w.Write([]byte(`{"choices":[{"message":{"content":"","reasoning_content":"hmm"},"finish_reason":"length"}]}`))
		case body["model"] == "data-uri":
			w.Write([]byte(`{"data":[{"url":"data:image/png;base64,` + b64 + `"}]}`))
		case body["model"] == "slow-fail":
			w.Write([]byte("   \n  " + `{"error":{"message":"upstream gagal","code":"all_providers_failed"}}`))
		case body["model"] == "broke":
			w.WriteHeader(402)
			w.Write([]byte(`{"error":{"message":"Saldo tidak cukup","type":"billing","code":"insufficient_balance"}}`))
		}
	}))
	defer srv.Close()
	c := New(srv.URL, "k", 5*time.Second)
	ctx := context.Background()

	if img, err := c.GenerateImage(ctx, "data-uri", "x", 1080, 1080); err != nil || img.Mime != "image/png" {
		t.Fatalf("data URI: %v", err)
	}
	if _, err := c.GenerateImage(ctx, "slow-fail", "x", 1080, 1080); err == nil || !strings.Contains(err.Error(), "all_providers_failed") {
		t.Fatalf("200+error: %v", err)
	}
	_, err := c.GenerateImage(ctx, "broke", "x", 1080, 1080)
	var ae *APIError
	if !errors.As(err, &ae) || ae.Status != 402 || ae.Code != "insufficient_balance" {
		t.Fatalf("402: %v", err)
	}
	if chatCalls != 0 {
		t.Fatalf("fallback chat tidak boleh jalan untuk 402 / 200+error, terpanggil %d kali", chatCalls)
	}
	if txt, err := c.Chat(ctx, "thinker", nil, 10); err != nil || txt != `{"pages":[1]}` {
		t.Fatalf("strip think: %q %v", txt, err)
	}
	if _, err := c.Chat(ctx, "reasoner", nil, 10); err == nil || !strings.Contains(err.Error(), "reasoning") {
		t.Fatalf("reasoning habis: %v", err)
	}
	ids, err := c.ListModels(ctx, "images")
	if err != nil || !slices.Equal(ids, []string{"gpt-image-2", "kenari/auto"}) {
		t.Fatalf("ListModels: %v %v", ids, err)
	}
}
