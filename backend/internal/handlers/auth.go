package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/kikiraihan/racik/backend/internal/auth"
	"github.com/kikiraihan/racik/backend/internal/httpx"
)

const tokenTTL = 30 * 24 * time.Hour

type userDTO struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	IsGuest bool   `json:"isGuest"`
}

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

func (c *credentials) validate() error {
	c.Email = strings.ToLower(strings.TrimSpace(c.Email))
	if _, err := mail.ParseAddress(c.Email); err != nil {
		return errors.New("email tidak valid")
	}
	if len(c.Password) < 8 {
		return errors.New("password minimal 8 karakter")
	}
	return nil
}

func (s *Server) issue(w http.ResponseWriter, status int, u userDTO) {
	tok, err := auth.Sign(s.Cfg.JWTSecret, u.ID, u.Email, tokenTTL)
	if err != nil {
		httpx.Error(w, 500, "gagal membuat token")
		return
	}
	httpx.JSON(w, status, map[string]any{"token": tok, "user": u})
}

func isUniqueViolation(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if err := httpx.Decode(r, &c, 1<<16); err != nil {
		httpx.Error(w, 400, "body tidak valid")
		return
	}
	if err := c.validate(); err != nil {
		httpx.Error(w, 400, err.Error())
		return
	}
	hash, err := auth.HashPassword(c.Password)
	if err != nil {
		httpx.Error(w, 500, "gagal memproses password")
		return
	}
	u := userDTO{Email: c.Email, Name: strings.TrimSpace(c.Name)}
	err = s.DB.QueryRowContext(r.Context(),
		`INSERT INTO users(email, name, password_hash) VALUES($1,$2,$3) RETURNING id`, u.Email, u.Name, hash).Scan(&u.ID)
	if isUniqueViolation(err) {
		httpx.Error(w, 409, "email sudah terdaftar")
		return
	}
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	s.issue(w, 201, u)
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if err := httpx.Decode(r, &c, 1<<16); err != nil {
		httpx.Error(w, 400, "body tidak valid")
		return
	}
	c.Email = strings.ToLower(strings.TrimSpace(c.Email))
	var u userDTO
	var hash string
	err := s.DB.QueryRowContext(r.Context(),
		`SELECT id, email, name, password_hash, is_guest FROM users WHERE email=$1`, c.Email).Scan(&u.ID, &u.Email, &u.Name, &hash, &u.IsGuest)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !auth.CheckPassword(hash, c.Password)) {
		httpx.Error(w, 401, "email atau password salah")
		return
	}
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	s.issue(w, 200, u)
}

// guest membuat akun tamu agar user baru bisa langsung auto-generate desain
// tanpa harus mendaftar terlebih dahulu.
func (s *Server) guest(w http.ResponseWriter, r *http.Request) {
	b := make([]byte, 8)
	rand.Read(b)
	u := userDTO{Email: "guest-" + hex.EncodeToString(b) + "@guest.racik", Name: "Tamu", IsGuest: true}
	hash, _ := auth.HashPassword(hex.EncodeToString(b) + time.Now().String())
	if err := s.DB.QueryRowContext(r.Context(),
		`INSERT INTO users(email, name, password_hash, is_guest) VALUES($1,$2,$3,true) RETURNING id`, u.Email, u.Name, hash).Scan(&u.ID); err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	s.issue(w, 201, u)
}

// upgradeGuest mengubah akun tamu menjadi akun permanen (desain tetap tersimpan).
func (s *Server) upgradeGuest(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if err := httpx.Decode(r, &c, 1<<16); err != nil {
		httpx.Error(w, 400, "body tidak valid")
		return
	}
	if err := c.validate(); err != nil {
		httpx.Error(w, 400, err.Error())
		return
	}
	hash, _ := auth.HashPassword(c.Password)
	cu := httpx.CurrentUser(r)
	u := userDTO{ID: cu.ID, Email: c.Email, Name: strings.TrimSpace(c.Name)}
	res, err := s.DB.ExecContext(r.Context(),
		`UPDATE users SET email=$1, name=$2, password_hash=$3, is_guest=false WHERE id=$4 AND is_guest`, u.Email, u.Name, hash, cu.ID)
	if isUniqueViolation(err) {
		httpx.Error(w, 409, "email sudah terdaftar")
		return
	}
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		httpx.Error(w, 400, "akun ini bukan akun tamu")
		return
	}
	s.issue(w, 200, u)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	var u userDTO
	err := s.DB.QueryRowContext(r.Context(), `SELECT id, email, name, is_guest FROM users WHERE id=$1`, httpx.CurrentUser(r).ID).
		Scan(&u.ID, &u.Email, &u.Name, &u.IsGuest)
	if err != nil {
		httpx.Error(w, 401, "user tidak ditemukan")
		return
	}
	httpx.JSON(w, 200, u)
}
