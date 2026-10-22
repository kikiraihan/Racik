// mockkenari: server tiruan gateway kenari.id (OpenAI-compatible) untuk demo & testing
// tanpa API key. Menghasilkan layout JSON sederhana dan gambar gradien.
//
//	go run ./cmd/mockkenari            # listen :9090
//	KENARI_BASE_URL=http://localhost:9090/v1 KENARI_API_KEY=mock go run ./cmd/server
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var palettes = [][]string{
	{"#4F46E5", "#7C3AED", "#F5F3FF", "#111827", "#FBBF24"},
	{"#0EA5E9", "#0F172A", "#F0F9FF", "#0F172A", "#F97316"},
	{"#059669", "#064E3B", "#ECFDF5", "#064E3B", "#FDE047"},
	{"#E11D48", "#9F1239", "#FFF1F2", "#1F2937", "#FACC15"},
}

func main() {
	addr := os.Getenv("MOCK_ADDR")
	if addr == "" {
		addr = ":9090"
	}
	http.HandleFunc("/v1/chat/completions", chat)
	http.HandleFunc("/v1/images/generations", images)
	http.HandleFunc("/v1/images/edits", images)
	http.HandleFunc("/v1/models", models)
	log.Printf("mock kenari.id di %s/v1", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

// models: tiruan GET /v1/models (publik di kenari), dipilah per "endpoints" — dipakai
// Panel admin → Katalog model.
func models(w http.ResponseWriter, r *http.Request) {
	type m struct {
		ID        string   `json:"id"`
		OwnedBy   string   `json:"owned_by"`
		Endpoints []string `json:"endpoints"`
	}
	writeJSON(w, map[string]any{"object": "list", "data": []m{
		{"claude-sonnet-5-5", "anthropic", []string{"chat"}},
		{"claude-haiku-5-5", "anthropic", []string{"chat"}},
		{"gpt-oss-120b", "openai", []string{"chat"}},
		{"deepseek-v4-flash", "deepseek", []string{"chat"}},
		{"step-3-7-flash:free", "stepfun", []string{"chat"}},
		{"step-5-preview:free", "stepfun", []string{"chat"}},
		{"nemotron-3-super-120b-a12b:free", "nvidia", []string{"chat"}},
		{"gpt-image-2", "openai", []string{"images"}},
		{"nano-banana-2", "google", []string{"images"}},
		{"nano-banana-2-lite", "google", []string{"images"}},
		{"hunyuan-image-v2.0", "tencent", []string{"images"}},
		{"tts-1", "openai", []string{"audio"}},
	}})
}

func chat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content any    `json:"content"`
		} `json:"messages"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	var sys, user string
	for _, m := range req.Messages {
		s, _ := m.Content.(string)
		if m.Role == "system" {
			sys = s
		} else if m.Role == "user" {
			user = s
		}
	}
	var content string
	switch {
	case strings.Contains(sys, "JSON array of 5"):
		content = `["Desain Keren dalam Sekejap", "Ide Jadi Nyata", "Lebih Cepat, Lebih Cantik", "Bikin Sekarang!", "Kreatif Tanpa Batas"]`
	default:
		content = buildSpec(user)
	}
	writeJSON(w, map[string]any{"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": content}}}})
}

var sizeRe = regexp.MustCompile(`Page size: (\d+)x(\d+)`)
var pagesRe = regexp.MustCompile(`Number of pages: (\d+)`)

func buildSpec(user string) string {
	W, H, N := 1080, 1080, 1
	if m := sizeRe.FindStringSubmatch(user); m != nil {
		W, _ = strconv.Atoi(m[1])
		H, _ = strconv.Atoi(m[2])
	}
	if m := pagesRe.FindStringSubmatch(user); m != nil {
		N, _ = strconv.Atoi(m[1])
	}
	brief := user
	if i := strings.LastIndex(user, "Brief from the user:\n"); i >= 0 {
		brief = strings.TrimSpace(user[i+len("Brief from the user:\n"):])
	} else if i := strings.Index(user, "instruction:\n"); i >= 0 {
		brief = strings.TrimSpace(user[i+len("instruction:\n"):])
	}
	h := fnv.New32a()
	h.Write([]byte(brief))
	pal := palettes[int(h.Sum32())%len(palettes)]
	title := brief
	if len(title) > 48 {
		title = title[:48]
	}
	u := float64(min(W, H))
	pages := []any{}
	for i := 0; i < max(N, 1); i++ {
		var els []any
		var bg any
		if i == 0 {
			bg = map[string]any{"type": "gradient", "colors": []string{pal[0], pal[1]}, "angle": 135}
			els = append(els,
				map[string]any{"type": "circle", "x": float64(W) * 0.62, "y": -u * 0.2, "w": u * 0.8, "h": u * 0.8, "fill": "#FFFFFF", "opacity": 0.1},
				map[string]any{"type": "image", "x": float64(W) * 0.58, "y": float64(H) * 0.18, "w": float64(W) * 0.34, "h": float64(H) * 0.64, "imagePrompt": "illustration about " + brief, "radius": u * 0.04},
				map[string]any{"type": "text", "role": "label", "text": "RACIK AI", "x": float64(W) * 0.07, "y": float64(H) * 0.26, "w": float64(W) * 0.46, "fontSize": u * 0.035, "fontFamily": "Inter", "fontWeight": "bold", "color": pal[4], "letterSpacing": 300},
				map[string]any{"type": "text", "role": "heading", "text": title, "x": float64(W) * 0.07, "y": float64(H) * 0.33, "w": float64(W) * 0.48, "fontSize": u * 0.085, "fontFamily": "Montserrat", "fontWeight": "800", "color": "#FFFFFF", "lineHeight": 1.1},
				map[string]any{"type": "rect", "x": float64(W) * 0.07, "y": float64(H) * 0.76, "w": u * 0.36, "h": u * 0.09, "fill": pal[4], "radius": u * 0.045},
				map[string]any{"type": "text", "role": "button", "text": "Mulai sekarang", "x": float64(W) * 0.07, "y": float64(H)*0.76 + u*0.025, "w": u * 0.36, "fontSize": u * 0.035, "fontFamily": "Poppins", "fontWeight": "bold", "color": pal[3], "align": "center"},
			)
		} else {
			bg = map[string]any{"type": "color", "color": pal[2]}
			els = append(els,
				map[string]any{"type": "rect", "x": 0, "y": 0, "w": u * 0.02, "h": H, "fill": pal[0]},
				map[string]any{"type": "text", "role": "heading", "text": fmt.Sprintf("Poin %d", i), "x": float64(W) * 0.08, "y": float64(H) * 0.12, "w": float64(W) * 0.84, "fontSize": u * 0.08, "fontFamily": "Montserrat", "fontWeight": "800", "color": pal[3]},
				map[string]any{"type": "text", "role": "body", "text": "• Penjelasan singkat pertama\n• Data pendukung yang relevan\n• Kesimpulan untuk audiens", "x": float64(W) * 0.08, "y": float64(H) * 0.34, "w": float64(W) * 0.84, "fontSize": u * 0.042, "fontFamily": "Inter", "color": "#374151", "lineHeight": 1.6},
				map[string]any{"type": "star", "x": float64(W) - u*0.2, "y": float64(H) - u*0.2, "w": u * 0.12, "h": u * 0.12, "fill": pal[4]},
			)
		}
		pages = append(pages, map[string]any{"name": fmt.Sprintf("Halaman %d", i+1), "background": bg, "elements": els})
	}
	b, _ := json.Marshal(map[string]any{"title": title, "palette": pal, "fonts": map[string]string{"heading": "Montserrat", "body": "Inter"}, "pages": pages})
	return "```json\n" + string(b) + "\n```"
}

func images(w http.ResponseWriter, r *http.Request) {
	width, height := 1024, 1024
	var body struct {
		Size   string `json:"size"`
		Prompt string `json:"prompt"`
	}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		json.NewDecoder(r.Body).Decode(&body)
		fmt.Sscanf(body.Size, "%dx%d", &width, &height)
	} else {
		r.ParseMultipartForm(32 << 20)
		body.Prompt = r.FormValue("prompt")
	}
	h := fnv.New32a()
	h.Write([]byte(body.Prompt))
	seed := float64(h.Sum32()%360) / 360
	img := image.NewRGBA(image.Rect(0, 0, width/4, height/4))
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			t := float64(x+y) / float64(img.Bounds().Dx()+img.Bounds().Dy())
			cx, cy := float64(x)-float64(img.Bounds().Dx())*0.6, float64(y)-float64(img.Bounds().Dy())*0.4
			d := math.Sqrt(cx*cx+cy*cy) / float64(img.Bounds().Dx())
			img.Set(x, y, hsl(math.Mod(seed+t*0.25, 1), 0.65, 0.45+0.25*math.Max(0, 0.5-d)))
		}
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	writeJSON(w, map[string]any{"data": []any{map[string]any{"b64_json": base64.StdEncoding.EncodeToString(buf.Bytes())}}})
}

func hsl(h, s, l float64) color.RGBA {
	f := func(n float64) uint8 {
		k := math.Mod(n+h*12, 12)
		a := s * math.Min(l, 1-l)
		return uint8(255 * (l - a*math.Max(-1, math.Min(math.Min(k-3, 9-k), 1))))
	}
	return color.RGBA{f(0), f(8), f(4), 255}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
