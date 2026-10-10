package ai

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Fonts yang dimuat frontend (Google Fonts). AI hanya boleh memakai font ini.
var Fonts = []string{
	"Inter", "Poppins", "Montserrat", "Playfair Display", "Roboto", "Lato", "Oswald",
	"Raleway", "Merriweather", "Bebas Neue", "Pacifico", "Dancing Script", "Space Grotesk",
	"DM Serif Display", "Nunito", "Open Sans", "Archivo Black", "Lora", "Caveat", "Anton",
}

// DesignRequest adalah input dari user untuk auto-generate desain.
type DesignRequest struct {
	Prompt         string `json:"prompt"`
	DesignType     string `json:"designType"` // presentation | instagram-post | story | poster | ...
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	Pages          int    `json:"pages"`
	Style          string `json:"style"`
	Language       string `json:"language"`
	TextModel      string `json:"textModel"`
	ImageModel     string `json:"imageModel"`
	GenerateImages bool   `json:"generateImages"`
}

// BrandContext adalah brand kit user yang disisipkan ke prompt.
type BrandContext struct {
	Name   string
	Colors []string
	Fonts  []string
	Voice  string
}

// Example adalah desain yang sebelumnya disukai/tidak disukai user.
type Example struct {
	Prompt   string
	Spec     json.RawMessage
	Liked    bool
	Feedback string
}

const specSchema = `Return ONLY a JSON object (no markdown, no commentary) with this exact shape:
{
  "title": string,
  "palette": ["#hex", ...],                 // 3-6 colors used
  "fonts": {"heading": string, "body": string},
  "pages": [
    {
      "name": string,
      "background": {"type": "color", "color": "#hex"}
                  | {"type": "gradient", "colors": ["#hex", "#hex"], "angle": number}
                  | {"type": "image", "imagePrompt": string, "overlay": "#hex or rgba()", "overlayOpacity": number 0..1},
      "elements": [  // ordered bottom -> top (layer order)
        {"type": "text", "role": "heading|subheading|body|caption|button|label",
         "text": string, "x": number, "y": number, "w": number,
         "fontSize": number, "fontFamily": string, "fontWeight": "normal"|"bold"|"600"|"800",
         "italic": boolean, "color": "#hex", "align": "left"|"center"|"right",
         "lineHeight": number, "letterSpacing": number, "uppercase": boolean},
        {"type": "rect", "x": number, "y": number, "w": number, "h": number,
         "fill": "#hex", "radius": number, "opacity": number, "stroke": "#hex", "strokeWidth": number},
        {"type": "circle", "x": number, "y": number, "w": number, "h": number, "fill": "#hex", "opacity": number},
        {"type": "triangle"|"star", "x": number, "y": number, "w": number, "h": number, "fill": "#hex", "opacity": number},
        {"type": "line", "x": number, "y": number, "w": number, "stroke": "#hex", "strokeWidth": number},
        {"type": "image", "x": number, "y": number, "w": number, "h": number,
         "imagePrompt": string, "shape": "rect"|"circle", "radius": number}
      ]
    }
  ]
}
Rules:
- Coordinates are in pixels; (0,0) is top-left of the page. Keep everything inside the page with comfortable margins (at least 5 percent of the width).
- Text "w" is the text box width; text wraps inside it. Make sure text never overlaps other text.
- letterSpacing is in 1/1000 em (0 = normal, 100-400 for spaced uppercase labels). lineHeight is a multiplier (1.0-1.6).
- Strong visual hierarchy: one dominant heading, clear spacing, aligned grid, good contrast (WCAG AA).
- Use 1-2 font families only, chosen from: %s.
- imagePrompt must be a detailed English prompt for a photo/illustration WITHOUT any text in the image.
- Use at most %d image elements/background images in total across all pages.
- Write the visible copy in %s. Be concise and specific to the user's topic, no lorem ipsum.`

const systemDesigner = `You are Racik, a senior graphic & presentation designer (Canva-level). You turn a short brief into a polished, modern, ready-to-edit layout expressed as JSON.`

