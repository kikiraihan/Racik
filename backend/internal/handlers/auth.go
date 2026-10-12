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
	Role    string `json:"role"`    // user | admin
	Balance int64  `json:"balance"` // saldo (Rupiah)
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

func (s *Server) issue(w http.ResponseWriter, r *http.Request, status int, u userDTO) {
	if u.Role == "" {
		u.Role = RoleUser
	}
	if a := httpx.ActivityFrom(r.Context()); a != nil {
		a.SetUser(u.ID, u.Email)
	}
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
	u := userDTO{Email: c.Email, Name: strings.TrimSpace(c.Name), Role: RoleUser}
	if s.isAdminEmail(u.Email) {
		u.Role = RoleAdmin
	}
	activity(r, "email", u.Email)
	err = s.DB.QueryRowContext(r.Context(),
		`INSERT INTO users(email, name, password_hash, role) VALUES($1,$2,$3,$4) RETURNING id`, u.Email, u.Name, hash, u.Role).Scan(&u.ID)
	if isUniqueViolation(err) {
		httpx.Error(w, 409, "email sudah terdaftar")
		return
	}
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	s.issue(w, r, 201, u)
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if err := httpx.Decode(r, &c, 1<<16); err != nil {
		httpx.Error(w, 400, "body tidak valid")
		return
	}
	c.Email = strings.ToLower(strings.TrimSpace(c.Email))
	// Email yang dicoba selalu dicatat (juga saat gagal) untuk mendeteksi brute force.
	activity(r, "email", c.Email)
	var u userDTO
	var hash string
	err := s.DB.QueryRowContext(r.Context(),
		`SELECT id, email, name, password_hash, is_guest, role, balance FROM users WHERE email=$1`, c.Email).
		Scan(&u.ID, &u.Email, &u.Name, &hash, &u.IsGuest, &u.Role, &u.Balance)
	if errors.Is(err, sql.ErrNoRows) {
		activity(r, "reason", "email_not_found")
		httpx.Error(w, 401, "email atau password salah")
		return
	}
	if err == nil && hash == "" {
		// Akun daftar lewat Google, belum punya password.
		activity(r, "reason", "google_only")
		activity(r, "target_user_id", u.ID)
		httpx.Error(w, 401, "akun ini terdaftar lewat Google, silakan masuk dengan Google")
		return
	}
	if err == nil && !auth.CheckPassword(hash, c.Password) {
		activity(r, "reason", "wrong_password")
		activity(r, "target_user_id", u.ID)
		httpx.Error(w, 401, "email atau password salah")
		return
	}
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	if u.Role != RoleAdmin && s.isAdminEmail(u.Email) {
		if _, err := s.DB.ExecContext(r.Context(), `UPDATE users SET role='admin' WHERE id=$1`, u.ID); err == nil {
			u.Role = RoleAdmin
		}
	}
	activity(r, "role", u.Role)
	s.issue(w, r, 200, u)
}

// guest membuat akun tamu agar user baru bisa langsung auto-generate desain
// tanpa harus mendaftar terlebih dahulu.
func (s *Server) guest(w http.ResponseWriter, r *http.Request) {
	b := make([]byte, 8)
	rand.Read(b)
	u := userDTO{Email: "guest-" + hex.EncodeToString(b) + "@guest.racik", Name: "Tamu", IsGuest: true, Role: RoleUser}
	hash, _ := auth.HashPassword(hex.EncodeToString(b) + time.Now().String())
	if err := s.DB.QueryRowContext(r.Context(),
		`INSERT INTO users(email, name, password_hash, is_guest) VALUES($1,$2,$3,true) RETURNING id`, u.Email, u.Name, hash).Scan(&u.ID); err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	s.issue(w, r, 201, u)
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
	u := userDTO{ID: cu.ID, Email: c.Email, Name: strings.TrimSpace(c.Name), Role: RoleUser}
	if s.isAdminEmail(u.Email) {
		u.Role = RoleAdmin
	}
	activity(r, "email", u.Email)
	activity(r, "guest_email", cu.Email)
	res, err := s.DB.ExecContext(r.Context(),
		`UPDATE users SET email=$1, name=$2, password_hash=$3, is_guest=false, role=$5 WHERE id=$4 AND is_guest`, u.Email, u.Name, hash, cu.ID, u.Role)
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
	_ = s.DB.QueryRowContext(r.Context(), `SELECT balance FROM users WHERE id=$1`, u.ID).Scan(&u.Balance)
	s.issue(w, r, 200, u)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	var u userDTO
	err := s.DB.QueryRowContext(r.Context(), `SELECT id, email, name, is_guest, role, balance FROM users WHERE id=$1`, httpx.CurrentUser(r).ID).
		Scan(&u.ID, &u.Email, &u.Name, &u.IsGuest, &u.Role, &u.Balance)
	if err != nil {
		httpx.Error(w, 401, "user tidak ditemukan")
		return
	}
	httpx.JSON(w, 200, u)
}

