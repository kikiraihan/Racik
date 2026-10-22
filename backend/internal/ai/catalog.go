package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Katalog model dari API provider (GET {base}/models) untuk Panel admin → Katalog model, seperti
// admin → Model AI di tiara/katilidev: admin mencari model dari daftar provider lalu menambahkannya
// ke katalog (tabel ai_models) tanpa mengubah env / restart. Konfigurasi AI hanya menawarkan model
// katalog yang aktif (dipasang ke router lewat Router.SetCatalog). Katalog tersedia untuk
// OpenRouter dan kenari.

const catalogTTL = 10 * time.Minute

// Jenis model di katalog.
const (
	KindText  = "text"
	KindImage = "image"
)

// CatalogModel: satu model dari daftar API provider.
type CatalogModel struct {
	ID              string   `json:"id"`
	Kind            string   `json:"kind"` // text | image
	Name            string   `json:"name"`
	Maker           string   `json:"maker,omitempty"` // pembuat model (owned_by / awalan id OpenRouter)
	Description     string   `json:"description,omitempty"`
	ContextLength   *int     `json:"contextLength"`
	PromptPrice     *float64 `json:"promptPrice"`     // USD / 1 juta token input (OpenRouter)
	CompletionPrice *float64 `json:"completionPrice"` // USD / 1 juta token output (OpenRouter)
	ImagePrice      *float64 `json:"imagePrice"`      // USD / gambar (kalau dicantumkan OpenRouter)
	InputModalities []string `json:"inputModalities"`
	IsFree          bool     `json:"isFree"`
}

type orCatalog struct {
	Data []struct {
		ID            string `json:"id"`
		Name          string `json:"name"`
		Description   string `json:"description"`
		ContextLength *int   `json:"context_length"`
		Pricing       struct {
			Prompt     string `json:"prompt"`
			Completion string `json:"completion"`
			Image      string `json:"image"`
		} `json:"pricing"`
		Architecture struct {
			InputModalities  []string `json:"input_modalities"`
			OutputModalities []string `json:"output_modalities"`
		} `json:"architecture"`
	} `json:"data"`
}

// Catalog: daftar model satu provider dengan cache 10 menit.
type Catalog struct {
	Provider string
	fetch    func(ctx context.Context) ([]CatalogModel, error)

	mu   sync.Mutex
	list []CatalogModel
	at   time.Time
}

// NewCatalog: katalog OpenRouter (GET /models dengan architecture & pricing).
func NewCatalog(c *Client) *Catalog {
	return &Catalog{Provider: "openrouter", fetch: c.fetchCatalog}
}

// NewKenariCatalog: katalog kenari (GET /models publik, dipilah per kolom "endpoints").
func NewKenariCatalog(c *Client) *Catalog {
	return &Catalog{Provider: "kenari", fetch: c.fetchKenariCatalog}
}

// Models: katalog untuk jenis kind ("" = semua). force = ambil ulang dari provider.
// Daftar kosong dianggap error, bukan "semua model sudah dihapus".
func (c *Catalog) Models(ctx context.Context, kind string, force bool) ([]CatalogModel, error) {
	all, err := c.load(ctx, force)
	if err != nil {
		return nil, err
	}
	if kind == "" {
		return all, nil
	}
	out := make([]CatalogModel, 0, len(all))
	for _, m := range all {
		if m.Kind == kind {
			out = append(out, m)
		}
	}
	return out, nil
}

// Find: cari satu model berjenis kind di katalog (pakai cache); nil jika tidak ada.
func (c *Catalog) Find(ctx context.Context, kind, id string) (*CatalogModel, error) {
	list, err := c.load(ctx, false)
	if err != nil {
		return nil, err
	}
	for _, m := range list {
		if m.ID == id && m.Kind == kind {
			return &m, nil
		}
	}
	return nil, nil
}

func (c *Catalog) load(ctx context.Context, force bool) ([]CatalogModel, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !force && c.list != nil && time.Since(c.at) < catalogTTL {
		return c.list, nil
	}
	list, err := c.fetch(ctx)
	if err != nil {
		return nil, err
	}
	c.list, c.at = list, time.Now()
	return list, nil
}

// getModels: GET {base}/models (dengan API key kalau ada).
func (c *Client) getModels(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	if c.APIKey != "" {
		c.setHeaders(req)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gagal menghubungi %s: %w", c.Name, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, responseError(c.Name, "/models", resp, raw)
	}
	return raw, nil
}

// ChatUnsupported: varian model yang tidak bisa dipanggil lewat /chat/completions maupun
// /images/generations, mis. "anthropic/claude-sonnet-5.5:batch" di OpenRouter (hanya Batch
// API, ditolak 404 "cannot be used with the chat/completions endpoint").
func ChatUnsupported(id string) bool {
	return strings.HasSuffix(id, ":batch")
}

