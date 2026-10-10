package applog

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestBulkAndSearch(t *testing.T) {
	var (
		mu     sync.Mutex
		lines  []string
		search map[string]any
		paths  []string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		paths = append(paths, r.Method+" "+r.URL.Path)
		if u, p, _ := r.BasicAuth(); u != "elastic" || p != "rahasia" {
			t.Errorf("basic auth tidak terkirim: %q %q", u, p)
		}
		switch {
		case r.URL.Path == "/_bulk":
			if ct := r.Header.Get("Content-Type"); ct != "application/x-ndjson" {
				t.Errorf("content-type bulk = %q", ct)
			}
			sc := bufio.NewScanner(r.Body)
			for sc.Scan() {
				lines = append(lines, sc.Text())
			}
			w.Write([]byte(`{"errors":false,"items":[]}`))
		case strings.HasSuffix(r.URL.Path, "/_search"):
			b, _ := io.ReadAll(r.Body)
			json.Unmarshal(b, &search)
			w.Write([]byte(`{"hits":{"total":{"value":1},"hits":[{"_id":"x1","_source":{"action":"auth.login","success":false}}]}}`))
		default:
			w.Write([]byte(`{"acknowledged":true}`))
		}
	}))
	defer srv.Close()

	l := New(Config{URL: srv.URL, User: "elastic", Password: "rahasia", IndexPrefix: "racik", RetentionDays: 30})
	l.now = func() time.Time { return time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC) }
	if err := l.Setup(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go l.Run(ctx)
	l.Activity(Event{"action": "auth.login", "success": false})
	l.Vendor(Event{"provider": "kenari", "model": "gpt-image-2"})
	cancel()
	l.Wait(5 * time.Second)

	mu.Lock()
	if len(lines) != 4 {
		t.Fatalf("bulk lines = %d, mau 4: %v", len(lines), lines)
	}
	if !strings.Contains(lines[0], `"racik-activity-2026.10"`) || !strings.Contains(lines[2], `"racik-vendor-2026.10"`) {
		t.Errorf("index salah: %v", lines)
	}
	if !strings.Contains(lines[1], `"@timestamp":"2026-10-10T08:00:00Z"`) {
		t.Errorf("timestamp tidak terisi: %s", lines[1])
	}
	joined := strings.Join(paths, ",")
	if !strings.Contains(joined, "PUT /_ilm/policy/racik-logs") || !strings.Contains(joined, "PUT /_index_template/racik-logs") {
		t.Errorf("setup tidak membuat policy/template: %s", joined)
	}
	mu.Unlock()

	ok := false
	res, err := l.Search(context.Background(), Query{Kind: KindActivity, Action: "auth.login", Success: &ok, Text: "budi", Size: 20})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 || res.Items[0]["_id"] != "x1" {
		t.Errorf("hasil search = %+v", res)
	}
	b, _ := json.Marshal(search)
	for _, want := range []string{`"action":"auth.login"`, `"success":false`, `"query":"budi"`, `"size":20`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("query search tidak memuat %s: %s", want, b)
		}
	}
}

func TestDisabledWritesStdout(t *testing.T) {
	l := New(Config{})
	if l.Enabled() {
		t.Fatal("tanpa URL harus nonaktif")
	}
	if _, err := l.Search(context.Background(), Query{}); err != ErrDisabled {
		t.Errorf("search tanpa ES = %v", err)
	}
}
