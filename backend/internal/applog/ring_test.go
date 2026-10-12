package applog

import (
	"fmt"
	"testing"
)

func TestRing(t *testing.T) {
	r := NewRing(3)
	fmt.Fprint(r, "2026/10/12 10:00:00 GET /api/health 200 1ms\n2026/10/12 10:00:01 POST /api/ai/design 502 3s\n")
	fmt.Fprint(r, "PERINGATAN log: setup ")
	fmt.Fprint(r, "gagal\nbaris biasa\n")

	items, last := r.Lines(RingQuery{})
	if last != 4 || len(items) != 3 {
		t.Fatalf("last=%d len=%d (ring harus menyimpan 3 baris terakhir)", last, len(items))
	}
	if items[0].Msg != "baris biasa" || items[0].Level != "info" || items[0].Source != "app" {
		t.Errorf("baris terbaru salah: %+v", items[0])
	}
	if items[1].Msg != "PERINGATAN log: setup gagal" || items[1].Level != "error" {
		t.Errorf("baris gabungan dari dua Write salah: %+v", items[1])
	}
	if items[2].Source != "http" || items[2].Level != "error" || items[2].Msg != "POST /api/ai/design 502 3s" {
		t.Errorf("access log 5xx salah: %+v", items[2])
	}

	if got, _ := r.Lines(RingQuery{Level: "error"}); len(got) != 2 {
		t.Errorf("filter error: %d baris", len(got))
	}
	if got, _ := r.Lines(RingQuery{Source: "http"}); len(got) != 1 {
		t.Errorf("filter sumber http: %d baris", len(got))
	}
	if got, _ := r.Lines(RingQuery{Text: "DESIGN"}); len(got) != 1 {
		t.Errorf("cari teks: %d baris", len(got))
	}
	if got, _ := r.Lines(RingQuery{After: 3}); len(got) != 1 || got[0].Seq != 4 {
		t.Errorf("after: %+v", got)
	}
}