func (c *Client) fetchCatalog(ctx context.Context) ([]CatalogModel, error) {
	raw, err := c.getModels(ctx)
	if err != nil {
		return nil, err
	}
	var list orCatalog
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("respons GET %s/models tidak valid: %w", c.BaseURL, err)
	}

	out := make([]CatalogModel, 0, len(list.Data))
	for _, m := range list.Data {
		kind := catalogKind(m.Architecture.OutputModalities)
		if m.ID == "" || kind == "" || ChatUnsupported(m.ID) {
			continue
		}
		cm := CatalogModel{
			ID:              m.ID,
			Kind:            kind,
			Name:            strings.TrimSpace(m.Name),
			Description:     truncateRunes(strings.TrimSpace(m.Description), 300),
			ContextLength:   m.ContextLength,
			PromptPrice:     perMillion(m.Pricing.Prompt),
			CompletionPrice: perMillion(m.Pricing.Completion),
			ImagePrice:      parsePrice(m.Pricing.Image),
			InputModalities: m.Architecture.InputModalities,
		}
		cm.Maker, _, _ = strings.Cut(m.ID, "/")
		if cm.Name == "" {
			cm.Name = m.ID
		}
		cm.IsFree = strings.HasSuffix(m.ID, ":free") ||
			(cm.PromptPrice != nil && *cm.PromptPrice == 0 && cm.CompletionPrice != nil && *cm.CompletionPrice == 0)
		out = append(out, cm)
	}
	if len(out) == 0 {
		return nil, errors.New("daftar model OpenRouter kosong")
	}
	sortCatalog(out)
	return out, nil
}

// fetchKenariCatalog: GET /models kenari memuat model teks, gambar, suara, video sekaligus;
// dipilah per kolom "endpoints" ("chat" = teks, "images" = gambar). Entri tanpa endpoints
// (mis. kenari/auto) dianggap model teks. Harga kenari (Rupiah) tidak dibaca dari daftar ini.
func (c *Client) fetchKenariCatalog(ctx context.Context) ([]CatalogModel, error) {
	raw, err := c.getModels(ctx)
	if err != nil {
		return nil, err
	}
	var list struct {
		Data []struct {
			ID            string   `json:"id"`
			Name          string   `json:"name"`
			OwnedBy       string   `json:"owned_by"`
			Description   string   `json:"description"`
			ContextLength *int     `json:"context_length"`
			Endpoints     []string `json:"endpoints"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("respons GET %s/models tidak valid: %w", c.BaseURL, err)
	}
	out := make([]CatalogModel, 0, len(list.Data))
	for _, m := range list.Data {
		if m.ID == "" || ChatUnsupported(m.ID) {
			continue
		}
		var kinds []string
		if m.Endpoints == nil || slices.Contains(m.Endpoints, "chat") {
			kinds = append(kinds, KindText)
		}
		if slices.Contains(m.Endpoints, "images") {
			kinds = append(kinds, KindImage)
		}
		name := strings.TrimSpace(m.Name)
		if name == "" {
			name = m.ID
		}
		for _, kind := range kinds {
			cm := CatalogModel{
				ID:            m.ID,
				Kind:          kind,
				Name:          name,
				Maker:         strings.TrimSpace(m.OwnedBy),
				Description:   truncateRunes(strings.TrimSpace(m.Description), 300),
				ContextLength: m.ContextLength,
				IsFree:        strings.HasSuffix(m.ID, ":free"),
			}
			if cm.Maker == "" {
				cm.Maker, _, _ = strings.Cut(m.ID, "/")
			}
			out = append(out, cm)
		}
	}
	if len(out) == 0 {
		return nil, errors.New("daftar model kenari kosong")
	}
	sortCatalog(out)
	return out, nil
}

func sortCatalog(list []CatalogModel) {
	sort.Slice(list, func(i, j int) bool {
		if list[i].ID != list[j].ID {
			return list[i].ID < list[j].ID
		}
		return list[i].Kind < list[j].Kind
	})
}

// catalogKind: model yang bisa mengeluarkan gambar = model gambar (dipanggil lewat chat
// completions + modalities); keluaran teks saja = model teks. Data tanpa architecture
// dianggap teks. Selain itu (audio, embedding, ...) dilewati.
func catalogKind(out []string) string {
	switch {
	case len(out) == 0:
		return KindText
	case slices.Contains(out, "image"):
		return KindImage
	case slices.Contains(out, "text"):
		return KindText
	}
	return ""
}

func parsePrice(v string) *float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil || f < 0 {
		return nil
	}
	return &f
}

// perMillion: harga OpenRouter per token (string) → USD per 1 juta token.
func perMillion(v string) *float64 {
	f := parsePrice(v)
	if f != nil {
		*f *= 1e6
	}
	return f
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
