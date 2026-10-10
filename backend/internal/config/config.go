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
