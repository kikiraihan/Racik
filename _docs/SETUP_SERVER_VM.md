# Panduan Setup Racik di Server VM

Panduan ini berurutan dari nol sampai Racik jalan di `https://racik.katili.dev` dan ter-deploy
otomatis tiap `git push origin main`. Ikuti dari atas ke bawah; setiap bagian diakhiri **cek**
yang harus lolos sebelum lanjut.

VM-nya **sama** dengan katilidev dan our-life-planner, jadi beberapa hal (Docker, nginx host,
certbot, swap, user deploy) kemungkinan besar sudah ada — bagian itu cukup dicek, tidak perlu
dipasang ulang.

> Penjelasan lebih panjang (kenapa begini, opsi lain, troubleshooting lengkap):
> [`DEPLOY.md`](../DEPLOY.md) dan [`CICD.md`](../CICD.md). Dokumen ini adalah versi
> "jalankan perintah ini, berurutan".

---

## Ringkasan

| Item | Nilai |
|---|---|
| VM | `43.128.81.214` (sama dengan katilidev & our-life-planner) |
| Folder repo | `/var/www/racik` |
| Branch produksi | `main` |
| Domain (contoh) | `racik.katili.dev` — ganti di semua perintah kalau beda |
| Port host | `127.0.0.1:8091` (frontend Nuxt). db / Elasticsearch: port acak, hanya localhost |
| Container | `racik-db-1`, `racik-backend-1`, `racik-frontend-1`, `racik-elasticsearch-1` |
| Volume data | `racik_pgdata` (database), `racik_uploads` (gambar), `racik_esdata` (system log) |

Port yang sudah dipakai app lain di VM ini: **8082** & **5433** (katilidev), **8090**
(our-life-planner), **127.0.0.1:3000** (undangankami-waha). Jangan pakai port itu.

### Checklist

- [ ] 1. Login & cek resource VM
- [ ] 2. Docker, git, curl, make tersedia
- [ ] 3. Setting kernel untuk Elasticsearch
- [ ] 4. Akses GitHub dari VM (deploy key)
- [ ] 5. Clone repo
- [ ] 6. Isi `.env`
- [ ] 7. Jalankan container pertama kali
- [ ] 8. nginx host
- [ ] 9. HTTPS (certbot)
- [ ] 10. Cloudflare DNS
- [ ] 11. Login admin & cek fitur
- [ ] 12. CI/CD: SSH key GitHub Actions + secrets
- [ ] 13. Uji deploy otomatis
- [ ] 14. Backup terjadwal (cron)

---

## 1. Login & cek resource VM

```bash
ssh root@43.128.81.214
```

```bash
free -h                 # RAM: total & available
swapon --show           # swap sudah ada?
df -h /                 # sisa disk
nproc
docker stats --no-stream --format 'table {{.Name}}\t{{.MemUsage}}'   # pemakaian app lain
```

Racik menambah ±1,5 GB RAM (Elasticsearch ±1 GB, sisanya Postgres + Go + Node), plus lonjakan
saat build `nuxt build`.

| Kondisi | Tindakan |
|---|---|
| RAM available ≥ 2 GB | lanjut |
| RAM available 1–2 GB | nanti di `.env` isi `ES_JAVA_OPTS=-Xms256m -Xmx256m`, pastikan ada swap |
| RAM available < 1 GB | tambah swap dulu (di bawah), pertimbangkan upgrade VM |
| Disk sisa < 10 GB | bersihkan dulu: `docker system df`, `docker builder prune -f` |

Kalau belum ada swap:

```bash
sudo fallocate -l 2G /swapfile && sudo chmod 600 /swapfile && sudo mkswap /swapfile && sudo swapon /swapfile
echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab
```

**Cek:** `free -h` menunjukkan baris `Swap` > 0.

---

## 2. Docker, git, curl, make

Sudah terpasang untuk app lain — cukup verifikasi:

```bash
docker --version
docker compose version     # harus Compose v2 (perintah 'docker compose', bukan 'docker-compose')
git --version
curl --version | head -1
make --version | head -1
docker ps                  # harus jalan TANPA sudo
```

Kalau `make` tidak ada: `sudo apt-get install -y make`.
Kalau `docker ps` kena `permission denied`: `sudo usermod -aG docker $USER`, logout, login lagi.

**Cek:** semua perintah di atas jalan tanpa error, `docker ps` menampilkan container katilidev
& our-life-planner.

---

## 3. Setting kernel untuk Elasticsearch

```bash
sysctl vm.max_map_count
```

Kalau hasilnya < `262144`:

```bash
sudo sysctl -w vm.max_map_count=262144
echo 'vm.max_map_count=262144' | sudo tee /etc/sysctl.d/99-elasticsearch.conf
```

