// Package applog mengirim system log Racik ke Elasticsearch:
//
//   - activity: request user ke Racik untuk fitur yang riskan (login/daftar, hit model AI,
//     aksi admin) → index <prefix>-activity-YYYY.MM
//   - vendor: request Racik ke vendor AI (provider, model, durasi, error, harga vendor)
//     → index <prefix>-vendor-YYYY.MM
//
// Log dikirim asinkron lewat Bulk API dalam batch agar tidak memperlambat request. Kalau
// LOG_ES_URL kosong atau Elasticsearch gagal dihubungi, log ditulis ke stdout sebagai JSON
// per baris (tidak hilang, bisa dikumpulkan dari log container).
package applog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

const (
	KindActivity = "activity"
	KindVendor   = "vendor"

	batchSize     = 200
	flushInterval = 2 * time.Second
)

// Event adalah satu dokumen log. Field "@timestamp" diisi otomatis.
type Event map[string]any

type Config struct {
	URL           string // mis. http://elasticsearch:9200 (kosong = stdout)
	User          string
	Password      string
	APIKey        string
	IndexPrefix   string
	RetentionDays int
}

type Logger struct {
	cfg     Config
	http    *http.Client
	ch      chan entry
	dropped atomic.Int64
	done    chan struct{}
	now     func() time.Time
}

type entry struct {
	kind string
	doc  Event
}

func New(cfg Config) *Logger {
	if cfg.IndexPrefix == "" {
		cfg.IndexPrefix = "racik"
	}
	return &Logger{
		cfg:  cfg,
		http: &http.Client{Timeout: 15 * time.Second},
		ch:   make(chan entry, 5000),
		done: make(chan struct{}),
		now:  time.Now,
	}
}

// Enabled: log dikirim ke Elasticsearch (bukan hanya stdout).
func (l *Logger) Enabled() bool { return l.cfg.URL != "" }

func (l *Logger) Activity(e Event) { l.push(KindActivity, e) }
func (l *Logger) Vendor(e Event)   { l.push(KindVendor, e) }

func (l *Logger) push(kind string, e Event) {
	if _, ok := e["@timestamp"]; !ok {
		e["@timestamp"] = l.now().UTC().Format(time.RFC3339Nano)
	}
	select {
	case l.ch <- entry{kind, e}:
	default:
		// Antrian penuh (ES lambat/mati): jangan blok request user.
		if n := l.dropped.Add(1); n%100 == 1 {
			log.Printf("applog: antrian penuh, %d log dibuang", n)
		}
		writeStdout(kind, e)
	}
}

// Run memproses antrian sampai ctx selesai, lalu mengirim sisa log. Panggil di goroutine.
func (l *Logger) Run(ctx context.Context) {
	defer close(l.done)
	tick := time.NewTicker(flushInterval)
	defer tick.Stop()
	var buf []entry
	flush := func() {
		if len(buf) > 0 {
			l.flush(buf)
			buf = buf[:0]
		}
	}
	for {
		select {
		case e := <-l.ch:
			buf = append(buf, e)
			if len(buf) >= batchSize {
				flush()
			}
		case <-tick.C:
			flush()
		case <-ctx.Done():
			for {
				select {
				case e := <-l.ch:
					buf = append(buf, e)
				default:
					flush()
					return
				}
			}
		}
	}
}

// Wait menunggu Run selesai mengirim sisa log (dipakai saat shutdown).
func (l *Logger) Wait(timeout time.Duration) {
	select {
	case <-l.done:
	case <-time.After(timeout):
	}
}

func (l *Logger) index(kind string, t time.Time) string {
	return fmt.Sprintf("%s-%s-%s", l.cfg.IndexPrefix, kind, t.UTC().Format("2006.01"))
}

