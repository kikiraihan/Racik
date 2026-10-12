# Deploy ke VPS (satu VM bareng katilidev & our-life-planner)

Racik dipasang di VPS yang **sudah** menjalankan katilidev dan our-life-planner, dengan pola
yang sama:

```
Browser → Cloudflare (HTTPS) → nginx host VPS (HTTPS via certbot) → 127.0.0.1:APP_PORT
                                                                        │
                                                         ┌──────────────┴──────────────┐
                                                         │ frontend (Nuxt, node :3000) │
                                                         │  - SPA editor               │
                                                         │  - proxy /api & /uploads    │
                                                         └──────────────┬──────────────┘
                                                                        │ network internal docker
                                          ┌─────────────────────────────┼──────────────────────┐
                                          │                             │                      │
                                ┌─────────┴─────────┐        ┌──────────┴─────────┐   ┌────────┴────────┐
                                │ backend (Go :8080)│ ─────▶ │ db (Postgres 16)   │   │ elasticsearch   │
                                │ + volume uploads  │        │ volume pgdata      │   │ (system log)    │
                                └───────────────────┘        └────────────────────┘   └─────────────────┘
```

Racik **tidak** memasang reverse proxy publik sendiri dan tidak membuka port 80/443 — itu tetap
tugas nginx host yang sudah ada. Compose hanya mem-publish **satu** port ke `127.0.0.1`
(frontend). Port db / Elasticsearch / Kibana juga cuma di `127.0.0.1` dan **acak** kecuali diisi
di `.env`, jadi tidak pernah bentrok dengan Postgres katilidev (5433) atau port milik app lain.

## Peta port di VM ini

| App | Lokasi | Port host (127.0.0.1) |
|---|---|---|
| katilidev | `/var/www/katilidev` | 8082 (nginx), 5433 (pgsql), WAHA acak |
| our-life-planner | `/var/www/our-life-planner` | 8090 |
| **racik** | `/var/www/racik` | **8091** (frontend); db/ES acak |

Cek dulu sebelum memakai 8091:

```bash
ss -tlnp | grep LISTEN
```

Kalau 8091 sudah dipakai, pilih port lain dan isi `APP_PORT` di `.env`.

## 0. Cek resource VPS

Racik menambah 4 container: Postgres, backend Go, Nuxt (node), dan **Elasticsearch** — yang
terakhir ini paling berat (heap 512 MB → ±1 GB RAM). Di VM yang sudah menjalankan katilidev
(PHP, Postgres, WAHA/Chromium) dan our-life-planner, cek dulu:

```bash
free -h      # sisa RAM
df -h /      # sisa disk (image + volume ES)
nproc
```

- RAM sisa < 1,5 GB → turunkan heap ES di `.env`: `ES_JAVA_OPTS=-Xms256m -Xmx256m`, dan
  pastikan ada swap (lihat CICD.md → "Build memakai resource VPS").
- Elasticsearch butuh `vm.max_map_count` yang cukup:

  ```bash
  sudo sysctl -w vm.max_map_count=262144
  echo 'vm.max_map_count=262144' | sudo tee /etc/sysctl.d/99-elasticsearch.conf
  ```

  Ini setting kernel untuk seluruh VM, tapi tidak mengganggu app lain.

## 1. Clone & konfigurasi

```bash
cd /var/www
git clone git@github-racik:kikiraihan/Racik.git racik   # alias github-racik: CICD.md langkah A4 (repo public: https://github.com/kikiraihan/Racik.git)
cd racik
cp .env.example .env
nano .env
```

Isi minimal `.env` untuk production:

