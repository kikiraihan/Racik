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

// Katalog model OpenRouter (GET {base}/models) untuk Panel admin → Konfigurasi AI, seperti
// admin → Model AI di tiara: admin mencari model dari katalog lalu menambahkannya ke daftar
// pilihan tanpa mengubah env / restart. Model tambahan disimpan di app_settings
// (key "openrouter_models") dan dipasang ke router lewat Router.SetExtraModels.

const catalogTTL = 10 * time.Minute

// Jenis model di katalog.
const (
	KindText  = "text"
	KindImage = "image"
)

// CatalogModel: satu model dari katalog OpenRouter.
type CatalogModel struct {
	ID              string   `json:"id"`
	Kind            string   `json:"kind"` // text | image
	Name            string   `json:"name"`
	Description     string   `json:"description,omitempty"`
	ContextLength   *int     `json:"contextLength"`
	PromptPrice     *float64 `json:"promptPrice"`     // USD / 1 juta token input
	CompletionPrice *float64 `json:"completionPrice"` // USD / 1 juta token output
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

// Catalog: daftar model OpenRouter dengan cache 10 menit.
type Catalog struct {
	client *Client

	mu   sync.Mutex
	list []CatalogModel
	at   time.Time
}

func NewCatalog(c *Client) *Catalog { return &Catalog{client: c} }

// Models: katalog untuk jenis kind ("" = semua). force = ambil ulang dari OpenRouter.
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

// Find: cari satu model di katalog (pakai cache); nil jika tidak ada.
func (c *Catalog) Find(ctx context.Context, id string) (*CatalogModel, error) {
	list, err := c.load(ctx, false)
	if err != nil {
		return nil, err
	}
	i := sort.Search(len(list), func(i int) bool { return list[i].ID >= id })
	if i < len(list) && list[i].ID == id {
		m := list[i]
		return &m, nil
	}
	return nil, nil
}

func (c *Catalog) load(ctx context.Context, force bool) ([]CatalogModel, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !force && c.list != nil && time.Since(c.at) < catalogTTL {
		return c.list, nil
	}
	list, err := c.client.fetchCatalog(ctx)
	if err != nil {
		return nil, err
	}
	c.list, c.at = list, time.Now()
	return list, nil
}

func (c *Client) fetchCatalog(ctx context.Context) ([]CatalogModel, error) {
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
	var list orCatalog
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("respons GET %s/models tidak valid: %w", c.BaseURL, err)
	}

	out := make([]CatalogModel, 0, len(list.Data))
	for _, m := range list.Data {
		kind := catalogKind(m.Architecture.OutputModalities)
		if m.ID == "" || kind == "" {
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
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
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