Setting ini berlaku untuk seluruh VM tapi tidak berpengaruh ke app lain.

**Cek:** `sysctl vm.max_map_count` → `262144`.

---

## 4. Akses GitHub dari VM (deploy key)

Repo private → VM butuh kunci baca sendiri. Deploy key GitHub hanya berlaku untuk **satu repo**,
jadi key our-life-planner (`github-olp`) tidak bisa dipakai di sini.

Jalankan sebagai **user yang sama** yang nanti dipakai GitHub Actions (mis. `root`):

```bash
ssh-keygen -t ed25519 -C "vps-racik-readonly" -f ~/.ssh/github_racik -N ""
cat ~/.ssh/github_racik.pub
```

Salin barisnya → **github.com/kikiraihan/Racik → Settings → Deploy keys → Add deploy key**
→ judul `vps-katili` → **jangan** centang *Allow write access* → **Add key**.

Kembali ke VM:

```bash
cat >> ~/.ssh/config <<'EOF'

Host github-racik
	HostName github.com
	User git
	IdentityFile ~/.ssh/github_racik
	IdentitiesOnly yes
EOF
chmod 600 ~/.ssh/config
grep -q github.com ~/.ssh/known_hosts 2>/dev/null || ssh-keyscan github.com >> ~/.ssh/known_hosts
```

**Cek:**

```bash
ssh -T git@github-racik
# Hi kikiraihan/Racik! You've successfully authenticated, but GitHub does not provide shell access.
```

> Jangan menulis `Host github.com` di `~/.ssh/config` — kalau blok itu sudah ada (dari app lain),
> SSH memakai blok pertama yang cocok dan blok baru diabaikan diam-diam.

---

## 5. Clone repo

```bash
cd /var/www
git clone -b main git@github-racik:kikiraihan/Racik.git racik
cd /var/www/racik
chmod +x scripts/*.sh
```

**Cek:** `git remote -v` menunjukkan `git@github-racik:kikiraihan/Racik.git`, dan
`ls scripts` berisi `deploy.sh rollback.sh`.

---

## 6. Isi `.env`

```bash
cd /var/www/racik
cp .env.example .env
```

Buat secret dulu (catat hasilnya):

```bash
echo "DB_PASSWORD=$(openssl rand -hex 24)"
echo "JWT_SECRET=$(openssl rand -hex 32)"
```

Lalu `nano .env` dan ubah baris-baris ini:

```dotenv
APP_PORT=8091
DB_PASSWORD=<hasil openssl di atas>
DB_HOST_PORT=
ES_HOST_PORT=
ES_JAVA_OPTS=-Xms512m -Xmx512m          # -Xms256m -Xmx256m kalau RAM mepet (langkah 1)

JWT_SECRET=<hasil openssl di atas>
CORS_ORIGINS=https://racik.katili.dev

ADMIN_EMAILS=email-anda@domain.com
ADMIN_PASSWORD=<password awal admin>

KENARI_API_KEY=kn-...
```

Biarkan apa adanya: `DATABASE_URL`, `UPLOAD_DIR`, `LOG_ES_URL` (di Docker ditimpa otomatis oleh
`docker-compose.yml`), `PUBLIC_URL` (kosong).

> **`DB_PASSWORD` harus final sebelum langkah 7.** Postgres hanya membaca password ini saat
> volume database pertama kali dibuat. Mengganti nanti = harus `ALTER USER` juga
> (lihat DEPLOY.md langkah 1).

Amankan file-nya:

```bash
chmod 600 .env
```

**Cek:**

```bash
grep -E '^(APP_PORT|CORS_ORIGINS|ADMIN_EMAILS)=' .env
grep -cE '^(JWT_SECRET|DB_PASSWORD)=.{20,}' .env    # harus 2
ss -tlnp | grep ':8091 ' || echo "port 8091 bebas"
```

---

## 7. Jalankan container pertama kali

```bash
cd /var/www/racik
make up          # build image + start (pertama kali 3–8 menit)
make ps
```

Semua service harus `running`; `db` dan `backend` lama-lama `(healthy)`. Elasticsearch butuh
±30–60 detik untuk siap.

```bash
make logs-backend      # Ctrl+C untuk keluar
```

Yang dicari di log: `Racik API berjalan di :8080 (AI kenari.id aktif: true, ...)`. Abaikan
`PERINGATAN log: setup index template Elasticsearch gagal` **hanya** kalau muncul sekali saat ES
belum siap; kalau terus berulang lihat bagian Masalah umum.

**Cek:**

```bash
make health                         # {"ok":true,"ai":true}
curl -sI http://127.0.0.1:8091/ | head -1   # HTTP/1.1 200 OK
```

---

## 8. nginx host

