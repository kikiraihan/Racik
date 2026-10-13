package ai

import "time"

// Model cadangan kenari (sama dengan katilidev AiModels::KENARI_*_FALLBACK): teks
// sengaja model `:free` dari vendor lain supaya saldo nol pun tetap ada jalan keluar;
// model gambar tidak ada yang gratis, jadi cadangannya model murah dari vendor lain.
const (
	KenariTextFallback  = "nemotron-3-super-120b-a12b:free"
	KenariImageFallback = "nano-banana-2-lite"
)

// New membuat klien kenari.id.
//
// Teks ikut jatuh ke model cadangan pada 402 `insufficient_balance` (cadangannya gratis)
// dan 503 `all_providers_failed` (tidak ada biaya terpotong); gambar tidak pada 402,
// karena semua model gambar kenari berbayar.
func New(baseURL, apiKey string, timeout time.Duration) *Client {
	c := NewOpenAI("kenari", baseURL, apiKey, timeout)
	c.TextFallback = KenariTextFallback
	c.ImageFallback = KenariImageFallback
	c.TextFallbackStatuses = []int{400, 402, 404, 410, 422, 503}
	c.ImageFallbackStatuses = []int{400, 404, 410, 422, 503}
	c.ChatExtra = func(model string) map[string]any {
		switch model {
		// Model reasoning yang mode berpikirnya tidak bisa dimatikan — dikecilkan saja.
		case "step-3-7-flash:free", "step-3-7-flash", "step-5-preview:free", "gpt-oss-120b":
			return map[string]any{"reasoning_effort": "low"}
		}
		return nil
	}
	return c
}