func (l *Logger) flush(batch []entry) {
	if !l.Enabled() {
		for _, e := range batch {
			writeStdout(e.kind, e.doc)
		}
		return
	}
	var body bytes.Buffer
	for _, e := range batch {
		t := l.now()
		if s, ok := e.doc["@timestamp"].(string); ok {
			if pt, err := time.Parse(time.RFC3339Nano, s); err == nil {
				t = pt
			}
		}
		meta, _ := json.Marshal(map[string]any{"create": map[string]string{"_index": l.index(e.kind, t)}})
		doc, err := json.Marshal(e.doc)
		if err != nil {
			continue
		}
		body.Write(meta)
		body.WriteByte('\n')
		body.Write(doc)
		body.WriteByte('\n')
	}
	payload := body.Bytes()
	var err error
	for attempt := 0; attempt < 2; attempt++ {
		if err = l.bulk(payload); err == nil {
			return
		}
		time.Sleep(time.Second)
	}
	log.Printf("applog: gagal kirim %d log ke Elasticsearch: %v (ditulis ke stdout)", len(batch), err)
	for _, e := range batch {
		writeStdout(e.kind, e.doc)
	}
}

func (l *Logger) bulk(payload []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var res struct {
		Errors bool `json:"errors"`
		Items  []map[string]struct {
			Status int `json:"status"`
			Error  any `json:"error"`
		} `json:"items"`
	}
	if err := l.do(ctx, http.MethodPost, "/_bulk", "application/x-ndjson", payload, &res); err != nil {
		return err
	}
	if res.Errors {
		// Dokumen yang ditolak (mis. mapping bentrok) tidak diulang supaya tidak macet.
		for _, it := range res.Items {
			for _, r := range it {
				if r.Error != nil {
					b, _ := json.Marshal(r.Error)
					log.Printf("applog: dokumen ditolak (%d): %s", r.Status, b)
					return nil
				}
			}
		}
	}
	return nil
}

func (l *Logger) do(ctx context.Context, method, path, contentType string, body []byte, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, l.cfg.URL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	switch {
	case l.cfg.APIKey != "":
		req.Header.Set("Authorization", "ApiKey "+l.cfg.APIKey)
	case l.cfg.User != "":
		req.SetBasicAuth(l.cfg.User, l.cfg.Password)
	}
	resp, err := l.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("elasticsearch %s %s: %d %s", method, path, resp.StatusCode, truncate(string(raw), 300))
	}
	if out != nil {
		return json.Unmarshal(raw, out)
	}
	return nil
}

// Setup membuat ILM policy (retensi) dan index template untuk index log. Gagal setup tidak
// menghentikan server; ES tetap bisa menerima log dengan mapping dinamis.
func (l *Logger) Setup(ctx context.Context) error {
	if !l.Enabled() {
		return nil
	}
	name := l.cfg.IndexPrefix + "-logs"
	settings := map[string]any{"number_of_shards": 1, "number_of_replicas": 0}
	if l.cfg.RetentionDays > 0 {
		policy := map[string]any{"policy": map[string]any{"phases": map[string]any{
			"hot":    map[string]any{"actions": map[string]any{}},
			"delete": map[string]any{"min_age": fmt.Sprintf("%dd", l.cfg.RetentionDays), "actions": map[string]any{"delete": map[string]any{}}},
		}}}
		b, _ := json.Marshal(policy)
		if err := l.do(ctx, http.MethodPut, "/_ilm/policy/"+name, "application/json", b, nil); err != nil {
			return err
		}
		settings["index.lifecycle.name"] = name
	}
	text := map[string]any{"type": "text"}
	tmpl := map[string]any{
		"index_patterns": []string{l.cfg.IndexPrefix + "-" + KindActivity + "-*", l.cfg.IndexPrefix + "-" + KindVendor + "-*"},
		"template": map[string]any{
			"settings": settings,
			"mappings": map[string]any{
				// String lain otomatis keyword (bisa difilter & diagregasi di Kibana).
				"dynamic_templates": []any{map[string]any{"strings_as_keyword": map[string]any{
					"match_mapping_type": "string",
					"mapping":            map[string]any{"type": "keyword", "ignore_above": 1024},
				}}},
				"properties": map[string]any{
					"@timestamp":  map[string]any{"type": "date"},
					"duration_ms": map[string]any{"type": "long"},
					"status":      map[string]any{"type": "integer"},
					"success":     map[string]any{"type": "boolean"},
					"prompt":      text,
					"error":       text,
					"user_agent":  text,
					"ip":          map[string]any{"type": "keyword"},
					// Detail tambahan per aksi, bentuknya bebas.
					"details": map[string]any{"type": "flattened"},
				},
			},
		},
	}
	b, _ := json.Marshal(tmpl)
	return l.do(ctx, http.MethodPut, "/_index_template/"+name, "application/json", b, nil)
}