```bash
sudo tee /etc/nginx/sites-available/racik.katili.dev >/dev/null <<'EOF'
server {
	listen 80;
	server_name racik.katili.dev;

	# Default nginx 1m -> upload gambar > 1 MB ditolak 413. Harus >= MAX_UPLOAD_MB (20).
	client_max_body_size 25M;

	location / {
		proxy_pass http://127.0.0.1:8091;
		proxy_http_version 1.1;
		proxy_set_header Host              $host;
		proxy_set_header X-Real-IP         $remote_addr;
		proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
		proxy_set_header X-Forwarded-Proto $scheme;

		# Generate AI bisa beberapa menit; default 60s -> 504.
		proxy_read_timeout 960s;
		proxy_send_timeout 960s;
	}
}
EOF
sudo ln -s /etc/nginx/sites-available/racik.katili.dev /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

**Cek:** `sudo nginx -t` → `syntax is ok` / `test is successful`, **tanpa**
`conflicting server name "racik.katili.dev"`.

---

## 9. HTTPS (certbot)

Supaya validasi HTTP-01 lancar, buat dulu record DNS-nya sebagai **DNS only** (langkah 10,
awan abu-abu), tunggu ±1 menit, lalu:

```bash
sudo certbot --nginx -d racik.katili.dev
```

Certbot menambah server block 443 + redirect HTTP→HTTPS. Pastikan setting penting ikut terbawa
ke block 443:

```bash
sudo nginx -T 2>/dev/null | grep -A30 'server_name racik.katili.dev' | grep -E 'listen|client_max_body_size|proxy_read_timeout'
```

Harus ada `listen 443 ssl`, `client_max_body_size 25M`, `proxy_read_timeout 960s` di block yang
sama. Kalau hilang, tambahkan manual di file `/etc/nginx/sites-available/racik.katili.dev`, lalu
`sudo nginx -t && sudo systemctl reload nginx`.

**Cek:** `sudo certbot certificates | grep -A2 racik` menampilkan sertifikat yang valid.

---

## 10. Cloudflare DNS

Dashboard Cloudflare → zone `katili.dev`:

1. **DNS → Add record**: tipe `A`, nama `racik`, isi `43.128.81.214`.
2. Proxy status — pilih salah satu:
   - **DNS only** (awan abu-abu) — disarankan untuk Racik. Request AI yang lama (Magic Design
     beberapa gambar) tidak diputus.
   - **Proxied** (awan oranye) — dapat CDN & proteksi, tapi Cloudflare memutus request yang
     lebih dari **100 detik** dengan error **524**.
3. **SSL/TLS** → **Full (strict)** (biasanya sudah berlaku untuk seluruh zone).

**Cek** (dari laptop):

```bash
curl -s https://racik.katili.dev/api/health    # {"ok":true,"ai":true}
```

---

## 11. Login admin & cek fitur

1. Buka `https://racik.katili.dev`.
2. Login dengan email di `ADMIN_EMAILS` + `ADMIN_PASSWORD` → menu **Panel admin** muncul.
3. Panel admin → **Konfigurasi AI**: pilih model teks & gambar.
4. Panel admin → **Pengguna & saldo** → **+ Saldo** untuk akun admin (hit AI memotong saldo).
5. Coba satu Magic Design kecil, lalu upload satu gambar > 1 MB di editor.
6. Panel admin → **System log**: aktivitas login & hit AI tadi harus muncul.
7. Kosongkan `ADMIN_PASSWORD` di `.env` (tidak perlu restart). Nilai itu hanya dipakai membuat
   akun admin saat belum ada (`ON CONFLICT DO NOTHING`), jadi tidak perlu disimpan di server.

**Cek:** keenam langkah berhasil tanpa 413 / 504 / 524.

---

## 12. CI/CD: SSH key GitHub Actions + secrets

**Di laptop** (bukan di VM):

```bash
ssh-keygen -t ed25519 -C "github-actions-racik" -f ~/.ssh/racik_deploy -N ""
ssh-copy-id -i ~/.ssh/racik_deploy.pub root@43.128.81.214
ssh -i ~/.ssh/racik_deploy root@43.128.81.214 "cd /var/www/racik && make version"
ssh-keyscan -H 43.128.81.214          # salin output-nya untuk SSH_KNOWN_HOSTS
```

**github.com/kikiraihan/Racik → Settings → Secrets and variables → Actions → New repository secret**:

| Secret | Isi |
|---|---|
| `SSH_HOST` | `43.128.81.214` |
| `SSH_USER` | `root` (user yang sama dengan langkah 4) |
| `SSH_PRIVATE_KEY` | seluruh isi `~/.ssh/racik_deploy` (termasuk baris BEGIN/END) |
| `SSH_KNOWN_HOSTS` | output `ssh-keyscan` di atas |
| `APP_DIR` | *(opsional)* hanya kalau folder bukan `/var/www/racik` |

