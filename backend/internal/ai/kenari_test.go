package ai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
