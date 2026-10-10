package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kikiraihan/racik/backend/internal/httpx"
)

type designDTO struct {
	ID        string          `json:"id"`
	Title     string          `json:"title"`
	Width     int             `json:"width"`
	Height    int             `json:"height"`
	Data      json.RawMessage `json:"data,omitempty"`
	Thumbnail string          `json:"thumbnail"`
	Starred   bool            `json:"starred"`
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

type designInput struct {
	Title     *string         `json:"title"`
	Width     *int            `json:"width"`
	Height    *int            `json:"height"`
	Data      json.RawMessage `json:"data"`
	Thumbnail *string         `json:"thumbnail"`
	Starred   *bool           `json:"starred"`
}

const maxDesignBody = 40 << 20

func (s *Server) listDesigns(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 60
	}
	rows, err := s.DB.QueryContext(r.Context(), `
		SELECT id, title, width, height, thumbnail, starred, created_at, updated_at
		FROM designs WHERE user_id=$1 AND ($2='' OR title ILIKE '%'||$2||'%')
		ORDER BY starred DESC, updated_at DESC LIMIT $3`, httpx.CurrentUser(r).ID, q, limit)
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	defer rows.Close()
	out := []designDTO{}
	for rows.Next() {
		var d designDTO
		if err := rows.Scan(&d.ID, &d.Title, &d.Width, &d.Height, &d.Thumbnail, &d.Starred, &d.CreatedAt, &d.UpdatedAt); err != nil {
			httpx.Error(w, 500, err.Error())
			return
		}
		out = append(out, d)
	}
	httpx.JSON(w, 200, out)
}

func (s *Server) createDesign(w http.ResponseWriter, r *http.Request) {
	var in designInput
	if err := httpx.Decode(r, &in, maxDesignBody); err != nil {
		httpx.Error(w, 400, "body tidak valid")
		return
	}
	title, width, height, thumb := "Desain tanpa judul", 1080, 1080, ""
	if in.Title != nil && strings.TrimSpace(*in.Title) != "" {
		title = strings.TrimSpace(*in.Title)
	}
	if in.Width != nil && *in.Width > 0 {
		width = *in.Width
	}
	if in.Height != nil && *in.Height > 0 {
		height = *in.Height
	}
	if in.Thumbnail != nil {
		thumb = *in.Thumbnail
	}
	data := in.Data
	if len(data) == 0 || !json.Valid(data) {
		data = json.RawMessage(`{"pages":[]}`)
	}
	var d designDTO
	err := s.DB.QueryRowContext(r.Context(), `
		INSERT INTO designs(user_id, title, width, height, data, thumbnail) VALUES($1,$2,$3,$4,$5,$6)
		RETURNING id, title, width, height, data, thumbnail, starred, created_at, updated_at`,
		httpx.CurrentUser(r).ID, title, width, height, []byte(data), thumb).
		Scan(&d.ID, &d.Title, &d.Width, &d.Height, &d.Data, &d.Thumbnail, &d.Starred, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 201, d)
}

func (s *Server) getDesign(w http.ResponseWriter, r *http.Request) {
	var d designDTO
	err := s.DB.QueryRowContext(r.Context(), `
		SELECT id, title, width, height, data, thumbnail, starred, created_at, updated_at
		FROM designs WHERE id=$1 AND user_id=$2`, r.PathValue("id"), httpx.CurrentUser(r).ID).
		Scan(&d.ID, &d.Title, &d.Width, &d.Height, &d.Data, &d.Thumbnail, &d.Starred, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) || isInvalidUUID(err) {
		httpx.Error(w, 404, "desain tidak ditemukan")
		return
	}
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, d)
}

func (s *Server) updateDesign(w http.ResponseWriter, r *http.Request) {
	var in designInput
	if err := httpx.Decode(r, &in, maxDesignBody); err != nil {
		httpx.Error(w, 400, "body tidak valid")
		return
	}
	var data any
	if len(in.Data) > 0 {
		if !json.Valid(in.Data) {
			httpx.Error(w, 400, "data desain bukan JSON valid")
			return
		}
		data = []byte(in.Data)
	}
	var updatedAt time.Time
	err := s.DB.QueryRowContext(r.Context(), `
		UPDATE designs SET
			title     = COALESCE($3, title),
			width     = COALESCE($4, width),
			height    = COALESCE($5, height),
			data      = COALESCE($6::jsonb, data),
			thumbnail = COALESCE($7, thumbnail),
			starred   = COALESCE($8, starred),
			updated_at = now()
		WHERE id=$1 AND user_id=$2 RETURNING updated_at`,
		r.PathValue("id"), httpx.CurrentUser(r).ID, in.Title, in.Width, in.Height, data, in.Thumbnail, in.Starred).Scan(&updatedAt)
	if errors.Is(err, sql.ErrNoRows) || isInvalidUUID(err) {
		httpx.Error(w, 404, "desain tidak ditemukan")
		return
	}
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"ok": true, "updatedAt": updatedAt})
}

func (s *Server) deleteDesign(w http.ResponseWriter, r *http.Request) {
	res, err := s.DB.ExecContext(r.Context(), `DELETE FROM designs WHERE id=$1 AND user_id=$2`, r.PathValue("id"), httpx.CurrentUser(r).ID)
	if err != nil && !isInvalidUUID(err) {
		httpx.Error(w, 500, err.Error())
		return
	}
	if n, _ := rowsAffected(res); n == 0 {
		httpx.Error(w, 404, "desain tidak ditemukan")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) duplicateDesign(w http.ResponseWriter, r *http.Request) {
	var d designDTO
	err := s.DB.QueryRowContext(r.Context(), `
		INSERT INTO designs(user_id, title, width, height, data, thumbnail)
		SELECT user_id, title || ' (salinan)', width, height, data, thumbnail FROM designs WHERE id=$1 AND user_id=$2
		RETURNING id, title, width, height, thumbnail, starred, created_at, updated_at`, r.PathValue("id"), httpx.CurrentUser(r).ID).
		Scan(&d.ID, &d.Title, &d.Width, &d.Height, &d.Thumbnail, &d.Starred, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) || isInvalidUUID(err) {
		httpx.Error(w, 404, "desain tidak ditemukan")
		return
	}
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 201, d)
}

func rowsAffected(res sql.Result) (int64, error) {
	if res == nil {
		return 0, nil
	}
	return res.RowsAffected()
}

// isInvalidUUID: id di URL bukan UUID valid -> perlakukan sebagai 404.
func isInvalidUUID(err error) bool {
	return err != nil && strings.Contains(err.Error(), "invalid input syntax for type uuid")
}
