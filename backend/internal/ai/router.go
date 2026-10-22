package ai

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/kikiraihan/racik/backend/internal/config"
)

type TextGenerator interface {
	Chat(ctx context.Context, model string, msgs []Message, maxTokens int) (string, error)
}

type ImageGenerator interface {
	GenerateImage(ctx context.Context, model, prompt string, width, height int) (*Image, error)
}

type ImageEditor interface {
	EditImage(ctx context.Context, model, prompt string, src *Image) (*Image, error)
}

type modelLister interface {
	ListModels(ctx context.Context, endpoint string) ([]string, error)
}

// Provider adalah satu provider AI beserta model yang boleh dipilih user.
type Provider struct {
	Name        string
	Label       string
	Text        TextGenerator  // nil = tidak melayani teks
	Image       ImageGenerator // nil = tidak melayani gambar
	Edit        ImageEditor    // nil = tidak bisa edit gambar
	TextModels  []string
	ImageModels []string

	// Daftar model dari API provider untuk dicek saat start (opsional).
	Lister                 modelLister
	CheckText, CheckImages bool

	// Inactive: provider terdaftar tapi API key-nya kosong (hanya kenari, untuk pesan error);
	// modelnya tidak ditawarkan.
	Inactive bool

	// Model aktif dari Panel admin → Katalog model (tabel ai_models). Selama katalog belum
	// dipasang (fromCatalog false) yang ditawarkan adalah model env. Diubah lewat
	// Router.SetCatalog, dibaca di bawah Router.mu.
	fromCatalog       bool
	catText, catImage []string
}

// textModels / imageModels: model katalog aktif (atau model env sebelum katalog dipasang).
// Panggil di bawah Router.mu.
func (p *Provider) textModels() []string {
	switch {
	case p.Inactive:
		return nil
	case p.fromCatalog:
		return p.catText
	}
	return p.TextModels
}

func (p *Provider) imageModels() []string {
	switch {
	case p.Inactive:
		return nil
	case p.fromCatalog:
		return p.catImage
	}
	return p.ImageModels
}

// CatalogModels: model aktif satu provider di katalog.
type CatalogModels struct {
	Text, Image []string
}

// Router memilih provider berdasarkan id pilihan "<provider>:<model>". Hanya awalan
// sampai ":" pertama yang dibuang — id kenari sendiri bisa memuat ":" (step-5-preview:free).
type Router struct {
	mu        sync.RWMutex // menjaga model tambahan admin (SetExtraModels)
	providers map[string]*Provider
	order     []string
	textDef   string
	imageDef  string

	// Observe dipanggil setelah setiap request ke vendor AI (untuk log vendor). Opsional.
	Observe func(ctx context.Context, c Call)

	// Recorder menerima setiap request HTTP ke vendor AI untuk AI API Call Log (calllog.go).
	// Opsional; LogBodies = simpan ringkasan body request & response.
	Recorder  CallRecorder
	LogBodies bool

	// Catalogs: daftar model dari API provider untuk Panel admin → Katalog model, per nama
	// provider (openrouter, kenari). Provider yang tidak aktif tidak punya entri.
	Catalogs map[string]*Catalog
}

// Call: ringkasan satu request ke vendor AI.
type Call struct {
	Choice    string // "<provider>:<model>"
	Provider  string
	Model     string
	Operation string // chat | image | edit
	Prompt    string // prompt terakhir dari user (dipotong oleh pencatat log)
	Duration  time.Duration
	Err       error
	OutBytes  int // panjang jawaban teks / ukuran gambar
}

func (r *Router) observe(ctx context.Context, c Call, start time.Time) {
	if r.Observe == nil {
		return
	}
	c.Provider, c.Model = SplitChoice(c.Choice)
	c.Duration = time.Since(start)
	r.Observe(ctx, c)
}

// lastUserText: isi teks pesan user terakhir, untuk dicatat di log vendor.
func lastUserText(msgs []Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role != "user" {
			continue
		}
		switch c := msgs[i].Content.(type) {
		case string:
			return c
		case []ContentPart:
			for _, p := range c {
				if p.Text != "" {
					return p.Text
				}
			}
		}
	}
	return ""
}