| Variabel | Isi dengan |
|---|---|
| `APP_PORT` | port bebas dari tabel di atas, mis. `8091` |
| `DB_PASSWORD` | `openssl rand -hex 24` — **isi sebelum `make up` pertama** (lihat catatan di bawah) |
| `JWT_SECRET` | `openssl rand -hex 32` — deploy **menolak** jalan kalau masih bawaan |
| `CORS_ORIGINS` | `https://racik.katili.dev` (ganti sesuai subdomain) |
| `PUBLIC_URL` | biarkan kosong (file upload dilayani relatif lewat proxy Nuxt) |
| `ADMIN_EMAILS` / `ADMIN_PASSWORD` | email admin + password awal (ganti setelah login pertama) |
| `GOOGLE_CLIENT_ID` | opsional — OAuth Client ID untuk login Google; tambahkan `https://racik.katili.dev` di *Authorized JavaScript origins* (lihat README → Login Google) |
| `KENARI_API_KEY` | API key kenari.id (atau key provider lain) |
| `ES_JAVA_OPTS` | `-Xms512m -Xmx512m` (turunkan kalau RAM mepet) |

`DATABASE_URL`, `UPLOAD_DIR` dan `LOG_ES_URL` di `.env` **diabaikan** di Docker — compose
menimpanya dengan alamat service internal (`db`, `/data/uploads`, `elasticsearch`).

> **`DB_PASSWORD` hanya dipakai saat volume database pertama kali dibuat.** Kalau diganti
> setelahnya, ubah juga password di dalam Postgres, baru restart backend:
>
> ```bash
> docker compose exec db psql -U racik -d racik -c "ALTER USER racik PASSWORD 'password-baru';"
> # lalu ganti DB_PASSWORD di .env, kemudian:
> docker compose up -d backend
> ```

## 2. Jalankan container

```bash
make up
make ps
make logs-backend
```

Verifikasi langsung di VPS **sebelum** menyentuh nginx host:

```bash
make health                              # {"ok":true,"ai":...}  (frontend -> backend)
curl -I http://127.0.0.1:8091/           # 200, HTML Nuxt
```

## 3. Tambah server block di nginx host

Buat file baru (jangan edit config katilidev / our-life-planner):

```bash
sudo nano /etc/nginx/sites-available/racik.katili.dev
```

Isi (sesuaikan `server_name` dan port dengan `APP_PORT`):

```nginx
server {
	listen 80;
	server_name racik.katili.dev;

	# WAJIB. Default nginx 1m — upload gambar > 1 MB akan ditolak 413 sebelum sampai
	# ke Racik. Harus >= MAX_UPLOAD_MB di .env (default 20) + sedikit ruang.
	client_max_body_size 25M;

	location / {
		proxy_pass http://127.0.0.1:8091;
		proxy_http_version 1.1;
		proxy_set_header Host              $host;
		proxy_set_header X-Real-IP         $remote_addr;
		proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
		proxy_set_header X-Forwarded-Proto $scheme;

		# Generate AI (Magic Design = teks + beberapa gambar) bisa makan beberapa menit.
		# Default nginx 60s → 504 Gateway Timeout padahal backend masih bekerja.
		proxy_read_timeout 960s;
		proxy_send_timeout 960s;
	}
}
```

Aktifkan & reload:

```bash
sudo ln -s /etc/nginx/sites-available/racik.katili.dev /etc/nginx/sites-enabled/
sudo nginx -t        # pastikan TIDAK ada "conflicting server name"
sudo systemctl reload nginx
```

## 4. Sertifikat HTTPS di origin (certbot)

```bash
sudo certbot --nginx -d racik.katili.dev
```

Certbot menyisipkan `ssl_certificate` + redirect HTTP→HTTPS ke file dari langkah 3. Cek setelahnya
bahwa `client_max_body_size` dan `proxy_read_timeout` ikut ada di server block **443** (certbot
biasanya menyalin isi block, tapi pastikan dengan `sudo nginx -T | grep -A3 racik`).

> Kalau certbot gagal karena Cloudflare (proxied) mengganggu validasi HTTP-01: set dulu DNS
> record subdomain ini ke **DNS only** (awan abu-abu), jalankan ulang certbot, lalu aktifkan lagi
> proxy (awan oranye).

## 5. Setup Cloudflare

1. **DNS** → record `A` nama `racik` ke IP VPS (sama dengan subdomain lain).
2. **SSL/TLS** → **Full (strict)** (sudah berlaku untuk seluruh zone katili.dev).

