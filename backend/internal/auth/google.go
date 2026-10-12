package auth

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// GoogleCertsURL: kunci publik (JWKS) yang dipakai Google untuk menandatangani ID token.
const GoogleCertsURL = "https://www.googleapis.com/oauth2/v3/certs"

// GoogleClaims: isi ID token Google (Sign in with Google / Google Identity Services).
type GoogleClaims struct {
	Sub           string `json:"sub"` // id akun Google, tetap walau email diganti
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	Iss           string `json:"iss"`
	Aud           string `json:"aud"`
	Exp           int64  `json:"exp"`
	Iat           int64  `json:"iat"`
}

// GoogleVerifier memverifikasi ID token Google secara lokal (RS256 + JWKS Google yang
// di-cache), tanpa library pihak ketiga dan tanpa memanggil endpoint tokeninfo per login.
type GoogleVerifier struct {
	ClientIDs []string // OAuth Client ID (aud yang diterima)
	CertsURL  string   // default GoogleCertsURL
	HTTP      *http.Client

	mu        sync.Mutex
	keys      map[string]*rsa.PublicKey
	expires   time.Time
	lastFetch time.Time
}

func NewGoogleVerifier(clientIDs []string) *GoogleVerifier {
	return &GoogleVerifier{ClientIDs: clientIDs, CertsURL: GoogleCertsURL, HTTP: &http.Client{Timeout: 10 * time.Second}}
}

// Enabled: login Google aktif kalau minimal satu Client ID diisi.
func (g *GoogleVerifier) Enabled() bool { return g != nil && len(g.ClientIDs) > 0 }

var googleIssuers = []string{"accounts.google.com", "https://accounts.google.com"}

// clockSkew: toleransi beda jam server dengan Google.
const clockSkew = 2 * time.Minute

func (g *GoogleVerifier) Verify(ctx context.Context, idToken string) (*GoogleClaims, error) {
	if !g.Enabled() {
		return nil, errors.New("login Google belum dikonfigurasi")
	}
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}
	rawHeader, err1 := b64.DecodeString(parts[0])
	rawPayload, err2 := b64.DecodeString(parts[1])
	sig, err3 := b64.DecodeString(parts[2])
	if err1 != nil || err2 != nil || err3 != nil {
		return nil, ErrInvalidToken
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(rawHeader, &header); err != nil || header.Alg != "RS256" || header.Kid == "" {
		return nil, ErrInvalidToken
	}
	key, err := g.key(ctx, header.Kid)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, sum[:], sig); err != nil {
		return nil, ErrInvalidToken
	}
	var c GoogleClaims
	if err := json.Unmarshal(rawPayload, &c); err != nil {
		return nil, ErrInvalidToken
	}
	now := time.Now()
	switch {
	case !slices.Contains(googleIssuers, c.Iss):
		return nil, errors.New("token Google: issuer tidak valid")
	case !slices.Contains(g.ClientIDs, c.Aud):
		return nil, errors.New("token Google: client ID tidak cocok")
	case now.Add(-clockSkew).Unix() > c.Exp:
		return nil, errors.New("token Google kedaluwarsa")
	case c.Sub == "":
		return nil, ErrInvalidToken
	case c.Email == "" || !c.EmailVerified:
		return nil, errors.New("email akun Google belum terverifikasi")
	}
	return &c, nil
}

// key mengambil kunci publik berdasarkan kid. JWKS di-cache sesuai Cache-Control dari
// Google; kid yang belum dikenal memicu fetch ulang (maksimal sekali per menit) karena
// Google merotasi kunci secara berkala.
func (g *GoogleVerifier) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if k, ok := g.keys[kid]; ok && time.Now().Before(g.expires) {
		return k, nil
	}
	if time.Since(g.lastFetch) > time.Minute || time.Now().After(g.expires) {
		if err := g.fetch(ctx); err != nil {
			// Kunci lama masih boleh dipakai kalau Google sedang tidak bisa dihubungi.
			if k, ok := g.keys[kid]; ok {
				return k, nil
			}
			return nil, err
		}
	}
	if k, ok := g.keys[kid]; ok {
		return k, nil
	}
	return nil, ErrInvalidToken
}

var maxAgeRe = regexp.MustCompile(`max-age=(\d+)`)

func (g *GoogleVerifier) fetch(ctx context.Context) error {
	g.lastFetch = time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.CertsURL, nil)
	if err != nil {
		return err
	}
	res, err := g.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("gagal mengambil kunci Google: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("gagal mengambil kunci Google: HTTP %d", res.StatusCode)
	}
	var jwks struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(res.Body).Decode(&jwks); err != nil {
		return fmt.Errorf("kunci Google tidak valid: %w", err)
	}
	keys := map[string]*rsa.PublicKey{}
	for _, k := range jwks.Keys {
		if k.Kty != "RSA" {
			continue
		}
		n, err1 := b64.DecodeString(k.N)
		e, err2 := b64.DecodeString(k.E)
		if err1 != nil || err2 != nil {
			continue
		}
		keys[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
	}
	if len(keys) == 0 {
		return errors.New("kunci Google kosong")
	}
	ttl := time.Hour
	if m := maxAgeRe.FindStringSubmatch(res.Header.Get("Cache-Control")); m != nil {
		if sec, err := strconv.Atoi(m[1]); err == nil && sec > 0 {
			ttl = time.Duration(sec) * time.Second
		}
	}
	g.keys, g.expires = keys, time.Now().Add(ttl)
	return nil
}