func NewRouter() *Router {
	return &Router{providers: map[string]*Provider{}, Catalogs: map[string]*Catalog{}}
}

func (r *Router) Add(p *Provider) {
	if _, ok := r.providers[p.Name]; !ok {
		r.order = append(r.order, p.Name)
	}
	r.providers[p.Name] = p
}

// SplitChoice memecah "<provider>:<model>".
func SplitChoice(choice string) (provider, model string) {
	provider, model, _ = strings.Cut(choice, ":")
	return provider, model
}

func Choice(provider, model string) string { return provider + ":" + model }

// Configured: ada minimal satu provider teks aktif.
func (r *Router) Configured() bool { return len(r.TextChoices()) > 0 }

func (r *Router) choices(pick func(*Provider) []string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []string
	for _, name := range r.order {
		for _, m := range pick(r.providers[name]) {
			out = append(out, Choice(name, m))
		}
	}
	return out
}

func (r *Router) TextChoices() []string {
	return r.choices(func(p *Provider) []string {
		if p.Text == nil {
			return nil
		}
		return p.textModels()
	})
}

func (r *Router) ImageChoices() []string {
	return r.choices(func(p *Provider) []string {
		if p.Image == nil {
			return nil
		}
		return p.imageModels()
	})
}

// EditChoices: model gambar yang providernya bisa edit gambar.
func (r *Router) EditChoices() []string {
	return r.choices(func(p *Provider) []string {
		if p.Edit == nil {
			return nil
		}
		return p.imageModels()
	})
}

// Labels: label tampilan per pilihan, "model · Provider".
func (r *Router) Labels() map[string]string {
	out := map[string]string{}
	for _, c := range append(r.TextChoices(), r.ImageChoices()...) {
		name, model := SplitChoice(c)
		out[c] = model + " · " + r.providers[name].Label
	}
	return out
}

// Has: provider name aktif (API key diisi).
func (r *Router) Has(name string) bool { return r.providers[name] != nil }

// SetCatalog memasang model aktif dari Panel admin → Katalog model, per nama provider.
// Provider yang tidak ada di models tidak menawarkan model apa pun; models nil = kembali
// ke model env. Aman dipanggil saat server berjalan.
func (r *Router) SetCatalog(models map[string]CatalogModels) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for name, p := range r.providers {
		m := models[name]
		p.fromCatalog = models != nil
		p.catText, p.catImage = slices.Clone(m.Text), slices.Clone(m.Image)
	}
}

// ProviderInfo: ringkasan provider untuk Panel admin (tanpa API key).
type ProviderInfo struct {
	Name     string   `json:"name"`
	Label    string   `json:"label"`
	Active   bool     `json:"active"` // API key diisi
	Text     bool     `json:"text"`   // melayani model teks
	Image    bool     `json:"image"`  // melayani model gambar
	Edit     bool     `json:"edit"`   // bisa edit gambar
	Catalog  bool     `json:"catalog"`
	EnvText  []string `json:"envText"`  // model teks dari env (bawaan)
	EnvImage []string `json:"envImage"` // model gambar dari env (bawaan)
}

// Providers: provider terdaftar sesuai urutan dropdown.
func (r *Router) Providers() []ProviderInfo {
	out := make([]ProviderInfo, 0, len(r.order))
	for _, name := range r.order {
		p := r.providers[name]
		info := ProviderInfo{Name: name, Label: p.Label, Active: !p.Inactive, Text: p.Text != nil, Image: p.Image != nil,
			Edit: p.Edit != nil, Catalog: r.Catalogs[name] != nil, EnvText: []string{}, EnvImage: []string{}}
		if info.Text {
			info.EnvText = slices.Clone(p.TextModels)
		}
		if info.Image {
			info.EnvImage = slices.Clone(p.ImageModels)
		}
		out = append(out, info)
	}
	return out
}

// SetDefaults mengatur model default; nilai kosong / tidak aktif = pilihan pertama.
func (r *Router) SetDefaults(text, image string) {
	r.textDef, r.imageDef = text, image
}

func (r *Router) DefaultText() string  { return pickDefault(r.textDef, r.TextChoices()) }
func (r *Router) DefaultImage() string { return pickDefault(r.imageDef, r.ImageChoices()) }

