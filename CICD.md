# CI/CD — Auto deploy ke VPS

Sama dengan our-life-planner. Setelah setup ini selesai, alur kerjanya:

```
Anda: git push origin main
        │
        ▼
GitHub Actions (CI)  → gofmt, go vet, go build, go test, nuxi typecheck, nuxt build, docker build
        │  (kalau semuanya hijau)
        ▼
GitHub Actions (Deploy) → SSH ke VPS → ./scripts/deploy.sh
        │
        ├─ tolak deploy kalau JWT_SECRET masih bawaan
        ├─ backup PostgreSQL → backups/racik-predeploy-<timestamp>.sql.gz
        ├─ git reset --hard origin/main
        ├─ docker compose build  (container lama tetap melayani selama build)
        ├─ docker compose up -d   (backend menjalankan migrasi SQL otomatis saat start)
        ├─ health check http://127.0.0.1:$APP_PORT/api/health  (lewat Nuxt → Go → Postgres)
        └─ kalau build / health check gagal → ROLLBACK otomatis ke commit sebelumnya
```

Pull request hanya menjalankan CI (tanpa deploy). Kalau CI merah, deploy **tidak** dijalankan —
VPS tidak tersentuh.

Prasyarat: VPS sudah di-setup mengikuti [DEPLOY.md](DEPLOY.md) (repo ter-clone di
`/var/www/racik`, `.env` terisi, nginx host & Cloudflare jalan).

> **Satu VM dengan katilidev & our-life-planner.** Ketiganya deploy lewat SSH ke VM yang sama,
> tapi tidak saling mengganggu: folder, nama project compose (`racik`), volume, dan port
> berbeda. Yang perlu diingat cuma: kalau dua repo di-push hampir bersamaan, dua build jalan
> paralel di VM yang sama — lebih lambat, dan kalau RAM mepet bisa OOM. Kalau itu terjadi,
> jalankan ulang deploy yang gagal dari tab Actions (**Re-run jobs**).

---

## Bagian A — Setup di sisi VPS

### A1. SSH key untuk GitHub Actions

Buat key baru khusus repo ini. **Di laptop Anda**:

```bash
ssh-keygen -t ed25519 -C "github-actions-racik" -f ~/.ssh/racik_deploy -N ""
ssh-copy-id -i ~/.ssh/racik_deploy.pub -p 22 root@43.128.81.214
ssh -i ~/.ssh/racik_deploy root@43.128.81.214 "echo koneksi-ok"
```

> Boleh juga memakai ulang key yang sudah dipakai our-life-planner (`~/.ssh/olp_deploy`) karena
> user & VM-nya sama — cukup isi secret yang sama di repo racik. Key terpisah lebih rapi: bisa
> dicabut per repo tanpa mematikan deploy repo lain.

### A2. User VPS bisa menjalankan docker tanpa sudo

```bash
docker ps          # harus jalan tanpa 'permission denied'
```

Kalau gagal: `sudo usermod -aG docker $USER`, logout, login lagi. (Kalau our-life-planner sudah
deploy otomatis dengan user yang sama, langkah ini sudah beres.)

### A3. Siapkan repo di VPS

```bash
cd /var/www/racik
git remote -v
git checkout main
git pull origin main
chmod +x scripts/deploy.sh scripts/rollback.sh
ls -la .env        # harus ada (tidak ikut git, aman dari reset --hard)
```

### A4. Beri VPS akses baca ke GitHub (WAJIB kalau repo private)

Deploy key GitHub **hanya berlaku untuk satu repo**, jadi key yang dipakai our-life-planner
(`github-olp`) tidak bisa dipakai untuk racik. Buat key + alias sendiri.

Semua di VPS, **sebagai user yang sama dengan `SSH_USER`**:

```bash
# 1. Key khusus akses GitHub repo racik
ssh-keygen -t ed25519 -C "vps-racik-readonly" -f ~/.ssh/github_racik -N ""
cat ~/.ssh/github_racik.pub      # salin barisnya
```

Buka **github.com/kikiraihan/Racik → Settings → Deploy keys → Add deploy key**, tempel, judul
mis. `vps-katili`, **jangan** centang "Allow write access".

