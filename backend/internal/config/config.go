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
		AITimeoutSec:  envInt("AI_TIMEOUT_SEC", 180),
	}
	c.KenariTextModel = env("KENARI_TEXT_MODEL", "claude-sonnet-4-5")
	c.KenariTextModels = list(env("KENARI_TEXT_MODELS", c.KenariTextModel))
	c.KenariImageModel = env("KENARI_IMAGE_MODEL", "gemini-2.5-flash-image")
	c.KenariImageModels = list(env("KENARI_IMAGE_MODELS", c.KenariImageModel+",gpt-image-1"))
	return c
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
