package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kikiraihan/racik/backend/internal/httpx"
)

type assetDTO struct {
	ID        string    `json:"id"`
	Filename  string    `json:"filename"`
	Mime      string    `json:"mime"`
	Size      int64     `json:"size"`
	URL       string    `json:"url"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"createdAt"`
}

var allowedMime = map[string]string{
	"image/png":     ".png",
	"image/jpeg":    ".jpg",
	"image/webp":    ".webp",
	"image/gif":     ".gif",
	"image/svg+xml": ".svg",
}

func (s *Server) assetURL(path string) string {
	return s.Cfg.PublicURL + "/uploads/" + filepath.ToSlash(path)
}

// saveAsset menyimpan file ke disk dan mencatatnya di tabel assets.
func (s *Server) saveAsset(ctx context.Context, userID string, data []byte, mime, filename, source string) (*assetDTO, error) {
	mime = strings.SplitN(mime, ";", 2)[0]
	ext, ok := allowedMime[mime]
	if !ok {
		return nil, errors.New("tipe file tidak didukung: " + mime)
	}
	b := make([]byte, 12)
	rand.Read(b)
	rel := filepath.Join(userID, hex.EncodeToString(b)+ext)
	full := filepath.Join(s.Cfg.UploadDir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(full, data, 0o644); err != nil {
		return nil, err
	}
	a := &assetDTO{Filename: filename, Mime: mime, Size: int64(len(data)), Source: source}
	err := s.DB.QueryRowContext(ctx, `INSERT INTO assets(user_id, filename, mime, size, path, source)
		VALUES($1,$2,$3,$4,$5,$6) RETURNING id, created_at`, userID, filename, mime, a.Size, rel, source).Scan(&a.ID, &a.CreatedAt)
	if err != nil {
		os.Remove(full)
		return nil, err
	}
	a.URL = s.assetURL(rel)
	return a, nil
}

func (s *Server) uploadAsset(w http.ResponseWriter, r *http.Request) {
	max := s.Cfg.MaxUploadMB << 20
	r.Body = http.MaxBytesReader(w, r.Body, max+1<<20)
	if err := r.ParseMultipartForm(max); err != nil {
		httpx.Error(w, 400, "file terlalu besar atau form tidak valid")
		return
	}
	f, hdr, err := r.FormFile("file")
	if err != nil {
		httpx.Error(w, 400, "field 'file' wajib diisi")
		return
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		httpx.Error(w, 400, err.Error())
		return
	}
	mime := http.DetectContentType(data)
	if strings.HasSuffix(strings.ToLower(hdr.Filename), ".svg") {
		mime = "image/svg+xml"
	}
	a, err := s.saveAsset(r.Context(), httpx.CurrentUser(r).ID, data, mime, filepath.Base(hdr.Filename), "upload")
	if err != nil {
		httpx.Error(w, 400, err.Error())
		return
	}
	httpx.JSON(w, 201, a)
}

func (s *Server) listAssets(w http.ResponseWriter, r *http.Request) {
	source := r.URL.Query().Get("source")
	rows, err := s.DB.QueryContext(r.Context(), `SELECT id, filename, mime, size, path, source, created_at
		FROM assets WHERE user_id=$1 AND ($2='' OR source=$2) ORDER BY created_at DESC LIMIT 200`, httpx.CurrentUser(r).ID, source)
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	defer rows.Close()
	out := []assetDTO{}
	for rows.Next() {
		var a assetDTO
		var path string
		if err := rows.Scan(&a.ID, &a.Filename, &a.Mime, &a.Size, &path, &a.Source, &a.CreatedAt); err != nil {
			httpx.Error(w, 500, err.Error())
			return
		}
		a.URL = s.assetURL(path)
		out = append(out, a)
	}
	httpx.JSON(w, 200, out)
}

func (s *Server) deleteAsset(w http.ResponseWriter, r *http.Request) {
	var path string
	err := s.DB.QueryRowContext(r.Context(), `DELETE FROM assets WHERE id=$1 AND user_id=$2 RETURNING path`,
		r.PathValue("id"), httpx.CurrentUser(r).ID).Scan(&path)
	if err != nil {
		httpx.Error(w, 404, "asset tidak ditemukan")
		return
	}
	os.Remove(filepath.Join(s.Cfg.UploadDir, path))
	w.WriteHeader(http.StatusNoContent)
}

// readOwnAsset membaca file asset milik user berdasarkan URL /uploads/... (untuk AI edit).
func (s *Server) readOwnAsset(userID, url string) ([]byte, string, error) {
	_, rel, ok := strings.Cut(url, "/uploads/")
	if !ok {
		return nil, "", errors.New("url asset tidak dikenal")
	}
	rel = filepath.Clean(rel)
	if strings.Contains(rel, "..") || !strings.HasPrefix(rel, userID+string(filepath.Separator)) {
		return nil, "", errors.New("asset bukan milik anda")
	}
	data, err := os.ReadFile(filepath.Join(s.Cfg.UploadDir, rel))
	if err != nil {
		return nil, "", err
	}
	return data, http.DetectContentType(data), nil
}