func pickDefault(want string, choices []string) string {
	if want != "" && slices.Contains(choices, want) {
		return want
	}
	if len(choices) > 0 {
		return choices[0]
	}
	return want
}

func (r *Router) provider(choice string) (*Provider, string, error) {
	name, model := SplitChoice(choice)
	p := r.providers[name]
	if p == nil || model == "" {
		if choice == "" {
			return nil, "", fmt.Errorf("belum ada model AI aktif: %w", ErrNotConfigured)
		}
		return nil, "", fmt.Errorf("provider AI %q belum dikonfigurasi: %w", name, ErrNotConfigured)
	}
	return p, model, nil
}

func (r *Router) Chat(ctx context.Context, choice string, msgs []Message, maxTokens int) (text string, err error) {
	defer func(start time.Time) {
		r.observe(ctx, Call{Choice: choice, Operation: "chat", Prompt: lastUserText(msgs), Err: err, OutBytes: len(text)}, start)
	}(time.Now())
	ctx = r.withCallScope(ctx, choice, "chat")
	p, model, err := r.provider(choice)
	if err != nil {
		return "", err
	}
	if p.Text == nil {
		return "", fmt.Errorf("%s tidak melayani model teks", p.Label)
	}
	return p.Text.Chat(ctx, model, msgs, maxTokens)
}

func (r *Router) GenerateImage(ctx context.Context, choice, prompt string, width, height int) (img *Image, err error) {
	defer func(start time.Time) {
		r.observe(ctx, Call{Choice: choice, Operation: "image", Prompt: prompt, Err: err, OutBytes: imageSize(img)}, start)
	}(time.Now())
	ctx = r.withCallScope(ctx, choice, "image")
	p, model, err := r.provider(choice)
	if err != nil {
		return nil, err
	}
	if p.Image == nil {
		return nil, fmt.Errorf("%s tidak melayani model gambar", p.Label)
	}
	return p.Image.GenerateImage(ctx, model, prompt, width, height)
}

func (r *Router) EditImage(ctx context.Context, choice, prompt string, src *Image) (img *Image, err error) {
	defer func(start time.Time) {
		r.observe(ctx, Call{Choice: choice, Operation: "edit", Prompt: prompt, Err: err, OutBytes: imageSize(img)}, start)
	}(time.Now())
	ctx = r.withCallScope(ctx, choice, "edit")
	p, model, err := r.provider(choice)
	if err != nil {
		return nil, err
	}
	if p.Edit == nil {
		return nil, fmt.Errorf("%s tidak bisa edit gambar, pilih model gambar kenari atau OpenRouter", p.Label)
	}
	return p.Edit.EditImage(ctx, model, prompt, src)
}

func imageSize(img *Image) int {
	if img == nil {
		return 0
	}
	return len(img.Data)
}

// CheckModels mencocokkan model di env dengan daftar /models provider yang menyediakannya
// dan mengembalikan peringatan (tidak pernah menggagalkan start).
func (r *Router) CheckModels(ctx context.Context) []string {
	var warnings []string
	for _, name := range r.order {
		p := r.providers[name]
		if p.Lister == nil || p.Inactive {
			continue
		}
		groups := []struct {
			on       bool
			endpoint string
			models   []string
		}{{p.CheckText && p.Text != nil, "chat", p.TextModels}, {p.CheckImages && p.Image != nil, "images", p.ImageModels}}
		for _, g := range groups {
			if !g.on || len(g.models) == 0 {
				continue
			}
			ids, err := p.Lister.ListModels(ctx, g.endpoint)
			if err != nil || len(ids) == 0 {
				warnings = append(warnings, fmt.Sprintf("%s: daftar model (%s) tidak bisa dibaca, cek model dilewati: %v", name, g.endpoint, err))
				continue
			}
			for _, m := range g.models {
				if !slices.Contains(ids, m) {
					warnings = append(warnings, fmt.Sprintf("%s: model %q tidak ada di daftar /models (%s) — cek env model %s", name, m, g.endpoint, strings.ToUpper(name)))
				}
			}
		}
	}
	return warnings
}

