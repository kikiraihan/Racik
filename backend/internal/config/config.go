// Package config memuat konfigurasi aplikasi dari environment variable.
package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Addr        string
	DatabaseURL string
	JWTSecret   string
	CORSOrigins []string
	UploadDir   string
	PublicURL   string // URL publik backend untuk URL file upload; kosong = relatif (/uploads/..) lewat proxy Nuxt
	MaxUploadMB int64

	// Konektor AI lewat gateway kenari.id (OpenAI-compatible).
	KenariBaseURL     string
	KenariAPIKey      string
	KenariTextModel   string   // model LLM untuk menyusun layout (mis. Claude)
	KenariTextModels  []string // daftar model teks yang boleh dipilih user
	KenariImageModel  string   // model gambar default (Nano Banana / GPT Image)
	KenariImageModels []string // daftar model gambar yang boleh dipilih user
	AITimeoutSec      int

	// Provider AI lain (sama dengan katilidev). Provider aktif kalau API key-nya diisi
	// (Pollinations: POLLINATIONS_ENABLED=true); modelnya masuk ke pilihan di UI sebagai
	// "<provider>:<model>".
	Anthropic    Provider // teks (Claude API langsung)
	Groq         Provider // teks
	Nvidia       Provider // teks + gambar (NVIDIA NIM)
	Agnes        Provider // teks + gambar
	OpenRouter   Provider // teks + gambar (gambar lewat chat completions)
	HuggingFace  Provider // gambar
	Pollinations Provider // gambar, gratis tanpa key

	AnthropicEffort  string // effort Claude: low | medium | high | xhigh | max
	NvidiaImageSteps int    // langkah FLUX.2 di NVIDIA

	// Model default lintas provider, format "<provider>:<model>" (mis. "kenari:claude-sonnet-5-5").
	// Kosong = model pertama provider aktif, urutan: kenari, anthropic, groq, nvidia, agnes,
	// openrouter (teks) / kenari, nvidia, agnes, openrouter, huggingface, pollinations (gambar).
	AITextModel  string
	AIImageModel string

	// Akses admin: email di ADMIN_EMAILS otomatis ber-role admin (saat start, daftar, login).
	// ADMIN_PASSWORD (opsional) membuat akun admin pertama kalau email pertama belum terdaftar.
	AdminEmails   []string
	AdminPassword string

	// Sign in / sign up dengan Google (Google Identity Services). Isi dengan OAuth Client ID
	// tipe "Web application" dari Google Cloud Console; kosong = tombol Google disembunyikan.
	// Boleh lebih dari satu (pisahkan koma), mis. client ID terpisah untuk dev & production.
	GoogleClientIDs []string

	// Tarif default, hanya dipakai untuk mengisi menu Konfigurasi Harga pertama kali.
	// Setelah itu harga diatur admin dari UI (tersimpan di tabel app_settings / model_prices).
	DefaultMarkupPercent float64 // harga user = harga vendor + markup %
	DefaultFreeHitPrice  int64   // harga per hit (Rp) untuk model yang gratis di vendor

	// System log (activity user + request ke vendor AI) ke Elasticsearch.
	// LOG_ES_URL kosong = log ditulis ke stdout sebagai JSON per baris.
	LogESURL         string
	LogESUser        string
	LogESPassword    string
	LogESAPIKey      string
	LogIndexPrefix   string // index: <prefix>-activity-YYYY.MM & <prefix>-vendor-YYYY.MM
	LogRetentionDays int    // ILM: index dihapus setelah N hari (0 = simpan selamanya)
}

// Provider: konfigurasi satu provider AI selain kenari.
type Provider struct {
	APIKey      string
	BaseURL     string
	Enabled     bool // hanya untuk provider tanpa key (Pollinations)
	TextModels  []string
	ImageModels []string
}

