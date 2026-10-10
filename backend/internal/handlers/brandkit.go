package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/kikiraihan/racik/backend/internal/ai"
	"github.com/kikiraihan/racik/backend/internal/httpx"
)

type brandKitDTO struct {
	Name    string   `json:"name"`
	Colors  []string `json:"colors"`
	Fonts   []string `json:"fonts"`
	LogoURL string   `json:"logoUrl"`
	Voice   string   `json:"voice"`
}

func (s *Server) loadBrandKit(ctx context.Context, userID string) brandKitDTO {
	bk := brandKitDTO{Name: "Brand saya", Colors: []string{}, Fonts: []string{}}
	var colors, fonts []byte
	err := s.DB.QueryRowContext(ctx, `SELECT name, colors, fonts, logo_url, voice FROM brand_kits WHERE user_id=$1`, userID).
		Scan(&bk.Name, &colors, &fonts, &bk.LogoURL, &bk.Voice)
	if err == nil {
		json.Unmarshal(colors, &bk.Colors)
		json.Unmarshal(fonts, &bk.Fonts)
	}
	return bk
}

func (s *Server) brandContext(ctx context.Context, userID string) *ai.BrandContext {
	bk := s.loadBrandKit(ctx, userID)
	return &ai.BrandContext{Name: bk.Name, Colors: bk.Colors, Fonts: bk.Fonts, Voice: bk.Voice}
}

func (s *Server) getBrandKit(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, 200, s.loadBrandKit(r.Context(), httpx.CurrentUser(r).ID))
}

func (s *Server) putBrandKit(w http.ResponseWriter, r *http.Request) {
	var in brandKitDTO
	if err := httpx.Decode(r, &in, 1<<20); err != nil {
		httpx.Error(w, 400, "body tidak valid")
		return
	}
	if in.Colors == nil {
		in.Colors = []string{}
	}
	if in.Fonts == nil {
		in.Fonts = []string{}
	}
	colors, _ := json.Marshal(in.Colors)
	fonts, _ := json.Marshal(in.Fonts)
	_, err := s.DB.ExecContext(r.Context(), `
		INSERT INTO brand_kits(user_id, name, colors, fonts, logo_url, voice, updated_at) VALUES($1,$2,$3,$4,$5,$6,now())
		ON CONFLICT (user_id) DO UPDATE SET name=EXCLUDED.name, colors=EXCLUDED.colors, fonts=EXCLUDED.fonts,
			logo_url=EXCLUDED.logo_url, voice=EXCLUDED.voice, updated_at=now()`,
		httpx.CurrentUser(r).ID, in.Name, colors, fonts, in.LogoURL, in.Voice)
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, in)
}
