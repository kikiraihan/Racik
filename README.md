# Racik — editor desain mirip Canva + AI

Racik adalah web app editor desain (gaya Canva) dengan **AI auto-generate desain**. Cukup tulis ide, AI (Claude) menyusun layout & copy, model gambar (Google Nano Banana / GPT Image) membuat visual, lalu hasilnya bisa diedit penuh: teks, gambar, layer, dan diekspor ke **PPTX, PDF, PNG/JPG, SVG**.

- **Frontend**: Vue 3 + Nuxt 3 (SPA), editor kanvas berbasis [fabric.js](http://fabricjs.com) v6, export memakai `pptxgenjs`, `jspdf`, `jszip`
- **Backend**: Go native (`net/http` standard library, tanpa framework), JWT & hashing password juga pakai stdlib
- **Database**: PostgreSQL (driver `pgx`), migrasi SQL otomatis saat start
- **AI connector**: gateway [kenari.id](https://kenari.id) (format OpenAI-compatible) → Claude untuk layout, Nano Banana / GPT Image untuk gambar; provider lain (Anthropic, Groq, NVIDIA NIM, Agnes AI, OpenRouter, Hugging Face, Pollinations) bisa diaktifkan lewat env

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
- Zoom (Ctrl+scroll, tombol +/−, pas ke layar), **Magic Resize** ke ukuran lain dengan skala isi
- **Board kanvas**: semua halaman tampil berjajar sebagai frame; geser board (scroll, alat geser `H`, tahan Spasi, atau seret area kosong), klik frame untuk mengedit, seret label frame untuk memindah posisinya
- Layout editor: sidebar kiri (judul, token warna & font, layer, daftar halaman) dan sidebar kanan (akun, Export, riwayat AI, properti, saldo) yang bisa ditutup (`Ctrl+\`), tool rail & kontrol zoom melayang, serta **kotak chat AI melayang** di bawah kanvas (Perbaiki halaman / Halaman baru / Gambar AI)
- Autosave ke server + thumbnail
- **Export**: PNG (bisa transparan), JPG, PDF multi halaman, **PPTX dengan teks yang bisa diedit** (atau mode gambar penuh), SVG, Racik JSON; multi halaman otomatis di-zip untuk format gambar

### AI (via kenari.id)
- **Magic Design**: prompt → desain multi halaman lengkap (layout, copy, palet, font, gambar AI); bisa ditambahkan sebagai halaman baru / ganti halaman / ganti semua
- **Perbaiki desain** (Magic Redesign): AI membaca halaman aktif lalu menyusun ulang sesuai instruksi ("lebih minimalis", "warna lebih cerah", dst.)
- **Gambar AI**: generate gambar baru (Nano Banana / GPT Image), isi placeholder gambar dari layout AI, gaya & rasio
- **Edit gambar AI**: ubah gambar terpilih dengan instruksi
- **Copywriting**: 5 alternatif headline/tagline/caption/CTA, atau tulis ulang teks terpilih
- **Personalisasi**: rating 👍/👎 + masukan, Brand Kit (warna, font, logo, tone of voice) otomatis dipakai AI
- Pemilihan model teks & gambar lintas provider oleh admin (Panel admin → Konfigurasi AI), diambil dari **Katalog model** (halaman terpisah): model env otomatis masuk sebagai bawaan, model OpenRouter & kenari bisa ditambah dari daftar `GET /models` provider

### Akses public & admin, saldo, system log
- **Dua akses**: user biasa (public) dan **admin**. Admin ditentukan lewat `ADMIN_EMAILS`; admin lain bisa diangkat dari panel admin.
- **User tidak tahu dalaman AI**: provider & model tidak pernah dikirim ke browser user (tidak ada pilihan model, pesan error vendor diganti pesan umum + kode request). Model per fitur (teks/layout, gambar, edit gambar) dipilih admin di **Panel admin → Konfigurasi AI**; API key provider tetap di env.
- **Saldo (top up)**: setiap hit AI memotong saldo (Rupiah). Editor biasa (teks, gambar, layer, export, template) **gratis**.
  - Biaya: Magic Design / Perbaiki = 1x tarif model teks + tarif model gambar × gambar yang berhasil dibuat; Gambar AI = tarif gambar; Edit gambar = tarif edit; Copywriting = tarif teks.
  - Saldo dipotong di depan (atomik, tidak bisa minus), lalu **dikembalikan otomatis** kalau vendor gagal / gambar gagal dibuat. Saldo kurang → `402` dengan `code: insufficient_balance`; kalau saldo hanya cukup untuk sebagian gambar, sisanya dilewati.
  - Riwayat lengkap di halaman **Saldo** (`/wallet`), tersimpan di tabel `wallet_transactions`.
- **Konfigurasi harga** (Panel admin): harga ke user = **harga vendor + 5%** (markup global bisa diubah, juga markup khusus per model); model yang gratis di vendor / belum diatur = **Rp 100 per hit**. Harga vendor diisi admin dalam Rupiah per hit.
- **Tambah saldo oleh admin** (tahap testing, sebelum ada payment gateway): Panel admin → Pengguna & saldo → tombol **+ Saldo** (nilai negatif = koreksi). Tercatat di riwayat saldo & activity log.
- **System log ke Elasticsearch** (lihat [System log](#system-log-elasticsearch)): activity user untuk fitur riskan (login/daftar/tamu, hit model AI, aksi admin) dan semua request ke vendor AI. Bisa dicari dari **Panel admin → System log** atau Kibana.
- **Panel admin** (`/admin`) punya tampilan sendiri, beda dari halaman public: sidebar gelap (bisa diciutkan `Ctrl+\`, jadi drawer di mobile), topbar dengan judul halaman, dan **mode gelap** (terang / gelap / ikut sistem, hanya berlaku di panel admin). Menu:
  - **Ringkasan**: jumlah user, generate AI hari ini, pemakaian saldo & top up 30 hari, desain tersimpan, kepuasan hasil AI, grafik 14 hari, generate per fitur, status sistem (provider AI, model aktif, Elasticsearch, login Google, log server), transaksi saldo terbaru
  - **Pengguna & saldo**, **Katalog model**, **Konfigurasi AI**, **Konfigurasi harga**
  - **AI API Call Log** (lihat [AI API Call Log](#ai-api-call-log)): setiap request HTTP ke vendor AI di PostgreSQL — ringkasan per fitur & model, daftar dengan filter, detail body request/response
  - **System log** (Elasticsearch) dan **Log server**: 3000 baris log backend terakhir dari memori (access log HTTP, peringatan, error) dengan filter level/sumber, pencarian, live refresh, dan unduh `.txt` — tetap bisa dipakai walaupun Elasticsearch belum diatur (isinya hilang saat backend restart)

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
  internal/ai       # konektor AI: kenari.id + provider lain (router.go), prompt designer
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
# buka http://localhost:8091  (port = APP_PORT di .env)
```

Hanya frontend yang dipublish ke host (`127.0.0.1:${APP_PORT}`); backend, Postgres dan
Elasticsearch cuma bisa dijangkau lewat network internal docker. Port db/ES di host acak —
isi `DB_HOST_PORT=5432` di `.env` kalau ingin backend `go run` (opsi B) memakai db dari compose.

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

### Login Google

Sign in / sign up dengan Google memakai [Google Identity Services](https://developers.google.com/identity/gsi/web)
(tombol "Masuk/Daftar dengan Google" di halaman `/login`). Aktifkan dengan:

1. Google Cloud Console → **APIs & Services → OAuth consent screen**: isi nama app & email support.
2. **Credentials → Create credentials → OAuth client ID** → tipe **Web application**.
3. Di **Authorized JavaScript origins** tambahkan origin frontend, mis. `http://localhost:3000`,
   `http://localhost:8091`, `https://racik.katili.dev`. (Redirect URI tidak perlu — mode popup.)
4. Isi `GOOGLE_CLIENT_ID=<client id>` di `.env` lalu restart backend. Client secret **tidak** dipakai.
   Frontend membaca client ID dari `GET /api/auth/config` saat runtime, jadi tidak perlu build ulang.

Backend memverifikasi ID token secara lokal (tanda tangan RS256 dengan kunci publik Google yang
di-cache, `aud`, `iss`, `exp`, `email_verified`). Pencocokan akun: id akun Google (`google_sub`)
→ email yang sudah terdaftar (akun dihubungkan otomatis) → akun tamu yang sedang aktif
(di-upgrade, desain tetap tersimpan) → akun baru. Akun yang daftar lewat Google tidak punya
password, jadi login email + password untuk akun itu ditolak dengan pesan untuk masuk lewat Google.


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

### Provider AI lain

Selain kenari, backend bisa memakai provider yang juga dipakai katilidev. Provider aktif begitu
API key-nya diisi; modelnya (yang aktif di Katalog model) muncul di dropdown Panel admin → Konfigurasi AI dengan id `<provider>:<model>` (label
`model · Provider`). Model default lintas provider diatur dengan `AI_TEXT_MODEL` / `AI_IMAGE_MODEL`
(mis. `anthropic:claude-sonnet-5-5`); kosong = model kenari, atau model pertama provider aktif
kalau kenari tidak diisi. Urutan di dropdown: kenari, anthropic, groq, nvidia, agnes, openrouter,
huggingface, pollinations.

| Provider | Env key | Teks | Gambar | Edit gambar | Catatan |
|---|---|---|---|---|---|
| kenari | `KENARI_API_KEY` | ✔ | ✔ | ✔ | cadangan teks `nemotron-3-super-120b-a12b:free` (juga saat saldo habis), gambar `nano-banana-2-lite` |
| Anthropic | `ANTHROPIC_API_KEY` | ✔ | – | – | SDK resmi; `ANTHROPIC_EFFORT` (default `medium`); Sonnet/Opus 5.5 memakai server-side fallback saat permintaan ditolak |
| Groq | `GROQ_API_KEY` | ✔ | – | – | GPT-OSS dengan `reasoning_effort: low`; model yang dicabut → `openai/gpt-oss-120b` |
| NVIDIA NIM | `NVIDIA_API_KEY` | ✔ | ✔ | – | teks `integrate.api.nvidia.com`, gambar endpoint GenAI (FLUX.2/FLUX.1/SD3, ukuran kelipatan 64, prompt ≤ 800 karakter, polling 202/NVCF); GPT-OSS dengan `reasoning_effort: low`; cadangan teks `nvidia/nemotron-3-super-120b-a12b` |
| Agnes AI | `AGNES_API_KEY` | ✔ | ✔ | – | gambar diminta `size: 1K` + rasio terdekat; cadangan model 2.x |
| OpenRouter | `OPENROUTER_API_KEY` | ✔* | ✔ | ✔ | gambar & edit lewat chat completions (`modalities`); *isi `OPENROUTER_TEXT_MODELS` untuk teks |
| Hugging Face | `HUGGINGFACE_API_KEY` | – | ✔ | – | Inference API (`router.huggingface.co`); 503 = model sedang dimuat |
| Pollinations | `POLLINATIONS_ENABLED=true` | – | ✔ | – | gratis tanpa key |

**Katalog model** (Panel admin → Katalog model, `/admin/models`; seperti admin → Model AI di tiara &
katilidev) terpisah dari **Konfigurasi AI** (`/admin/ai`). Katalog = daftar model yang tersedia di
sistem (tabel `ai_models`, satu baris per provider + jenis + id model); Konfigurasi AI hanya
menawarkan model katalog yang **aktif** dari provider yang API key-nya diisi.

- **Model bawaan**: model env (`KENARI_*_MODELS`, `OPENROUTER_*_MODELS`, `GROQ_TEXT_MODELS`, …) masuk
  otomatis ke katalog setiap start (ditandai *bawaan env*). Tidak bisa dihapus — nonaktifkan saja
  kalau tidak ingin ditawarkan; status pilihan admin tidak ditimpa saat restart. Model yang dihapus
  dari env menjadi model biasa (boleh dihapus).
- **Tambah dari daftar provider**: untuk **OpenRouter** dan **kenari** (provider yang API key-nya
  diisi), katalog membaca `GET /models` provider itu (cache 10 menit, tombol Muat ulang). OpenRouter
  lengkap dengan konteks & harga USD sebagai acuan; model yang keluarannya gambar dianggap model
  gambar (dipanggil lewat chat completions, jadi juga bisa untuk edit gambar). Daftar kenari dipilah
  per kolom `endpoints` (`chat` = teks, `images` = gambar; entri tanpa `endpoints` = teks). Model
  yang ditambah langsung muncul di Konfigurasi AI tanpa mengubah env atau restart (maks. 100 model
  per provider & jenis).
- **Nonaktifkan / hapus**: model nonaktif hilang dari dropdown Konfigurasi AI. Model yang sedang
  dipilih di Konfigurasi AI tidak bisa dinonaktifkan atau dihapus (pilih model lain dulu).
- Model OpenRouter tambahan dari versi sebelumnya (`app_settings` key `openrouter_models`) dipindah
  ke katalog oleh migrasi `004_ai_models.sql`.

Endpoint admin: `GET /api/admin/models` (katalog + daftar provider), `GET /api/admin/models/remote?provider=openrouter|kenari&kind=text|image&refresh=1`,
`POST /api/admin/models` dengan body `{"provider": "kenari", "kind": "text", "ids": [...]}` (setiap id
dicek ke daftar provider), `PATCH /api/admin/models/{id}` (`status` active|disabled, `name`, `notes`),
`DELETE /api/admin/models/{id}`. `cmd/mockkenari` juga melayani `GET /v1/models` untuk mencoba katalog
kenari tanpa API key.

Edit gambar hanya didukung kenari dan OpenRouter. Kalau model gambar terpilih berasal dari provider
lain, backend memakai model edit pertama yang tersedia. Saat start, model kenari, Groq, NVIDIA (teks)
dan OpenRouter dicocokkan dengan `GET /models` masing-masing; yang tidak dikenal ditulis sebagai
`PERINGATAN` di log.

---

## Deploy & CI/CD

Production berjalan di VPS yang sama dengan katilidev & our-life-planner (nginx host +
Cloudflare di depan, container hanya di `127.0.0.1`).

- **[_docs/SETUP_SERVER_VM.md](_docs/SETUP_SERVER_VM.md)** — panduan langkah demi langkah setup di VM (checklist, perintah berurutan, cek tiap langkah).
- **[DEPLOY.md](DEPLOY.md)** — setup server pertama kali: port, `.env`, nginx host, certbot, Cloudflare, backup.
- **[CICD.md](CICD.md)** — auto deploy tiap `git push origin main` (GitHub Actions → SSH → `scripts/deploy.sh`,
  backup DB, health check, auto-rollback).
- `make help` — daftar perintah (`make deploy`, `make rollback`, `make backup`, `make ci`, …).

---

## System log (Elasticsearch)

Dua jenis log dikirim asinkron (batch lewat Bulk API, tidak memperlambat request) ke Elasticsearch:

| Index | Isi |
|---|---|
| `racik-activity-YYYY.MM` | Request user ke Racik untuk fitur riskan: `auth.login` (termasuk gagal + email yang dicoba + alasan), `auth.register`, `auth.google`, `auth.guest`, `auth.upgrade`, `ai.design`/`ai.refine`/`ai.image`/`ai.image_edit`/`ai.copy` (prompt ringkas, saldo dipotong/dikembalikan), `admin.balance`, `admin.user_role`, `admin.ai_config`, `admin.pricing`. Field: `@timestamp`, `request_id`, `action`, `user_id`, `email`, `ip`, `user_agent`, `status`, `success`, `duration_ms`, `error`, `details` |
| `racik-vendor-YYYY.MM` | Request Racik ke vendor AI: `provider`, `model`, `operation` (chat/image/edit), `feature`, `duration_ms`, `success`, `status`/`error_code`/`error` asli vendor, `prompt`, `vendor_price`, `user_price`, `user_id`, `request_id` |

- `request_id` sama dengan header `X-Request-ID`, kode yang tampil di pesan error user, dan kolom `request_id` di `wallet_transactions` — jadi satu keluhan user bisa ditelusuri sampai ke request vendor-nya.
- Saat start backend membuat index template (mapping) dan ILM policy retensi `LOG_RETENTION_DAYS` (default 90 hari).
- `LOG_ES_URL` kosong atau ES mati → log ditulis ke stdout (`[log:activity] {...}`), server tetap jalan.
- Docker Compose sudah menyertakan Elasticsearch single-node (heap 512 MB). Kibana opsional: `docker compose --profile kibana up` (port host acak di 127.0.0.1, lihat `docker compose port kibana 5601`).

Kenapa Elasticsearch: cocok untuk log terstruktur yang perlu dicari & difilter (per user, aksi, error), ada Kibana untuk dashboard, dan retensi via ILM. Kalau RAM server terbatas (ES butuh ±1 GB), alternatif yang lebih ringan adalah **VictoriaLogs** (single binary, RAM puluhan MB, menerima Bulk API Elasticsearch sehingga pengiriman log tetap jalan dengan `LOG_ES_URL=http://victorialogs:9428/insert/elasticsearch`) — tetapi pencarian di Panel admin memakai Search API Elasticsearch, jadi untuk VictoriaLogs pencarian dilakukan lewat UI VictoriaLogs/Grafana.

## AI API Call Log

Pola sama dengan AI API Call Log di tiara: **setiap request HTTP ke vendor AI** (termasuk percobaan ulang SDK, model cadangan kenari/Groq/NVIDIA, dan fallback gambar lewat chat) disimpan di tabel `ai_api_calls` PostgreSQL — selalu aktif, tidak butuh Elasticsearch.

- Isi per baris: `provider`, `endpoint` (path URL), `model` yang benar-benar dikirim, `operation` (chat/image/edit), `feature` (`ai.design`, `ai.refine`, …), `user_id`, `request_id`, `http_status`, `is_success`, `latency_ms`, token (`prompt`/`completion`/`total`, kalau dilaporkan vendor), `cost` (USD dari vendor, mis. OpenRouter), `vendor_price` (Rupiah per hit dari Konfigurasi harga, hanya yang sukses), `error_message`, ringkasan `request_body` & `response_body`.
- Pencatatan dipasang di `http.RoundTripper` klien provider (`backend/internal/ai/calllog.go`), hanya untuk panggilan lewat router AI — daftar `/models`, cek model saat start, dan unduhan gambar hasil tidak tercatat. Jawaban 200 + objek `error` dari kenari dicatat sebagai gagal.
- Gambar base64 (request edit gambar, jawaban gambar) tidak disimpan, diganti ukurannya; body JSON > 64 KB diringkas.
- Penulisan asinkron lewat antrean (request user tidak menunggu database); kalau antrean penuh baris dibuang dan dicatat di Log server.
- `AI_CALL_LOG_BODIES` (default `true`) — simpan ringkasan body; `AI_CALL_LOG_RETENTION_DAYS` (default 30, `0` = selamanya) — baris lama dihapus saat start lalu setiap 6 jam.
- `request_id` sama dengan System log / Log server; dari halaman detail ada tautan ke semua panggilan AI dalam request yang sama.

## API ringkas

| Method | Path | Keterangan |
|---|---|---|
| POST | `/api/auth/register` · `/login` · `/guest` · `/upgrade` | Auth (JWT) |
| GET | `/api/auth/config` | Config auth publik (`googleClientId`, kosong = login Google nonaktif) |
| POST | `/api/auth/google` | Sign in / sign up dengan Google (`{credential}` = ID token Google Identity Services) |
| GET | `/api/auth/me` | User aktif |
| GET/POST | `/api/designs` | List / buat desain |
| GET/PUT/DELETE | `/api/designs/{id}` | Detail / simpan / hapus |
| POST | `/api/designs/{id}/duplicate` | Duplikat |
| GET/POST/DELETE | `/api/assets` | Upload gambar (multipart `file`) |
| GET | `/api/templates` | Template bawaan |
| GET/PUT | `/api/brand-kit` | Brand kit |
| GET | `/api/ai/models` | Status layanan AI & tarif per fitur (tanpa nama model) |
| POST | `/api/ai/design` · `/refine` · `/image` · `/image-edit` · `/copy` | Fitur AI |
| GET | `/api/ai/generations` | Riwayat AI |
| POST | `/api/ai/generations/{id}/rate` | Rating 👍/👎 + masukan |
| GET | `/api/wallet` | Saldo, tarif fitur AI, riwayat transaksi |
| GET | `/api/admin/users` | (admin) Daftar user + saldo (`q`, `guests=1`, `page`) |
| POST | `/api/admin/users/{id}/balance` | (admin) Tambah/koreksi saldo `{amount, note}` |
| PATCH | `/api/admin/users/{id}` | (admin) Ubah role `{role: user\|admin}` |
| GET | `/api/admin/users/{id}/transactions` | (admin) Riwayat saldo user |
| GET/PUT | `/api/admin/ai-config` | (admin) Model teks / gambar / edit gambar |
| GET/PUT | `/api/admin/pricing` | (admin) Markup global, tarif model gratis, harga vendor per model |
| GET | `/api/admin/overview` | (admin) Angka ringkasan untuk halaman Ringkasan |
| GET | `/api/admin/server-logs` | (admin) Log server dari memori (`q`, `level=info\|warn\|error`, `source=http\|app\|log`, `after`, `limit`) |
| GET | `/api/admin/ai-calls` | (admin) AI API Call Log (`q`, `feature`, `operation`, `provider`, `model`, `success`, `userId`, `requestId`, `from`, `to` (YYYY-MM-DD), `sort`, `dir`, `page`, `size`) |
| GET | `/api/admin/ai-calls/summary` | (admin) Ringkasan per fitur & model (`days`, default 7) |
| GET | `/api/admin/ai-calls/{id}` | (admin) Detail satu panggilan + body request & response |
| POST | `/api/admin/ai-calls/delete` | (admin) Hapus log `{ids: [...]}` atau `{all: true}` |
| GET | `/api/admin/logs` | (admin) Cari system log (`type=activity\|vendor`, `q`, `action`, `success`, `userId`, `from`, `to`, `page`) |

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
- Top up saldo mandiri lewat payment gateway (QRIS/VA) — sementara saldo ditambah admin
- Saldo sambutan untuk user baru / akun tamu (saat ini akun baru mulai dari Rp 0)
