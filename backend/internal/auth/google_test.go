package auth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func googleToken(t *testing.T, key *rsa.PrivateKey, kid string, c GoogleClaims) string {
	t.Helper()
	h, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": kid, "typ": "JWT"})
	p, _ := json.Marshal(c)
	unsigned := b64.EncodeToString(h) + "." + b64.EncodeToString(p)
	sum := sha256.Sum256([]byte(unsigned))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	return unsigned + "." + b64.EncodeToString(sig)
}

func TestGoogleVerify(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=3600")
		json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kid": "k1", "kty": "RSA", "alg": "RS256",
			"n": b64.EncodeToString(key.N.Bytes()),
			"e": b64.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}}})
	}))
	defer srv.Close()

	g := NewGoogleVerifier([]string{"client-1"})
	g.CertsURL = srv.URL
	now := time.Now()
	ok := GoogleClaims{Sub: "123", Email: "a@b.c", EmailVerified: true, Iss: "https://accounts.google.com", Aud: "client-1", Iat: now.Unix(), Exp: now.Add(time.Hour).Unix()}

	c, err := g.Verify(context.Background(), googleToken(t, key, "k1", ok))
	if err != nil || c.Sub != "123" || c.Email != "a@b.c" {
		t.Fatalf("verify: %v %+v", err, c)
	}

	bad := map[string]GoogleClaims{}
	c1 := ok
	c1.Aud = "client-lain"
	bad["aud"] = c1
	c2 := ok
	c2.Iss = "evil.com"
	bad["iss"] = c2
	c3 := ok
	c3.Exp = now.Add(-time.Hour).Unix()
	bad["exp"] = c3
	c4 := ok
	c4.EmailVerified = false
	bad["email_verified"] = c4
	for name, claims := range bad {
		if _, err := g.Verify(context.Background(), googleToken(t, key, "k1", claims)); err == nil {
			t.Errorf("%s salah harus ditolak", name)
		}
	}

	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	if _, err := g.Verify(context.Background(), googleToken(t, other, "k1", ok)); err == nil {
		t.Error("tanda tangan kunci lain harus ditolak")
	}
	if _, err := g.Verify(context.Background(), googleToken(t, key, "k-tidak-ada", ok)); err == nil {
		t.Error("kid tidak dikenal harus ditolak")
	}
	if _, err := NewGoogleVerifier(nil).Verify(context.Background(), "x.y.z"); err == nil {
		t.Error("verifier tanpa client ID harus menolak")
	}
}
