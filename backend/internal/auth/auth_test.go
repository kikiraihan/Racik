package auth

import (
	"testing"
	"time"
)

func TestPassword(t *testing.T) {
	h, err := HashPassword("rahasia123")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(h, "rahasia123") || CheckPassword(h, "salah") {
		t.Fatal("verifikasi password salah")
	}
}

func TestJWT(t *testing.T) {
	tok, err := Sign("s3cret", "u1", "a@b.c", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	c, err := Verify("s3cret", tok)
	if err != nil || c.Sub != "u1" {
		t.Fatalf("verify: %v %+v", err, c)
	}
	if _, err := Verify("lain", tok); err == nil {
		t.Fatal("secret berbeda harus gagal")
	}
	exp, _ := Sign("s3cret", "u1", "a@b.c", -time.Minute)
	if _, err := Verify("s3cret", exp); err == nil {
		t.Fatal("token kedaluwarsa harus gagal")
	}
}
