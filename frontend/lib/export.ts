// Export desain: PNG, JPG, SVG, PDF, PPTX (teks tetap bisa diedit di PowerPoint), JSON.
import { FabricImage, Textbox, type StaticCanvas } from 'fabric'
import { renderPageToDataURL, withPageCanvas, type PageData } from './spec'

export type ExportFormat = 'png' | 'jpg' | 'svg' | 'pdf' | 'pptx' | 'json'

export interface ExportOptions {
  format: ExportFormat
  pages: number[] // index halaman
  scale: number // multiplier resolusi
  quality: number // 0..1 untuk JPG/PDF
  transparent?: boolean
  editableText?: boolean // PPTX: teks sebagai text box asli (default true)
}

function download(href: string, filename: string) {
  const a = document.createElement('a')
  a.href = href
  a.download = filename
  document.body.appendChild(a)
  a.click()
  a.remove()
}

function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  download(url, filename)
  setTimeout(() => URL.revokeObjectURL(url), 5000)
}

const safe = (s: string) => (s || 'desain').replace(/[^\w\- ]+/g, '').trim().replace(/\s+/g, '-') || 'desain'

async function zipOrSingle(files: { name: string; data: string | Blob }[], base: string) {
  if (files.length === 1) {
    const f = files[0]
    typeof f.data === 'string' ? download(f.data, f.name) : downloadBlob(f.data, f.name)
    return
  }
  const { default: JSZip } = await import('jszip')
  const zip = new JSZip()
  for (const f of files) {
    if (typeof f.data === 'string') zip.file(f.name, f.data.split(',')[1], { base64: true })
    else zip.file(f.name, f.data)
  }
  downloadBlob(await zip.generateAsync({ type: 'blob' }), `${base}.zip`)
}

export async function exportDesign(title: string, pages: PageData[], W: number, H: number, opt: ExportOptions) {
  const base = safe(title)
  const selected = opt.pages.map((i) => pages[i]).filter(Boolean)
  const num = (i: number) => (selected.length > 1 ? `-${i + 1}` : '')

  switch (opt.format) {
    case 'png':
    case 'jpg': {
      const files = []
      for (const [i, p] of selected.entries()) {
        const fmt = opt.format === 'jpg' ? 'jpeg' : 'png'
        const data = await withPageCanvas(p, W, H, (c) => {
          const prevBg = c.backgroundColor
          if (opt.transparent && fmt === 'png') c.backgroundColor = ''
          const url = c.toDataURL({ format: fmt, quality: opt.quality, multiplier: opt.scale })
          c.backgroundColor = prevBg
          return url
        })
        files.push({ name: `${base}${num(i)}.${opt.format}`, data })
      }
      return zipOrSingle(files, base)
    }
    case 'svg': {
      const files = []
      for (const [i, p] of selected.entries()) {
        const svg = await withPageCanvas(p, W, H, (c) => c.toSVG())
        files.push({ name: `${base}${num(i)}.svg`, data: new Blob([svg], { type: 'image/svg+xml' }) })
      }
      return zipOrSingle(files, base)
    }
    case 'pdf': {
      const { jsPDF } = await import('jspdf')
      const orient = W >= H ? 'landscape' : 'portrait'
      const doc = new jsPDF({ orientation: orient, unit: 'px', format: [W, H], hotfixes: ['px_scaling'], compress: true })
      for (const [i, p] of selected.entries()) {
        if (i > 0) doc.addPage([W, H], orient)
        const img = await renderPageToDataURL(p, W, H, { format: 'jpeg', quality: opt.quality, multiplier: opt.scale })
        doc.addImage(img, 'JPEG', 0, 0, W, H, undefined, 'FAST')
      }
      doc.save(`${base}.pdf`)
      return
    }
    case 'pptx':
      return exportPptx(base, selected, W, H, opt.scale, opt.editableText !== false)
    case 'json': {
      const blob = new Blob([JSON.stringify({ app: 'racik', version: 1, title, width: W, height: H, pages }, null, 0)], { type: 'application/json' })
      return downloadBlob(blob, `${base}.racik.json`)
    }
  }
}