Secret GitHub per repo — yang sudah diisi di our-life-planner tidak terbaca di sini, isi ulang.

**Cek:** perintah `ssh -i ~/.ssh/racik_deploy ... make version` di atas menampilkan commit.

---

## 13. Uji deploy otomatis

Pertama, uji script deploy langsung di VM:

```bash
cd /var/www/racik
make deploy
```

Harus terlihat: `Backup DB -> backups/racik-predeploy-...` → `git fetch` → build →
`Health check OK` → `Deploy selesai`.

Lalu uji lewat GitHub: tab **Actions → Deploy ke VPS → Run workflow** (branch `main`). Job `ci`
hijau dulu, lalu `deploy` menampilkan log yang sama.

**Cek:** `make version` di VM menunjukkan commit terbaru `main`.

Mulai sekarang: `git push origin main` (atau merge PR ke `main`) = deploy otomatis.

---

## 14. Backup terjadwal (cron)

Deploy sudah menyimpan 10 backup pre-deploy, tapi tetap jadwalkan backup harian:

```bash
crontab -e
```

Tambahkan (jam 03:17 & 03:27, beda menit dengan job app lain):

```cron
17 3 * * * cd /var/www/racik && make backup >/dev/null 2>&1 && ls -1t backups/racik-2*.sql.gz | tail -n +15 | xargs -r rm -f
27 3 * * 0 cd /var/www/racik && make backup-uploads >/dev/null 2>&1 && ls -1t backups/uploads-*.tar.gz | tail -n +5 | xargs -r rm -f
```

(Database harian simpan 14; upload mingguan simpan 4.) Salin folder `backups/` ke luar VM
secara berkala, mis. dari laptop:

```bash
rsync -av root@43.128.81.214:/var/www/racik/backups/ ~/backup-racik/
```

**Cek:** besok pagi `ls -lt /var/www/racik/backups | head` berisi file `racik-<tanggal>.sql.gz`.

---

## Operasional sehari-hari

| Kebutuhan | Perintah (di `/var/www/racik`) |
|---|---|
| Status / log | `make ps` · `make logs` · `make logs-backend` |
| Versi yang jalan | `make version` |
| Health | `make health` |
| Deploy manual | `make deploy` |
| Rollback | `make rollback` · `make rollback SHA=abc1234` |
| Restore DB | `make restore FILE=backups/racik-xxxx.sql.gz` |
| psql | `make psql` |
| Ubah `.env` | edit, lalu `docker compose up -d backend` (backend membaca env saat start) |
| Kibana (opsional) | `docker compose --profile kibana up -d kibana && docker compose port kibana 5601`, lalu dari laptop `ssh -L 5601:127.0.0.1:<port> root@43.128.81.214` → `http://localhost:5601` |
| Cek ruang disk Docker | `docker system df` · `docker builder prune -f --filter until=168h` |

## Masalah umum

| Gejala | Penyebab & solusi |
|---|---|
| `bind: address already in use` saat `make up` | `APP_PORT` dipakai app lain → ganti di `.env`, sesuaikan `proxy_pass` nginx |
| 502 Bad Gateway | container mati / port beda dengan `proxy_pass` → `make ps`, `make health` |
| 413 saat upload | `client_max_body_size` tidak ada di block **443** (langkah 9) |
| 504 saat generate AI | `proxy_read_timeout` belum 960s di block 443 |
| 524 saat generate AI | Cloudflare proxied, batas 100 detik → set DNS only (langkah 10) |
| Elasticsearch restart terus / exit 137 | OOM → turunkan `ES_JAVA_OPTS`, tambah swap. App tetap jalan; hanya System log kosong |
| Backend: `password authentication failed` | `DB_PASSWORD` diganti setelah volume dibuat → `ALTER USER` (DEPLOY.md langkah 1) |
| Deploy: `JWT_SECRET ... masih kosong/bawaan` | isi `JWT_SECRET` di `.env` (langkah 6) |
| Deploy: `could not read Username for 'https://github.com'` | remote masih HTTPS → `git remote set-url origin git@github-racik:kikiraihan/Racik.git` |
| Deploy: `Permission denied (publickey)` saat `git fetch` | deploy key belum ada / dibuat user lain → ulangi langkah 4 sebagai `SSH_USER` |
| Actions: `Permission denied (publickey)` di step SSH | `SSH_PRIVATE_KEY` terpotong, atau public key belum di `authorized_keys` (langkah 12) |
| Health check gagal → rollback otomatis | baca 80 baris log di output Actions; biasanya env baru belum ada di `.env` atau migrasi gagal |
