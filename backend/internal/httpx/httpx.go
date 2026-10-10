// Package httpx berisi helper HTTP: JSON response, middleware CORS, logging, recover, dan auth.
package httpx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/kikiraihan/racik/backend/internal/auth"
)

type ctxKey int

const (
	userKey ctxKey = iota + 1
	requestIDKey
	activityKey
)

type User struct {
	ID    string
	Email string
}

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func Error(w http.ResponseWriter, status int, msg string) {
	JSON(w, status, map[string]string{"error": msg})
}

// Decode membaca body JSON dengan batas ukuran.
func Decode(r *http.Request, v any, maxBytes int64) error {
	r.Body = http.MaxBytesReader(nil, r.Body, maxBytes)
	dec := json.NewDecoder(r.Body)
	return dec.Decode(v)
}

func CurrentUser(r *http.Request) User { return UserFrom(r.Context()) }

// UserFrom membaca user login dari context (dipakai juga oleh log vendor AI).
func UserFrom(ctx context.Context) User {
	u, _ := ctx.Value(userKey).(User)
	return u
}

// RequestID: id unik per request (header X-Request-ID), untuk korelasi activity log,
// log vendor, dan transaksi saldo.
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

func newRequestID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// ClientIP: IP asli user. Frontend Nuxt mem-proxy /api sehingga IP user ada di X-Forwarded-For.
func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ip, _, _ := strings.Cut(xff, ",")
		return strings.TrimSpace(ip)
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// RequireAuth memvalidasi header Authorization: Bearer <jwt>.
func RequireAuth(secret string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		token, ok := strings.CutPrefix(h, "Bearer ")
		if !ok || token == "" {
			Error(w, http.StatusUnauthorized, "silakan login terlebih dahulu")
			return
		}
		c, err := auth.Verify(secret, token)
		if err != nil {
			Error(w, http.StatusUnauthorized, err.Error())
			return
		}
		ctx := context.WithValue(r.Context(), userKey, User{ID: c.Sub, Email: c.Email})
		if a := ActivityFrom(ctx); a != nil {
			a.SetUser(c.Sub, c.Email)
		}
		next(w, r.WithContext(ctx))
	}
}

func CORS(origins []string, next http.Handler) http.Handler {
	allowAll := slices.Contains(origins, "*")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && (allowAll || slices.Contains(origins, origin)) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Max-Age", "600")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		id := r.Header.Get("X-Request-ID")
		if len(id) == 0 || len(id) > 64 || strings.ContainsFunc(id, func(c rune) bool {
			return !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-')
		}) {
			id = newRequestID()
		}
		w.Header().Set("X-Request-ID", id)
		r = r.WithContext(context.WithValue(r.Context(), requestIDKey, id))
		rec := &statusRecorder{ResponseWriter: w, status: 200}
		defer func() {
			if v := recover(); v != nil {
				log.Printf("panic: %v", v)
				Error(rec, http.StatusInternalServerError, "terjadi kesalahan server")
			}
			log.Printf("%s %s %d %s", r.Method, r.URL.Path, rec.status, time.Since(start).Round(time.Millisecond))
		}()
		next.ServeHTTP(rec, r)
	})
}

// Activity adalah catatan satu request user untuk activity log (login, hit model AI, aksi
// admin). Dibuat oleh Track; handler boleh menambah detail lewat Set.
type Activity struct {
	Action string
	mu     sync.Mutex
	userID string
	email  string
	fields map[string]any
}

func ActivityFrom(ctx context.Context) *Activity {
	a, _ := ctx.Value(activityKey).(*Activity)
	return a
}

func (a *Activity) SetUser(id, email string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if id != "" {
		a.userID = id
	}
	if email != "" {
		a.email = email
	}
}

func (a *Activity) Set(k string, v any) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.fields == nil {
		a.fields = map[string]any{}
	}
	a.fields[k] = v
}

// Snapshot mengembalikan user & detail yang terkumpul.
func (a *Activity) Snapshot() (userID, email string, fields map[string]any) {
	a.mu.Lock()
	defer a.mu.Unlock()
	f := make(map[string]any, len(a.fields))
	for k, v := range a.fields {
		f[k] = v
	}
	return a.userID, a.email, f
}

// ActivityResult dikirim ke sink setelah handler selesai.
type ActivityResult struct {
	Activity *Activity
	Status   int
	Duration time.Duration
	Error    string // pesan error JSON untuk status >= 400
}

// errorCapture merekam status dan isi body untuk respon error (bukan respon sukses
// yang bisa berisi data besar).
type errorCapture struct {
	http.ResponseWriter
	status int
	body   []byte
}

func (e *errorCapture) WriteHeader(code int) {
	e.status = code
	e.ResponseWriter.WriteHeader(code)
}

func (e *errorCapture) Write(b []byte) (int, error) {
	if e.status >= 400 && len(e.body) < 512 {
		e.body = append(e.body, b[:min(len(b), 512-len(e.body))]...)
	}
	return e.ResponseWriter.Write(b)
}

// Track membungkus handler yang aktivitasnya perlu dicatat (fitur riskan: login, hit model,
// aksi admin) lalu meneruskan hasilnya ke sink.
func Track(action string, sink func(r *http.Request, res ActivityResult), next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a := &Activity{Action: action}
		r = r.WithContext(context.WithValue(r.Context(), activityKey, a))
		rec := &errorCapture{ResponseWriter: w, status: 200}
		start := time.Now()
		defer func() {
			res := ActivityResult{Activity: a, Status: rec.status, Duration: time.Since(start)}
			if rec.status >= 400 {
				var e struct {
					Error string `json:"error"`
				}
				if json.Unmarshal(rec.body, &e) == nil && e.Error != "" {
					res.Error = e.Error
				} else {
					res.Error = strings.TrimSpace(string(rec.body))
				}
			}
			sink(r, res)
		}()
		next(rec, r)
	}
}
