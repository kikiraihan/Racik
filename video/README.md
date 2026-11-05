# Video teaser

`racik-teaser.mp4` — teaser 32 detik, 1920×1080, 30 fps, tanpa suara.

| Detik | Adegan |
|---|---|
| 0–3,5 | Logo + "Racik — Tulis idemu. AI menyusun desainnya." |
| 3,5–11 | Prompt "Poster promo kopi susu gula aren…" → Generate desain → tiga variasi desain dari AI |
| 11–18 | Editor: pilih judul, ganti warna, geser badge diskon, minta AI memperbaiki halaman |
| 18–21,5 | Export ke PPTX, PDF, PNG, SVG |
| 21,5–25 | Halaman Template (screenshot `src/templates.jpg` dari racik.katili.dev) |
| 25–28,6 | Fitur: perbaiki desain, gambar AI, copywriting, Brand Kit, belajar dari selera, Magic Resize |
| 28,6–32,5 | Penutup + racik.katili.dev |

## Render ulang

Sumber animasi ada di `src/teaser.html`. Buka langsung di browser untuk pratinjau. Untuk membuat MP4
(butuh Node + Playwright/Chromium + ffmpeg, dan font Inter terpasang):

```bash
node video/src/render.js
```
