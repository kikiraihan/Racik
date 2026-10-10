package handlers

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/kikiraihan/racik/backend/internal/httpx"
)

//go:embed templates.json
var builtinTemplates []byte

type templateDTO struct {
	ID        string          `json:"id"`
	Slug      string          `json:"slug"`
	Title     string          `json:"title"`
	Category  string          `json:"category"`
	Width     int             `json:"width"`
	Height    int             `json:"height"`
	Spec      json.RawMessage `json:"spec,omitempty"`
	CreatedAt time.Time       `json:"createdAt"`
}

// SeedTemplates memasukkan/memperbarui template bawaan ke database.
func SeedTemplates(ctx context.Context, db *sql.DB) error {
	var items []struct {
		Slug     string          `json:"slug"`
		Title    string          `json:"title"`
		Category string          `json:"category"`
		Width    int             `json:"width"`
		Height   int             `json:"height"`
		Spec     json.RawMessage `json:"spec"`
	}
	if err := json.Unmarshal(builtinTemplates, &items); err != nil {
		return err
	}
	for _, t := range items {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO templates(slug, title, category, width, height, spec) VALUES($1,$2,$3,$4,$5,$6)
			ON CONFLICT (slug) DO UPDATE SET title=EXCLUDED.title, category=EXCLUDED.category,
				width=EXCLUDED.width, height=EXCLUDED.height, spec=EXCLUDED.spec`,
			t.Slug, t.Title, t.Category, t.Width, t.Height, []byte(t.Spec)); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) listTemplates(w http.ResponseWriter, r *http.Request) {
	cat := r.URL.Query().Get("category")
	// Spec ikut dikirim agar frontend bisa merender preview tanpa request tambahan.
	rows, err := s.DB.QueryContext(r.Context(), `SELECT id, slug, title, category, width, height, spec, created_at
		FROM templates WHERE ($1='' OR category=$1) ORDER BY category, title`, cat)
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	defer rows.Close()
	out := []templateDTO{}
	for rows.Next() {
		var t templateDTO
		if err := rows.Scan(&t.ID, &t.Slug, &t.Title, &t.Category, &t.Width, &t.Height, &t.Spec, &t.CreatedAt); err != nil {
			httpx.Error(w, 500, err.Error())
			return
		}
		out = append(out, t)
	}
	httpx.JSON(w, 200, out)
}

func (s *Server) getTemplate(w http.ResponseWriter, r *http.Request) {
	var t templateDTO
	err := s.DB.QueryRowContext(r.Context(), `SELECT id, slug, title, category, width, height, spec, created_at
		FROM templates WHERE id::text=$1 OR slug=$1`, r.PathValue("id")).
		Scan(&t.ID, &t.Slug, &t.Title, &t.Category, &t.Width, &t.Height, &t.Spec, &t.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, 404, "template tidak ditemukan")
		return
	}
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, t)
}