```bash
# 2. Alias 'github-racik' (JANGAN pakai 'Host github.com' — SSH memakai blok pertama yang cocok)
cat >> ~/.ssh/config <<'EOF'

Host github-racik
	HostName github.com
	User git
	IdentityFile ~/.ssh/github_racik
	IdentitiesOnly yes
EOF
chmod 600 ~/.ssh/config

# 3. Host key github.com (kalau belum ada dari setup our-life-planner)
grep -q github.com ~/.ssh/known_hosts 2>/dev/null || ssh-keyscan github.com >> ~/.ssh/known_hosts

# 4. Remote pakai alias
cd /var/www/racik
git remote set-url origin git@github-racik:kikiraihan/Racik.git

# 5. Verifikasi — harus sukses tanpa menanyakan apa pun
ssh -T git@github-racik          # "Hi kikiraihan/Racik! You've successfully authenticated..."
git fetch origin main
```

> **Clone pertama kali** (sebelum ada folder repo): langkah 1–3 dulu, lalu
> `git clone git@github-racik:kikiraihan/Racik.git /var/www/racik`.
>
> **Repo public?** Langkah A4 tidak perlu — HTTPS tanpa kredensial sudah bisa fetch.

### A5. Coba deploy manual dulu

```bash
cd /var/www/racik
make deploy
```

Yang seharusnya terlihat: backup DB → git fetch → build → `Health check OK` → `Deploy selesai`.

### A6. Ambil host key VPS

**Di laptop**:

```bash
ssh-keyscan -p 22 -H 43.128.81.214
```

Salin seluruh output-nya untuk secret `SSH_KNOWN_HOSTS` (isinya sama dengan yang dipakai
our-life-planner, karena VM-nya sama).

---

## Bagian B — Setup di sisi GitHub

### B1. Tambahkan Secrets

**github.com/kikiraihan/Racik → Settings → Secrets and variables → Actions → New repository secret**

| Nama secret | Isi | Wajib? |
|---|---|---|
| `SSH_HOST` | IP VPS, mis. `43.128.81.214` | ✅ |
| `SSH_USER` | user SSH di VPS, mis. `root` | ✅ |
| `SSH_PRIVATE_KEY` | **seluruh isi** `~/.ssh/racik_deploy` termasuk baris `-----BEGIN...`/`-----END...` | ✅ |
| `SSH_KNOWN_HOSTS` | output `ssh-keyscan` dari langkah A6 | Sangat disarankan |
| `SSH_PORT` | port SSH kalau bukan 22 | Opsional |
| `APP_DIR` | lokasi repo di VPS kalau bukan `/var/www/racik` | Opsional |

Secret di GitHub per repo — yang sudah diisi di our-life-planner tidak terbaca dari racik, jadi
isi ulang di sini (nilai `SSH_HOST`, `SSH_USER`, `SSH_KNOWN_HOSTS` boleh sama persis).

Salin private key ke clipboard (macOS): `pbcopy < ~/.ssh/racik_deploy`

### B2. Merge ke main

Workflow ada di `.github/workflows/ci.yml` dan `.github/workflows/deploy.yml`. Begitu masuk ke
`main`, push berikutnya otomatis deploy.

### B3. Lihat hasilnya

Tab **Actions** → workflow **Deploy ke VPS**: job `ci` jalan duluan, lalu `deploy`. Log deploy
dari server tampil real-time di step "Jalankan deploy di VPS".

---

## Pemakaian sehari-hari

| Tujuan | Perintah |
|---|---|
| Deploy otomatis | `git push origin main` (±3–6 menit, build Nuxt paling lama) |
| Re-deploy tanpa commit baru | Actions → **Deploy ke VPS** → **Run workflow** (bisa centang "Lewati backup DB") |
| Deploy dari laptop tanpa GitHub | `make deploy-remote SSH_TARGET=root@43.128.81.214` |
| Deploy dari dalam VPS | `cd /var/www/racik && make deploy` |
| Rollback ke sebelum deploy terakhir | `make rollback` (di VPS) |
| Rollback ke commit tertentu | `make rollback SHA=abc1234` |
| Cek versi / health / status / log | `make version` · `make health` · `make ps` · `make logs` |
| Cek CI di lokal sebelum push | `make ci` |