// BuildDesignMessages menyusun prompt untuk LLM agar menghasilkan "design spec".
func BuildDesignMessages(req DesignRequest, brand *BrandContext, examples []Example, maxImages int) []Message {
	lang := req.Language
	if lang == "" {
		lang = "the same language as the brief (default Indonesian)"
	}
	if !req.GenerateImages {
		maxImages = 0
	}
	sys := systemDesigner + "\n\n" + fmt.Sprintf(specSchema, strings.Join(Fonts, ", "), maxImages, lang)

	var b strings.Builder
	fmt.Fprintf(&b, "Design type: %s\nPage size: %dx%d px\nNumber of pages: %d\n", nz(req.DesignType, "custom"), req.Width, req.Height, max(req.Pages, 1))
	if req.Style != "" {
		fmt.Fprintf(&b, "Visual style: %s\n", req.Style)
	}
	if req.DesignType == "presentation" {
		b.WriteString("This is a slide deck: page 1 is a title slide, the last page is a closing/CTA slide, the middle pages each cover one key point with a short heading + 2-4 bullet lines (use '• ' prefix and \\n line breaks).\n")
	}
	if brand != nil && (len(brand.Colors) > 0 || len(brand.Fonts) > 0 || brand.Voice != "") {
		b.WriteString("\nBrand kit (MUST follow):\n")
		if brand.Name != "" {
			fmt.Fprintf(&b, "- Brand name: %s\n", brand.Name)
		}
		if len(brand.Colors) > 0 {
			fmt.Fprintf(&b, "- Brand colors: %s\n", strings.Join(brand.Colors, ", "))
		}
		if len(brand.Fonts) > 0 {
			fmt.Fprintf(&b, "- Brand fonts: %s\n", strings.Join(brand.Fonts, ", "))
		}
		if brand.Voice != "" {
			fmt.Fprintf(&b, "- Tone of voice: %s\n", brand.Voice)
		}
	}
	// Personalisasi: contoh yang disukai & masukan dari desain yang tidak disukai.
	var liked, notes []string
	for _, ex := range examples {
		if ex.Liked && len(liked) < 2 {
			liked = append(liked, fmt.Sprintf("Brief: %q\nSpec: %s", ex.Prompt, compact(ex.Spec, 3500)))
		}
		if ex.Feedback != "" && len(notes) < 6 {
			mark := "disliked"
			if ex.Liked {
				mark = "liked"
			}
			notes = append(notes, fmt.Sprintf("- (%s) %s", mark, ex.Feedback))
		}
	}
	if len(liked) > 0 {
		b.WriteString("\nThe user previously LIKED these designs. Match their taste (layout rhythm, color mood, typography) without copying the content:\n")
		b.WriteString(strings.Join(liked, "\n---\n"))
		b.WriteString("\n")
	}
	if len(notes) > 0 {
		b.WriteString("\nUser feedback on past designs (apply it):\n" + strings.Join(notes, "\n") + "\n")
	}
	fmt.Fprintf(&b, "\nBrief from the user:\n%s\n", req.Prompt)

	return []Message{{Role: "system", Content: sys}, {Role: "user", Content: b.String()}}
}

// BuildRefineMessages meminta LLM memperbaiki spec yang sudah ada sesuai instruksi.
func BuildRefineMessages(spec json.RawMessage, instruction string, width, height int, allowImages bool) []Message {
	maxImg := 0
	if allowImages {
		maxImg = 2
	}
	sys := systemDesigner + "\n\n" + fmt.Sprintf(specSchema, strings.Join(Fonts, ", "), maxImg, "the same language as the existing copy")
	sys += "\n- Keep existing image elements that have a \"src\" field unchanged (copy src as-is) unless asked to replace them. New images need imagePrompt."
	user := fmt.Sprintf("Page size: %dx%d px.\nCurrent design spec:\n%s\n\nImprove/modify it according to this instruction:\n%s", width, height, compact(spec, 20000), instruction)
	return []Message{{Role: "system", Content: sys}, {Role: "user", Content: user}}
}

// BuildCopyMessages untuk AI copywriting (headline, caption, dsb).
func BuildCopyMessages(prompt, kind, voice string) []Message {
	sys := `You are a sharp copywriter. Return ONLY a JSON array of 5 short alternative texts (strings). Match the brief's language.`
	if voice != "" {
		sys += " Tone of voice: " + voice + "."
	}
	return []Message{{Role: "system", Content: sys}, {Role: "user", Content: fmt.Sprintf("Kind: %s\nBrief: %s", nz(kind, "headline"), prompt)}}
}

func compact(raw json.RawMessage, limit int) string {
	var v any
	if json.Unmarshal(raw, &v) == nil {
		if b, err := json.Marshal(v); err == nil {
			raw = b
		}
	}
	s := string(raw)
	if len(s) > limit {
		s = s[:limit]
	}
	return s
}

func nz(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}