// GET /api/auth/config — konfigurasi auth publik untuk frontend (tombol Google tampil kalau
// googleClientId terisi). Dibaca saat runtime supaya client ID tidak perlu ditanam saat build.
func (s *Server) authConfig(w http.ResponseWriter, r *http.Request) {
	clientID := ""
	if s.Google.Enabled() {
		clientID = s.Google.ClientIDs[0]
	}
	httpx.JSON(w, 200, map[string]any{"googleClientId": clientID})
}

// POST /api/auth/google {credential} — sign in / sign up dengan Google. credential adalah ID
// token dari Google Identity Services, diverifikasi di backend (tanda tangan, aud, iss, exp,
// email terverifikasi). Urutan pencocokan akun:
//  1. google_sub sudah terhubung → login.
//  2. email sudah terdaftar (daftar manual) → akun dihubungkan dengan Google lalu login.
//  3. request membawa token akun tamu → akun tamu di-upgrade (desain tetap tersimpan).
//  4. selain itu → akun baru dibuat.
func (s *Server) googleAuth(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Credential string `json:"credential"`
	}
	if err := httpx.Decode(r, &req, 1<<16); err != nil || req.Credential == "" {
		httpx.Error(w, 400, "body tidak valid")
		return
	}
	if !s.Google.Enabled() {
		httpx.Error(w, 404, "login Google belum diaktifkan")
		return
	}
	g, err := s.Google.Verify(r.Context(), req.Credential)
	if err != nil {
		activity(r, "reason", err.Error())
		httpx.Error(w, 401, "login Google gagal: "+err.Error())
		return
	}
	email := strings.ToLower(strings.TrimSpace(g.Email))
	name := strings.TrimSpace(g.Name)
	activity(r, "email", email)
	ctx := r.Context()

	var u userDTO
	var googleSub sql.NullString
	err = s.DB.QueryRowContext(ctx,
		`SELECT id, email, name, is_guest, role, balance, google_sub FROM users WHERE google_sub=$1 OR email=$2
		 ORDER BY (google_sub=$1) DESC NULLS LAST LIMIT 1`, g.Sub, email).
		Scan(&u.ID, &u.Email, &u.Name, &u.IsGuest, &u.Role, &u.Balance, &googleSub)
	switch {
	case err == nil:
		// Akun ada. Email sama tapi sudah terhubung ke akun Google lain → tolak.
		if googleSub.Valid && googleSub.String != g.Sub {
			activity(r, "reason", "google_sub_mismatch")
			httpx.Error(w, 409, "email ini sudah terhubung dengan akun Google lain")
			return
		}
		if !googleSub.Valid {
			activity(r, "linked", true)
		}
		if u.Name == "" {
			u.Name = name
		}
		if u.Role != RoleAdmin && s.isAdminEmail(u.Email) {
			u.Role = RoleAdmin
		}
		if _, err := s.DB.ExecContext(ctx, `UPDATE users SET google_sub=$2, name=$3, role=$4 WHERE id=$1`,
			u.ID, g.Sub, u.Name, u.Role); err != nil {
			httpx.Error(w, 500, err.Error())
			return
		}
		activity(r, "role", u.Role)
		s.issue(w, r, 200, u)
		return
	case !errors.Is(err, sql.ErrNoRows):
		httpx.Error(w, 500, err.Error())
		return
	}

	// Akun belum ada: daftar. Kalau sedang memakai akun tamu, akun itu yang di-upgrade.
	u = userDTO{Email: email, Name: name, Role: RoleUser}
	if s.isAdminEmail(email) {
		u.Role = RoleAdmin
	}
	if guestID := s.optionalUserID(r); guestID != "" {
		res, err := s.DB.ExecContext(ctx,
			`UPDATE users SET email=$2, name=$3, password_hash='', google_sub=$4, is_guest=false, role=$5 WHERE id=$1 AND is_guest`,
			guestID, u.Email, u.Name, g.Sub, u.Role)
		if err != nil && !isUniqueViolation(err) {
			httpx.Error(w, 500, err.Error())
			return
		}
		if n, _ := rowsAffected(res); err == nil && n > 0 {
			u.ID = guestID
			activity(r, "upgraded_guest", true)
			_ = s.DB.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1`, u.ID).Scan(&u.Balance)
			s.issue(w, r, 200, u)
			return
		}
	}
	err = s.DB.QueryRowContext(ctx,
		`INSERT INTO users(email, name, password_hash, google_sub, role) VALUES($1,$2,'',$3,$4) RETURNING id`,
		u.Email, u.Name, g.Sub, u.Role).Scan(&u.ID)
	if isUniqueViolation(err) {
		// Dua request bersamaan untuk akun yang sama; request lain sudah membuat akunnya.
		httpx.Error(w, 409, "akun sedang dibuat, silakan coba lagi")
		return
	}
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	s.issue(w, r, 201, u)
}

// optionalUserID membaca user dari header Authorization tanpa mewajibkannya (endpoint publik
// yang perilakunya berbeda kalau user sudah punya sesi, mis. akun tamu).
func (s *Server) optionalUserID(r *http.Request) string {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || token == "" {
		return ""
	}
	c, err := auth.Verify(s.Cfg.JWTSecret, token)
	if err != nil {
		return ""
	}
	return c.Sub
}