Setelah rollback, `main` di GitHub masih berisi kode bermasalah — perbaiki atau `git revert`,
supaya deploy berikutnya tidak mengembalikan bug yang sama.

---

## Yang perlu diperhatikan

**Migrasi database tidak ikut mundur saat rollback.** Backend menjalankan
`backend/internal/db/migrations/*.sql` otomatis saat start. Rollback hanya memundurkan kode; kalau
versi gagal sudah sempat menjalankan migrasi yang tidak cocok dengan kode lama, restore backup
pre-deploy-nya:

```bash
ls -1t backups/racik-predeploy-*.sql.gz | head -3
make restore FILE=backups/racik-predeploy-<timestamp>.sql.gz
```

Karena itu, tulis migrasi yang **additive** (tambah kolom/tabel, jangan langsung hapus/rename)
supaya kode lama tetap jalan di atas skema baru.

**`git reset --hard` di server.** File yang diedit langsung di VPS akan hilang. Yang aman: `.env`,
`backups/`, `.deploy-previous` (di-ignore git), dan semua data di volume docker
(`racik_pgdata`, `racik_uploads`, `racik_esdata`).

**Downtime singkat.** Image di-build dulu sementara container lama tetap jalan; downtime hanya
beberapa detik saat container di-recreate.

**Build memakai resource VPS** (`npm ci` + `nuxt build` + `go build`), di VM yang juga
menjalankan katilidev & our-life-planner. Kalau build sering OOM, tambah swap:

```bash
sudo fallocate -l 2G /swapfile && sudo chmod 600 /swapfile && sudo mkswap /swapfile && sudo swapon /swapfile
echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab
```

(Kalau swap sudah dibuat waktu setup our-life-planner, cek dengan `swapon --show`.) Jangka
panjang kalau tetap berat: build image di GitHub Actions, push ke GHCR, VPS tinggal
`docker compose pull`.

**Disk.** Tiap deploy menyisakan build cache. `deploy.sh` sudah menghapus image dangling; sesekali
cek `docker system df` dan bersihkan cache build: `docker builder prune -f --filter until=168h`.

---

## Troubleshooting

**`Permission denied (publickey)` di step SSH** — `SSH_PRIVATE_KEY` terpotong (harus lengkap
dengan baris BEGIN/END), atau public key belum ada di `~/.ssh/authorized_keys` milik `SSH_USER`.

**`Host key verification failed`** — `SSH_KNOWN_HOSTS` tidak cocok (VPS ganti IP / reinstall).
Ulangi `ssh-keyscan` dan perbarui secret.

**`docker: permission denied ... Docker daemon`** — user SSH belum di grup `docker` (A2).

**`fatal: could not read Username for 'https://github.com'`** — repo private, remote masih HTTPS.
Ikuti A4.

**`git fetch gagal` dengan remote `git@github-racik:...`** — jalankan `ssh -T git@github-racik`
sebagai `SSH_USER`. `Permission denied (publickey)` → deploy key belum terpasang di repo racik
atau dibuat oleh user lain. Ingat: deploy key our-life-planner tidak berlaku untuk racik.

**`JWT_SECRET di .env masih kosong/bawaan`** — isi `JWT_SECRET` di `/var/www/racik/.env` dengan
`openssl rand -hex 32`.

**`.env belum ada di ...`** — buat dari `.env.example` (DEPLOY.md langkah 1).

**`Health check GAGAL` lalu rollback** — 80 baris log backend & frontend dicetak di output
Actions. Penyebab umum: env baru belum ditambahkan ke `.env` server, migrasi SQL gagal, atau
`DB_PASSWORD` tidak cocok dengan password di volume db.

**`cd: /var/www/racik: No such file or directory`** — set secret `APP_DIR` sesuai lokasi
sebenarnya (`pwd` di folder repo di VPS).

**`./scripts/deploy.sh: Permission denied`** — sekali di VPS: `chmod +x scripts/*.sh`.
