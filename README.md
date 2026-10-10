# Racik — editor desain mirip Canva + AI

Racik adalah web app editor desain (gaya Canva) dengan **AI auto-generate desain**. Cukup tulis ide, AI (Claude) menyusun layout & copy, model gambar (Google Nano Banana / GPT Image) membuat visual, lalu hasilnya bisa diedit penuh: teks, gambar, layer, dan diekspor ke **PPTX, PDF, PNG/JPG, SVG**.

- **Frontend**: Vue 3 + Nuxt 3 (SPA), editor kanvas berbasis [fabric.js](http://fabricjs.com) v6, export memakai `pptxgenjs`, `jspdf`, `jszip`
- **Backend**: Go native (`net/http` standard library, tanpa framework), JWT & hashing password juga pakai stdlib
- **Database**: PostgreSQL (driver `pgx`), migrasi SQL otomatis saat start
- **AI connector**: gateway [kenari.id](https://kenari.id) (format OpenAI-compatible) → Claude untuk layout, Nano Banana / GPT Image untuk gambar

---

## Product brief

| Pertanyaan | Jawaban |
|---|---|
| Siapa yang butuh? | Mahasiswa (presentasi tugas/skripsi), desainer yang butuh cepat & sederhana, tim marketing (konten sosmed, poster promo). Sekarang mereka bolak-balik antara ChatGPT untuk teks, generator gambar terpisah, lalu menyusun manual di Canva/PowerPoint. |
| Satu hal yang harus selesai di kunjungan pertama | **Auto-bikin desain dari AI.** Halaman pertama langsung kotak prompt; tidak perlu daftar (akun tamu otomatis), dalam satu klik desain jadi dan terbuka di editor. |
| 3 fitur wajib duluan | ✅ Edit gambar & teks · ✅ Layer tumpuk-tumpuk · ✅ Convert ke PPTX/PDF/Gambar |
| Kenapa lebih enak | Mirip Canva tapi custom: AI menyusun seluruh desain (bukan cuma 1 gambar), hasilnya objek yang bisa diedit, brand kit dipakai otomatis, PPTX dengan teks yang tetap bisa diedit. |
| Kenapa balik lagi | **Hasil desain makin bagus**: setiap hasil AI bisa diberi 👍/👎 + catatan. Desain yang disukai & masukan user disisipkan sebagai contoh (few-shot) di prompt berikutnya, ditambah Brand Kit — jadi AI makin paham selera user. |

---

## Fitur

### Editor (mirip Canva)
- Multi halaman: tambah, duplikat, hapus, urutkan halaman; thumbnail halaman
- **Teks**: font Google (20 font), ukuran, warna/gradien, tebal, miring, garis bawah, coret, perataan, daftar poin, huruf besar/kecil, jarak huruf & baris, bayangan, outline, sorotan, preset judul/subjudul/isi, kombinasi font
- **Gambar**: upload (klik, drag & drop ke kanvas, paste dari clipboard), filter cepat (Cerah, Dramatis, Hangat, Noir, Retro…), penyesuaian (kecerahan, kontras, saturasi, hue, blur, noise, pixelate, hitam-putih, sepia, vintage, invert), bingkai tumpul/lingkaran, flip, jadikan latar, ganti gambar
- **Elemen**: persegi, persegi tumpul, lingkaran, segitiga, bintang, pentagon, heksagon, belah ketupat, hati, panah, balon kata, garis (solid/putus/panah), bingkai foto, ikon (warna bisa diatur), emoji
- **Layer**: panel layer dengan drag untuk ubah urutan, sembunyikan/tampilkan, kunci, ganti nama, multi-select; maju/mundur/paling depan/paling belakang
- Grup / pisah grup, rata kiri/tengah/kanan/atas/bawah (ke halaman atau seleksi), distribusi, **smart guides/snapping**
- Posisi & ukuran presisi (X, Y, L, T, rotasi), transparansi, bayangan untuk semua objek
- Latar: warna, gradien (atur arah), gambar latar
- Gambar bebas (kuas), warna & ketebalan
- Undo/redo per halaman, salin/tempel/duplikat, menu klik-kanan, pintasan keyboard lengkap
- Zoom (Ctrl+scroll, slider, pas ke layar), **Magic Resize** ke ukuran lain dengan skala isi
- Autosave ke server + thumbnail
- **Export**: PNG (bisa transparan), JPG, PDF multi halaman, **PPTX dengan teks yang bisa diedit** (atau mode gambar penuh), SVG, Racik JSON; multi halaman otomatis di-zip untuk format gambar

### AI (via kenari.id)
- **Magic Design**: prompt → desain multi halaman lengkap (layout, copy, palet, font, gambar AI); bisa ditambahkan sebagai halaman baru / ganti halaman / ganti semua
- **Perbaiki desain** (Magic Redesign): AI membaca halaman aktif lalu menyusun ulang sesuai instruksi ("lebih minimalis", "warna lebih cerah", dst.)
- **Gambar AI**: generate gambar baru (Nano Banana / GPT Image), isi placeholder gambar dari layout AI, gaya & rasio
- **Edit gambar AI**: ubah gambar terpilih dengan instruksi
- **Copywriting**: 5 alternatif headline/tagline/caption/CTA, atau tulis ulang teks terpilih
- **Personalisasi**: rating 👍/👎 + masukan, Brand Kit (warna, font, logo, tone of voice) otomatis dipakai AI
- Pemilihan model teks & gambar dari UI (daftar diatur lewat env)

### Lainnya
- Landing page = kotak prompt AI, ukuran preset, galeri template (8 template bawaan), desain terbaru (cari, bintangi, duplikat, hapus)
- Akun tamu otomatis → bisa di-upgrade ke akun permanen tanpa kehilangan desain
- Brand Kit

---

## Struktur

```
backend/
  cmd/server        # API server
  cmd/mockkenari    # server tiruan kenari.id untuk demo/testing tanpa API key
  internal/ai       # konektor kenari.id (chat, image generate/edit) + prompt designer
  internal/auth     # PBKDF2 + JWT HS256 (stdlib)
  internal/db       # koneksi & migrasi (migrations/*.sql)
  internal/handlers # REST API + templates.json bawaan
frontend/
  lib/editor.ts     # inti editor (fabric.js): halaman, layer, history, snapping, filter, grup…
  lib/spec.ts       # konversi "design spec" (output AI/template) <-> objek fabric
  lib/export.ts     # export PNG/JPG/SVG/PDF/PPTX/JSON
  components/editor # UI editor (topbar, sidebar, toolbar kontekstual, panel properti, dll.)
  pages/            # landing, editor/[id], login, brand
```

### Alur AI
1. Frontend kirim brief → `POST /api/ai/design`
2. Backend menyusun prompt (skema JSON "design spec" + brand kit + contoh desain yang disukai user) → Claude via `POST {KENARI_BASE_URL}/chat/completions`
3. Elemen `image` / latar `image` yang punya `imagePrompt` di-generate paralel via `POST /images/generations` (fallback otomatis ke chat completions multimodal bila model ditolak endpoint gambar), disimpan sebagai asset
4. Frontend merender spec menjadi objek fabric yang bisa diedit penuh

---

## Menjalankan

### Opsi A — Docker Compose

```bash
cp .env.example .env          # isi KENARI_API_KEY
docker compose up --build
# buka http://localhost:3000
```

Demo tanpa API key (AI tiruan):
```bash
# di .env: KENARI_BASE_URL=http://mockkenari:9090/v1  dan  KENARI_API_KEY=mock
docker compose --profile demo up --build
```

### Opsi B — Lokal (dev)

Prasyarat: Go 1.24+, Node 22+, PostgreSQL 16.

```bash
# 1. database
createuser racik -P            # password: racik
createdb racik -O racik

# 2. backend (membaca .env di root otomatis)
cp .env.example .env
cd backend && go run ./cmd/server

# (opsional) AI tiruan tanpa API key, di terminal lain:
#   cd backend && go run ./cmd/mockkenari
#   lalu di .env: KENARI_BASE_URL=http://localhost:9090/v1, KENARI_API_KEY=mock

# 3. frontend
cd frontend && npm install && npm run dev
# buka http://localhost:3000  (/api & /uploads di-proxy ke backend :8080)
```

Ubah target proxy dengan `NUXT_API_TARGET=http://host:port` (dipakai saat `dev` dan saat `build`).

### Konfigurasi kenari.id

| Env | Default | Keterangan |
|---|---|---|
| `KENARI_BASE_URL` | `https://kenari.id/v1` | Base URL OpenAI-compatible |
| `KENARI_API_KEY` | – | API key kenari.id (wajib untuk AI) |
| `KENARI_TEXT_MODEL(S)` | `claude-sonnet-5-5` | Model untuk layout & copy (Claude) |
| `KENARI_IMAGE_MODEL(S)` | `gpt-image-2`, `nano-banana-2` | GPT Image / Nano Banana |
| `AI_TIMEOUT_SEC` | `300` | Timeout per request AI (docs kenari menyarankan ≥ 5 menit untuk gambar) |

Sesuaikan nama model dengan yang tersedia di akun kenari.id kamu — id harus **persis** seperti di
`GET https://kenari.id/v1/models` (publik, tanpa key). Saat start, backend mencocokkan
`KENARI_*_MODEL(S)` dengan daftar itu dan menulis `PERINGATAN kenari: ...` di log bila ada yang
tidak dikenal. Model default selalu ikut masuk ke daftar pilihan walau tidak ditulis di `..._MODELS`.

Perilaku kenari yang ditangani konektor (`backend/internal/ai/kenari.go`):

- Error `{"error":{"message","code"}}` diteruskan lengkap dengan `code` (mis. `insufficient_balance`)
  dan `Retry-After` untuk 429.
- Request > 45 dtk dijawab `200` + spasi lalu JSON; kegagalan di titik itu (`200` + `error`)
  dikenali sebagai error dan **tidak** diulang.
- `url` di jawaban gambar berupa `data:` URI → di-decode langsung.
- Fallback gambar lewat chat completions hanya dicoba bila model ditolak (400/404/410/422) —
  tidak untuk saldo habis (402), kuota (429), atau timeout, supaya tidak tertagih dua kali.
- Blok `<think>…</think>` dari model reasoning dibuang sebelum JSON dibaca; jawaban kosong karena
  token habis untuk reasoning menghasilkan pesan error yang jelas.

---

## API ringkas

| Method | Path | Keterangan |
|---|---|---|
| POST | `/api/auth/register` · `/login` · `/guest` · `/upgrade` | Auth (JWT) |
| GET | `/api/auth/me` | User aktif |
| GET/POST | `/api/designs` | List / buat desain |
| GET/PUT/DELETE | `/api/designs/{id}` | Detail / simpan / hapus |
| POST | `/api/designs/{id}/duplicate` | Duplikat |
| GET/POST/DELETE | `/api/assets` | Upload gambar (multipart `file`) |
| GET | `/api/templates` | Template bawaan |
| GET/PUT | `/api/brand-kit` | Brand kit |
| GET | `/api/ai/models` | Model AI yang tersedia |
| POST | `/api/ai/design` · `/refine` · `/image` · `/image-edit` · `/copy` | Fitur AI |
| GET | `/api/ai/generations` | Riwayat AI |
| POST | `/api/ai/generations/{id}/rate` | Rating 👍/👎 + masukan |

## Testing

```bash
cd backend && go test ./...
cd frontend && npx nuxi typecheck
```

## Batasan & roadmap
- Kolaborasi real-time, komentar, dan share link publik belum ada
- Crop gambar bebas (saat ini: bingkai, flip, cover-fit otomatis)
- Hapus background khusus (sementara bisa lewat "Edit gambar AI")
- Animasi/video, chart, tabel, teks melengkung
- Library foto stok (bisa diganti dengan Gambar AI)
