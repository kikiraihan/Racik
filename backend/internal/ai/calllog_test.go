package ai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type memRecorder struct {
	mu   sync.Mutex
	recs []CallRecord
}

func (m *memRecorder) RecordCall(_ context.Context, c CallRecord) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recs = append(m.recs, c)
}

func TestCallLog(t *testing.T) {
	b64 := base64.StdEncoding.EncodeToString(pngBytes)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/chat/completions":
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if body["model"] == "rusak" { // model ditolak → cadangan dicoba
				w.WriteHeader(404)
				w.Write([]byte(`{"error":{"message":"model tidak ada","code":"model_not_found"}}`))
				return
			}
			json.NewEncoder(w).Encode(map[string]any{
				"choices": []any{map[string]any{"message": map[string]any{"content": "halo"}}},
				"usage":   map[string]any{"prompt_tokens": 12, "completion_tokens": 3, "total_tokens": 15, "cost": 0.0002},
			})
		case "/images/generations":
			// kenari: request lama gagal di tengah jalan → 200 + objek error
			w.Write([]byte("   " + `{"error":{"message":"saldo habis","code":"insufficient_balance"}}`))
		case "/images/edits":
			json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"b64_json": b64}}})
		}
	}))
	defer srv.Close()

	c := New(srv.URL, "k", 5*time.Second)
	c.TextFallback = "cadangan"
	rec := &memRecorder{}
	r := NewRouter()
	r.Add(&Provider{Name: "kenari", Label: "kenari", Text: c, Image: c, Edit: c, TextModels: []string{"rusak"}, ImageModels: []string{"img"}})
	r.Recorder, r.LogBodies = rec, true
	ctx := context.Background()

	if text, err := r.Chat(ctx, "kenari:rusak", []Message{{Role: "user", Content: "hai"}}, 100); err != nil || text != "halo" {
		t.Fatalf("chat = %q, %v", text, err)
	}
	if _, err := r.GenerateImage(ctx, "kenari:img", "kucing", 512, 512); err == nil {
		t.Fatal("generate harusnya gagal")
	}
	if _, err := r.EditImage(ctx, "kenari:img", "jadi biru", &Image{Data: pngBytes, Mime: "image/png"}); err != nil {
		t.Fatal(err)
	}
	// tanpa scope router (mis. daftar /models) tidak dicatat
	c.ListModels(ctx, "")

	if len(rec.recs) != 4 {
		t.Fatalf("jumlah record = %d, mau 4: %+v", len(rec.recs), rec.recs)
	}
	rejected, fallback, gen, edit := rec.recs[0], rec.recs[1], rec.recs[2], rec.recs[3]
	if rejected.OK || rejected.HTTPStatus != 404 || rejected.Model != "rusak" || rejected.Error != "model tidak ada" {
		t.Fatalf("record model ditolak salah: %+v", rejected)
	}
	if !fallback.OK || fallback.Model != "cadangan" || fallback.Operation != "chat" || fallback.Provider != "kenari" ||
		fallback.TotalTokens != 15 || fallback.PromptTokens != 12 || fallback.Cost == nil || *fallback.Cost != 0.0002 {
		t.Fatalf("record cadangan salah: %+v", fallback)
	}
	if gen.OK || gen.HTTPStatus != 200 || gen.Error != "saldo habis" || gen.Endpoint != "/images/generations" {
		t.Fatalf("record 200+error salah: %+v", gen)
	}
	if !edit.OK || edit.Operation != "edit" || edit.Model != "img" {
		t.Fatalf("record edit salah: %+v", edit)
	}
	// gambar base64 tidak ikut tersimpan
	reqJSON, _ := json.Marshal(edit.Request)
	respJSON, _ := json.Marshal(edit.Response)
	if !strings.Contains(string(reqJSON), `"multipart":true`) || !strings.Contains(string(reqJSON), `"prompt":"jadi biru"`) {
		t.Fatalf("ringkasan request edit salah: %s", reqJSON)
	}
	if strings.Contains(string(respJSON), b64) || !strings.Contains(string(respJSON), "tidak disimpan") {
		t.Fatalf("base64 jawaban harusnya diringkas: %s", respJSON)
	}
	msgJSON, _ := json.Marshal(fallback.Request)
	if !strings.Contains(string(msgJSON), `"content":"hai"`) {
		t.Fatalf("request chat harusnya memuat pesan: %s", msgJSON)
	}

	// LogBodies false: body tidak disimpan
	r.LogBodies = false
	rec.recs = nil
	r.Chat(ctx, "kenari:cadangan", []Message{{Role: "user", Content: "hai"}}, 100)
	if len(rec.recs) != 1 || rec.recs[0].Request != nil || rec.recs[0].Response != nil {
		t.Fatalf("body harusnya tidak disimpan: %+v", rec.recs)
	}
}