// Query: filter pencarian log untuk halaman admin.
type Query struct {
	Kind    string // activity | vendor
	Text    string // pencarian bebas (simple_query_string)
	Action  string // activity: action, vendor: operation
	UserID  string
	Success *bool
	From    time.Time
	To      time.Time
	Page    int
	Size    int
}

type Result struct {
	Total int64   `json:"total"`
	Items []Event `json:"items"`
}

var ErrDisabled = errors.New("Elasticsearch belum dikonfigurasi (LOG_ES_URL kosong) — log ditulis ke stdout")

// Search mencari log terbaru di Elasticsearch.
func (l *Logger) Search(ctx context.Context, q Query) (*Result, error) {
	if !l.Enabled() {
		return nil, ErrDisabled
	}
	if q.Kind != KindVendor {
		q.Kind = KindActivity
	}
	q.Size = min(max(q.Size, 1), 200)
	q.Page = max(q.Page, 1)
	var filter []any
	if q.Action != "" {
		field := "action"
		if q.Kind == KindVendor {
			field = "operation"
		}
		filter = append(filter, map[string]any{"term": map[string]any{field: q.Action}})
	}
	if q.UserID != "" {
		filter = append(filter, map[string]any{"term": map[string]any{"user_id": q.UserID}})
	}
	if q.Success != nil {
		filter = append(filter, map[string]any{"term": map[string]any{"success": *q.Success}})
	}
	if !q.From.IsZero() || !q.To.IsZero() {
		rng := map[string]any{}
		if !q.From.IsZero() {
			rng["gte"] = q.From.UTC().Format(time.RFC3339)
		}
		if !q.To.IsZero() {
			rng["lte"] = q.To.UTC().Format(time.RFC3339)
		}
		filter = append(filter, map[string]any{"range": map[string]any{"@timestamp": rng}})
	}
	boolQ := map[string]any{"filter": filter}
	if strings.TrimSpace(q.Text) != "" {
		boolQ["must"] = []any{map[string]any{"simple_query_string": map[string]any{
			"query": q.Text, "default_operator": "and", "lenient": true,
		}}}
	}
	body, _ := json.Marshal(map[string]any{
		"query":            map[string]any{"bool": boolQ},
		"sort":             []any{map[string]any{"@timestamp": map[string]any{"order": "desc", "unmapped_type": "date"}}},
		"from":             (q.Page - 1) * q.Size,
		"size":             q.Size,
		"track_total_hits": true,
	})
	var res struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				ID     string `json:"_id"`
				Source Event  `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	path := "/" + url.PathEscape(l.cfg.IndexPrefix+"-"+q.Kind+"-*") + "/_search?ignore_unavailable=true&allow_no_indices=true"
	if err := l.do(ctx, http.MethodPost, path, "application/json", body, &res); err != nil {
		return nil, err
	}
	out := &Result{Total: res.Hits.Total.Value, Items: make([]Event, 0, len(res.Hits.Hits))}
	for _, h := range res.Hits.Hits {
		h.Source["_id"] = h.ID
		out.Items = append(out.Items, h.Source)
	}
	return out, nil
}

func writeStdout(kind string, e Event) {
	b, _ := json.Marshal(e)
	log.Printf("[log:%s] %s", kind, b)
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

// Truncate memotong teks panjang (prompt, error) sebelum masuk log.
func Truncate(s string, n int) string { return truncate(s, n) }