/**
 * PPTX: teks diekspor sebagai text box asli PowerPoint (bisa diedit),
 * objek lain (gambar, bentuk, gradien) dirender menjadi gambar per-objek agar tampilan identik.
 */
async function exportPptx(base: string, pages: PageData[], W: number, H: number, scale: number, editableText: boolean) {
  const { default: PptxGenJS } = await import('pptxgenjs')
  const pptx = new PptxGenJS()
  const PPI = 144 // 1920px -> 13.33in (ukuran slide 16:9 standar)
  const inch = (px: number) => px / PPI
  pptx.defineLayout({ name: 'RACIK', width: inch(W), height: inch(H) })
  pptx.layout = 'RACIK'

  for (const p of pages) {
    const slide = pptx.addSlide()
    if (!editableText) {
      // Mode tampilan identik: satu gambar penuh per slide.
      slide.background = { data: await renderPageToDataURL(p, W, H, { format: 'png', multiplier: Math.max(scale, 1) }) }
      continue
    }
    await withPageCanvas(p, W, H, async (c: StaticCanvas) => {
      const objs = c.getObjects()
      const prevVis = objs.map((o) => o.visible)
      // Latar: warna solid langsung, gradien dirender sebagai gambar.
      const bg = c.backgroundColor as any
      if (typeof bg === 'string' && bg) {
        slide.background = { color: toHex(bg) }
      } else if (bg) {
        objs.forEach((o) => (o.visible = false))
        c.renderAll()
        slide.background = { data: c.toDataURL({ format: 'png', multiplier: 1 }) }
        objs.forEach((o, i) => (o.visible = prevVis[i]))
      }
      for (const [i, o] of objs.entries()) {
        if (!prevVis[i]) continue
        const r = o.getBoundingRect()
        if (o instanceof Textbox && !o.shadow && !(o as any).fill?.colorStops) {
          const t = o as Textbox
          const singleLine = (t.textLines?.length || 1) === 1
          slide.addText(t.text || '', {
            // Teks satu baris tidak di-wrap agar tidak patah jika font berbeda di PowerPoint
            wrap: !singleLine, fit: singleLine ? 'none' : 'shrink',
            x: inch(t.left), y: inch(t.top), w: inch(t.width * t.scaleX), h: inch(t.height * t.scaleY),
            fontFace: t.fontFamily, fontSize: (t.fontSize * t.scaleY * 72) / PPI,
            color: toHex(String(t.fill || '#000000')), bold: ['bold', '600', '700', '800', '900'].includes(String(t.fontWeight)),
            italic: t.fontStyle === 'italic', underline: t.underline ? { style: 'sng' } : undefined, strike: t.linethrough ? 'sngStrike' : undefined,
            align: (t.textAlign === 'justify' ? 'justify' : t.textAlign) as any, valign: 'top', margin: 0,
            rotate: t.angle || 0, transparency: Math.round((1 - (t.opacity ?? 1)) * 100),
            lineSpacingMultiple: t.lineHeight, charSpacing: (t.charSpacing / 1000) * ((t.fontSize * t.scaleY * 72) / PPI),
          })
        } else {
          const data = o.toDataURL({ format: 'png', multiplier: o instanceof FabricImage ? Math.max(scale, 1) : 2 })
          slide.addImage({ data, x: inch(r.left), y: inch(r.top), w: inch(r.width), h: inch(r.height) })
        }
      }
    })
  }
  await pptx.writeFile({ fileName: `${base}.pptx` })
}

function toHex(c: string) {
  if (c.startsWith('#')) {
    const h = c.slice(1)
    return (h.length === 3 ? h.split('').map((x) => x + x).join('') : h.slice(0, 6)).toUpperCase()
  }
  const m = c.match(/\d+(\.\d+)?/g)
  if (!m) return '000000'
  return m.slice(0, 3).map((v) => Math.round(+v).toString(16).padStart(2, '0')).join('').toUpperCase()
}