> **Batas 100 detik Cloudflare.** Kalau record di-proxy (awan oranye), Cloudflare memutus request
> yang tidak dijawab dalam 100 detik dengan **error 524** — meskipun nginx host dan backend masih
> menunggu. Magic Design dengan beberapa gambar bisa melewati itu. Pilihan:
> - biarkan proxied dan terima bahwa request AI yang sangat lama bisa gagal di sisi browser
>   (cek perilaku saldo untuk kasus ini sebelum go-live), atau
> - set record `racik` ke **DNS only** (awan abu-abu): HTTPS tetap dari certbot, hanya tanpa
>   CDN/proteksi Cloudflare.

## 6. Verifikasi akhir

Buka `https://racik.katili.dev` → halaman prompt AI tampil, login dengan email di `ADMIN_EMAILS`,
Panel admin bisa dibuka.

## 7. Update ke versi terbaru

```bash
cd /var/www/racik
make deploy   # backup DB + git pull + rebuild + health check + auto-rollback kalau gagal
```

Atau otomatis tiap `git push origin main` lewat GitHub Actions — lihat **[CICD.md](CICD.md)**.

## 8. Backup & restore

```bash
make backup                                     # -> backups/racik-<timestamp>.sql.gz (pg_dump)
make backup-uploads                             # -> backups/uploads-<timestamp>.tar.gz (gambar)
make restore FILE=backups/racik-xxxx.sql.gz     # restore database
```

Tiap deploy otomatis menyimpan `backups/racik-predeploy-<timestamp>.sql.gz` (10 terakhir). Itu
bukan pengganti backup berkala — jadwalkan via cron dan salin ke luar VPS, mis.:

```cron
17 3 * * * cd /var/www/racik && make backup >/dev/null 2>&1 && ls -1t backups/racik-2*.sql.gz | tail -n +15 | xargs -r rm -f
```

Gambar upload/AI ada di volume `racik_uploads`, log di `racik_esdata`, database di
`racik_pgdata`. Ketiganya aman dari rebuild/restart; yang menghapusnya hanya `make clean`
atau `docker volume rm`.

## 9. Perintah lain

```bash
make psql       # psql ke database racik
make down       # hentikan container (volume aman)
make restart
make clean      # HATI-HATI: hapus container + SEMUA volume (database, upload, log)
docker compose port elasticsearch 9200   # port host ES (acak) untuk curl/debug
docker compose --profile kibana up -d kibana && docker compose port kibana 5601
                # lalu dari laptop: ssh -L 5601:127.0.0.1:<port> user@vps  → http://localhost:5601
```

## Troubleshooting

- **502 Bad Gateway dari nginx host** — container belum jalan, atau `APP_PORT` beda dengan
  `proxy_pass`. Cek `make ps` dan `make health`.
- **413 Request Entity Too Large saat upload** — `client_max_body_size` belum ada di server block
  443 racik (lihat langkah 3–4). Nilai di server block domain lain tidak berlaku untuk domain ini.
- **504 dari nginx saat generate AI** — `proxy_read_timeout` belum dinaikkan.
- **524 dari Cloudflare saat generate AI** — batas 100 detik Cloudflare, lihat langkah 5.
- **`bind: address already in use` saat `make up`** — `APP_PORT` (atau `DB_HOST_PORT` /
  `ES_HOST_PORT` kalau diisi) sudah dipakai app lain. Kosongkan yang opsional, ganti `APP_PORT`.
- **Elasticsearch restart terus / `exit code 137`** — kehabisan RAM (OOM). Turunkan
  `ES_JAVA_OPTS`, tambah swap. Backend tetap jalan tanpa ES (log ditulis ke stdout), hanya menu
  System log di Panel admin yang kosong.
- **Backend `password authentication failed`** — `DB_PASSWORD` di `.env` diganti setelah volume
  db dibuat. Lihat catatan di langkah 1.
- **Login gagal setelah deploy / semua user ter-logout** — `JWT_SECRET` berubah. Token lama
  tidak berlaku; login ulang.