// FromConfig menyusun router dari env. Hanya provider yang API key-nya diisi yang
// didaftarkan, kecuali kenari: selalu terdaftar supaya tanpa key apa pun user tetap
// mendapat pesan "KENARI_API_KEY belum diisi".
func FromConfig(cfg config.Config) *Router {
	timeout := time.Duration(cfg.AITimeoutSec) * time.Second
	r := NewRouter()

	kenari := New(cfg.KenariBaseURL, cfg.KenariAPIKey, timeout)
	r.Add(&Provider{Name: "kenari", Label: "kenari", Text: kenari, Image: kenari, Edit: kenari,
		TextModels: cfg.KenariTextModels, ImageModels: cfg.KenariImageModels,
		Lister: kenari, CheckText: true, CheckImages: true})
	if !kenari.Configured() {
		// Tetap terdaftar untuk pesan error, tapi modelnya tidak ditawarkan.
		r.providers["kenari"].Inactive = true
	} else {
		// Katalog /models kenari untuk menambah model dari Panel admin → Katalog model.
		r.Catalogs["kenari"] = NewKenariCatalog(kenari)
	}

	if p := cfg.Anthropic; p.APIKey != "" {
		r.Add(&Provider{Name: "anthropic", Label: "Anthropic", Text: NewAnthropic(p.APIKey, p.BaseURL, cfg.AnthropicEffort, timeout), TextModels: p.TextModels})
	}
	if p := cfg.Groq; p.APIKey != "" {
		g := NewGroq(p.APIKey, timeout)
		r.Add(&Provider{Name: "groq", Label: "Groq", Text: g, TextModels: p.TextModels, Lister: g, CheckText: true})
	}
	if p := cfg.Nvidia; p.APIKey != "" {
		t := NewNvidiaText(p.APIKey, timeout)
		// /models NVIDIA hanya memuat model teks (gambar ada di endpoint GenAI).
		r.Add(&Provider{Name: "nvidia", Label: "NVIDIA", Text: t, Image: NewNvidiaImage(p.APIKey, cfg.NvidiaImageSteps, timeout),
			TextModels: p.TextModels, ImageModels: p.ImageModels, Lister: t, CheckText: true})
	}
	if p := cfg.Agnes; p.APIKey != "" {
		a := NewAgnes(p.BaseURL, p.APIKey, timeout)
		r.Add(&Provider{Name: "agnes", Label: "Agnes AI", Text: a, Image: a, TextModels: p.TextModels, ImageModels: p.ImageModels})
	}
	if p := cfg.OpenRouter; p.APIKey != "" {
		o := NewOpenRouter(p.BaseURL, p.APIKey, cfg.PublicURL, "Racik", timeout)
		r.Add(&Provider{Name: "openrouter", Label: "OpenRouter", Text: o, Image: o, Edit: o,
			TextModels: p.TextModels, ImageModels: p.ImageModels, Lister: o, CheckText: true, CheckImages: true})
		// Katalog /models untuk menambah model dari Panel admin → Katalog model.
		r.Catalogs["openrouter"] = NewCatalog(o)
	}
	if p := cfg.HuggingFace; p.APIKey != "" {
		r.Add(&Provider{Name: "huggingface", Label: "Hugging Face", Image: NewHuggingFace(p.BaseURL, p.APIKey, timeout), ImageModels: p.ImageModels})
	}
	if p := cfg.Pollinations; p.Enabled {
		r.Add(&Provider{Name: "pollinations", Label: "Pollinations", Image: NewPollinations(true, timeout), ImageModels: p.ImageModels})
	}

	// Tanpa AI_*_MODEL, default = model kenari; kalau kenari tidak aktif, pickDefault
	// memilih model pertama provider lain (atau tetap kenari → pesan "KENARI_API_KEY
	// belum diisi" bila tidak ada provider sama sekali).
	text, image := cfg.AITextModel, cfg.AIImageModel
	if text == "" {
		text = Choice("kenari", cfg.KenariTextModel)
	}
	if image == "" {
		image = Choice("kenari", cfg.KenariImageModel)
	}
	r.SetDefaults(text, image)
	return r
}