func Load() Config {
	// Baca file .env (jika ada) tanpa menimpa env yang sudah di-set.
	loadDotEnv(".env")
	loadDotEnv("../.env")
	c := Config{
		Addr:          env("APP_ADDR", ":8080"),
		DatabaseURL:   env("DATABASE_URL", "postgres://racik:racik@localhost:5432/racik?sslmode=disable"),
		JWTSecret:     env("JWT_SECRET", "ganti-secret-ini-di-production"),
		CORSOrigins:   list(env("CORS_ORIGINS", "http://localhost:3000")),
		UploadDir:     env("UPLOAD_DIR", "./uploads"),
		PublicURL:     strings.TrimRight(env("PUBLIC_URL", ""), "/"),
		MaxUploadMB:   int64(envInt("MAX_UPLOAD_MB", 20)),
		KenariBaseURL: strings.TrimRight(env("KENARI_BASE_URL", "https://kenari.id/v1"), "/"),
		KenariAPIKey:  env("KENARI_API_KEY", ""),
		// Docs kenari menyarankan timeout >= 5 menit untuk generate gambar.
		AITimeoutSec: envInt("AI_TIMEOUT_SEC", 300),
	}
	// Id model harus persis sama dengan daftar GET https://kenari.id/v1/models
	// (publik). Default di bawah diambil dari katalog yang dipakai katilidev (Okt 2026),
	// kecuali claude-sonnet-5-5: id-nya ditebak dari pola kenari (claude-haiku-5-5),
	// belum dicek ke /models — kalau salah, log start menulis PERINGATAN.
	c.KenariTextModel = env("KENARI_TEXT_MODEL", "claude-sonnet-5-5")
	c.KenariTextModels = withDefault(c.KenariTextModel, list(env("KENARI_TEXT_MODELS", c.KenariTextModel)))
	c.KenariImageModel = env("KENARI_IMAGE_MODEL", "gpt-image-2")
	c.KenariImageModels = withDefault(c.KenariImageModel, list(env("KENARI_IMAGE_MODELS", c.KenariImageModel+",nano-banana-2")))

	// Default model provider lain mengikuti katilidev (config/services.php + app/Ai/AiModels.php).
	c.Anthropic = Provider{
		APIKey:     env("ANTHROPIC_API_KEY", ""),
		BaseURL:    env("ANTHROPIC_BASE_URL", ""),
		TextModels: list(env("ANTHROPIC_TEXT_MODELS", "claude-sonnet-5-5,claude-opus-5-5,claude-haiku-5-5")),
	}
	c.AnthropicEffort = env("ANTHROPIC_EFFORT", "medium")
	c.Groq = Provider{
		APIKey:     env("GROQ_API_KEY", ""),
		TextModels: list(env("GROQ_TEXT_MODELS", "openai/gpt-oss-120b,openai/gpt-oss-20b")),
	}
	c.Nvidia = Provider{
		APIKey:      env("NVIDIA_API_KEY", ""),
		TextModels:  list(env("NVIDIA_TEXT_MODELS", "nvidia/nemotron-3-super-120b-a12b,openai/gpt-oss-20b,nvidia/nemotron-3.5-lightning-30b-a3b")),
		ImageModels: list(env("NVIDIA_IMAGE_MODELS", "black-forest-labs/flux.2-klein-4b,black-forest-labs/flux.1-dev")),
	}
	c.NvidiaImageSteps = envInt("NVIDIA_IMAGE_STEPS", 4)
	c.Agnes = Provider{
		APIKey:      env("AGNES_API_KEY", ""),
		BaseURL:     strings.TrimRight(env("AGNES_BASE_URL", "https://apihub.agnes-ai.com/v1"), "/"),
		TextModels:  list(env("AGNES_TEXT_MODELS", "agnes-3.0-flash,agnes-2.5-flash")),
		ImageModels: list(env("AGNES_IMAGE_MODELS", "agnes-image-2.5-flash,agnes-image-2.1-flash")),
	}
	c.OpenRouter = Provider{
		APIKey:      env("OPENROUTER_API_KEY", ""),
		BaseURL:     strings.TrimRight(env("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1"), "/"),
		TextModels:  list(env("OPENROUTER_TEXT_MODELS", "")),
		ImageModels: list(env("OPENROUTER_IMAGE_MODELS", "black-forest-labs/flux.2-flex")),
	}
	c.HuggingFace = Provider{
		APIKey:      env("HUGGINGFACE_API_KEY", ""),
		BaseURL:     strings.TrimRight(env("HUGGINGFACE_BASE_URL", "https://router.huggingface.co/hf-inference/models"), "/"),
		ImageModels: list(env("HUGGINGFACE_IMAGE_MODELS", "black-forest-labs/FLUX.1-schnell")),
	}
	c.Pollinations = Provider{
		Enabled:     env("POLLINATIONS_ENABLED", "false") == "true",
		ImageModels: list(env("POLLINATIONS_IMAGE_MODELS", "flux")),
	}
	c.AITextModel = env("AI_TEXT_MODEL", "")
	c.AIImageModel = env("AI_IMAGE_MODEL", "")

	c.AdminEmails = list(strings.ToLower(env("ADMIN_EMAILS", "")))
	c.AdminPassword = env("ADMIN_PASSWORD", "")
	c.GoogleClientIDs = list(env("GOOGLE_CLIENT_ID", ""))
	c.DefaultMarkupPercent = envFloat("PRICE_MARKUP_PERCENT", 5)
	c.DefaultFreeHitPrice = int64(envInt("PRICE_FREE_HIT", 100))

	c.LogESURL = strings.TrimRight(env("LOG_ES_URL", ""), "/")
	c.LogESUser = env("LOG_ES_USER", "")
	c.LogESPassword = env("LOG_ES_PASSWORD", "")
	c.LogESAPIKey = env("LOG_ES_API_KEY", "")
	c.LogIndexPrefix = env("LOG_INDEX_PREFIX", "racik")
	c.LogRetentionDays = envInt("LOG_RETENTION_DAYS", 90)
	return c
}

// withDefault memastikan model default selalu ada (paling depan) di daftar pilihan,
// supaya dropdown di UI tidak kehilangan model yang sebenarnya dipakai.
func withDefault(def string, models []string) []string {
	for _, m := range models {
		if m == def {
			return models
		}
	}
	return append([]string{def}, models...)
}

func env(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(k)); err == nil {
		return v
	}
	return def
}

func envFloat(k string, def float64) float64 {
	if v, err := strconv.ParseFloat(os.Getenv(k), 64); err == nil {
		return v
	}
	return def
}

func list(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func loadDotEnv(path string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		if _, exists := os.LookupEnv(k); !exists {
			os.Setenv(k, v)
		}
	}
}
