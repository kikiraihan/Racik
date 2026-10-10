// Konversi "design spec" (format JSON hasil AI & template) <-> objek fabric.js.
import {
  Ellipse, FabricImage, FabricObject, Gradient, Line, Polygon, Rect, StaticCanvas, Textbox, Triangle,
  type Canvas,
} from 'fabric'
import { setupFabric, uid } from './fabric-setup'
import { ensureFonts, fontsInJSON, fontsInSpec } from './fonts'

export interface PageData {
  id: string
  name: string
  fabric: any // JSON canvas fabric (toObject)
  x?: number // posisi frame di board editor (unit desain); kosong = diatur otomatis berjajar
  y?: number
}

export interface DesignData {
  pages: PageData[]
  spec?: any // spec AI/template yang belum dirender (dirender saat editor pertama dibuka)
  generationId?: string
}

export const SERIAL_PROPS = ['id', 'name', 'locked', 'imagePrompt', 'fx', 'frame', 'role', 'selectable', 'evented', 'lockMovementX', 'lockMovementY', 'lockScalingX', 'lockScalingY', 'lockRotation', 'hasControls']

const n = (v: any, d = 0) => (typeof v === 'number' && isFinite(v) ? v : typeof v === 'string' && v.trim() !== '' && isFinite(+v) ? +v : d)

/** Gradien linear berdasarkan sudut ala CSS (0deg = ke atas, 90deg = ke kanan). */
export function linearGradient(colors: string[], angle: number, w: number, h: number, units: 'pixels' | 'percentage' = 'pixels') {
  const a = (angle * Math.PI) / 180
  const dx = Math.sin(a)
  const dy = -Math.cos(a)
  const W = units === 'pixels' ? w : 1
  const H = units === 'pixels' ? h : 1
  const L = (Math.abs(W * dx) + Math.abs(H * dy)) / 2
  const cx = W / 2
  const cy = H / 2
  const stops = colors.length ? colors : ['#ffffff', '#000000']
  return new Gradient({
    type: 'linear',
    gradientUnits: units,
    coords: { x1: cx - dx * L, y1: cy - dy * L, x2: cx + dx * L, y2: cy + dy * L },
    colorStops: stops.map((color, i) => ({ offset: stops.length === 1 ? 0 : i / (stops.length - 1), color })),
  })
}

export function starPoints(w: number, h: number, spikes = 5, inner = 0.45) {
  const pts: { x: number; y: number }[] = []
  for (let i = 0; i < spikes * 2; i++) {
    const r = i % 2 === 0 ? 1 : inner
    const ang = (Math.PI / spikes) * i - Math.PI / 2
    pts.push({ x: (w / 2) * (1 + r * Math.cos(ang)), y: (h / 2) * (1 + r * Math.sin(ang)) })
  }
  return pts
}

export function loadImage(src: string) {
  return FabricImage.fromURL(src, { crossOrigin: 'anonymous' })
}

/** Crop gambar agar menutupi kotak w x h (object-fit: cover). */
export function coverImage(img: FabricImage, w: number, h: number) {
  const el = img.getElement() as HTMLImageElement
  const iw = el.naturalWidth || el.width || img.width
  const ih = el.naturalHeight || el.height || img.height
  const scale = Math.max(w / iw, h / ih)
  const cw = w / scale
  const ch = h / scale
  img.set({ cropX: (iw - cw) / 2, cropY: (ih - ch) / 2, width: cw, height: ch, scaleX: scale, scaleY: scale })
  return img
}

/** Bingkai (frame) gambar: rounded / lingkaran via clipPath. */
export function applyFrame(img: FabricImage, frame: 'rect' | 'circle', radius = 0) {
  const w = img.width
  const h = img.height
  if (frame === 'circle') {
    // Lingkaran sempurna (sisi terpendek), dihitung dalam ruang skala objek
    const r = Math.min(w * (img.scaleX || 1), h * (img.scaleY || 1)) / 2
    img.clipPath = new Ellipse({ rx: r / (img.scaleX || 1), ry: r / (img.scaleY || 1), originX: 'center', originY: 'center' })
  } else if (radius > 0) {
    const r = radius / (img.scaleX || 1)
    img.clipPath = new Rect({ width: w, height: h, rx: r, ry: r, originX: 'center', originY: 'center' })
  } else {
    img.clipPath = undefined
  }
  img.set({ frame: { type: frame, radius } } as any)
  img.dirty = true
}

function placeholderImage(el: any) {
  const w = n(el.w, 300)
  const h = n(el.h, 300)
  const radius = el.shape === 'circle' ? Math.min(w, h) / 2 : n(el.radius)
  return new Rect({
    left: n(el.x), top: n(el.y), width: w, height: h, rx: radius, ry: radius,
    fill: '#E9E3F7', stroke: '#B79CF0', strokeWidth: 2, strokeDashArray: [12, 8],
    id: uid(), name: 'Gambar (placeholder AI)', imagePrompt: el.imagePrompt || '',
  } as any)
}

export async function elementToObject(el: any): Promise<FabricObject | null> {
  const common = { id: uid(), opacity: n(el.opacity, 1), angle: n(el.angle, 0) } as any
  switch (el.type) {
    case 'text': {
      const text = String(el.text ?? '')
      return new Textbox(el.uppercase ? text.toUpperCase() : text, {
        ...common,
        left: n(el.x), top: n(el.y), width: Math.max(n(el.w, 400), 20),
        fontSize: n(el.fontSize, 40), fontFamily: el.fontFamily || 'Inter',
        fontWeight: el.fontWeight || 'normal', fontStyle: el.italic ? 'italic' : 'normal',
        fill: el.color || '#111111', textAlign: el.align || 'left',
        lineHeight: n(el.lineHeight, 1.2), charSpacing: n(el.letterSpacing, 0),
        underline: !!el.underline, role: el.role || '',
        name: el.role ? `Teks (${el.role})` : 'Teks',
      } as any)
    }
    case 'rect':
      return new Rect({
        ...common, left: n(el.x), top: n(el.y), width: n(el.w, 100), height: n(el.h, 100),
        rx: n(el.radius), ry: n(el.radius), fill: el.fill || '#cccccc',
        stroke: el.stroke || undefined, strokeWidth: el.stroke ? n(el.strokeWidth, 1) : 0, name: 'Persegi',
      })
    case 'circle':
      return new Ellipse({ ...common, left: n(el.x), top: n(el.y), rx: n(el.w, 100) / 2, ry: n(el.h, el.w ?? 100) / 2, fill: el.fill || '#cccccc', name: 'Lingkaran' })
    case 'triangle':
      return new Triangle({ ...common, left: n(el.x), top: n(el.y), width: n(el.w, 100), height: n(el.h, 100), fill: el.fill || '#cccccc', name: 'Segitiga' })
    case 'star':
      return new Polygon(starPoints(n(el.w, 100), n(el.h, 100)), { ...common, left: n(el.x), top: n(el.y), fill: el.fill || '#cccccc', name: 'Bintang' })
    case 'line':
      return new Line([n(el.x), n(el.y), n(el.x) + n(el.w, 100), n(el.y) + n(el.h, 0)], {
        ...common, stroke: el.stroke || el.fill || '#111111', strokeWidth: n(el.strokeWidth, 4), name: 'Garis',
      })
    case 'image': {
      if (!el.src) return placeholderImage(el)
      try {
        const img = await loadImage(el.src)
        const w = n(el.w, img.width)
        const h = n(el.h, img.height)
        coverImage(img, w, h)
        img.set({ ...common, left: n(el.x), top: n(el.y), name: 'Gambar', imagePrompt: el.imagePrompt || '' } as any)
        if (el.shape === 'circle') applyFrame(img, 'circle')
        else if (n(el.radius) > 0) applyFrame(img, 'rect', n(el.radius))
        return img
      } catch {
        return placeholderImage(el)
      }
    }
  }
  return null
}

/** Render satu halaman spec ke canvas (dipakai juga untuk StaticCanvas offscreen). */
export async function renderSpecPage(canvas: Canvas | StaticCanvas, page: any, W: number, H: number) {
  await ensureFonts(fontsInSpec({ pages: [page] }))
  canvas.clear()
  const bg = page?.background || { type: 'color', color: '#ffffff' }
  canvas.backgroundColor = '#ffffff'
  if (bg.type === 'gradient' && Array.isArray(bg.colors)) {
    canvas.backgroundColor = linearGradient(bg.colors, n(bg.angle, 135), W, H) as any
  } else if (bg.type === 'color' && bg.color) {
    canvas.backgroundColor = bg.color
  } else if (bg.type === 'image') {
    canvas.backgroundColor = bg.color || '#1f1f1f'
    if (bg.src) {
      try {
        const img = coverImage(await loadImage(bg.src), W, H)
        img.set({ left: 0, top: 0, id: uid(), name: 'Gambar latar', imagePrompt: bg.imagePrompt || '' } as any)
        canvas.add(img)
      } catch {}
    }
    if (bg.overlay) {
      canvas.add(new Rect({ left: 0, top: 0, width: W, height: H, fill: bg.overlay, opacity: n(bg.overlayOpacity, 0.4), id: uid(), name: 'Overlay' } as any))
    }
  }
  for (const el of page?.elements || []) {
    const obj = await elementToObject(el)
    if (obj) canvas.add(obj)
  }
  canvas.requestRenderAll?.()
}

let offscreen: StaticCanvas | null = null
let queue: Promise<unknown> = Promise.resolve()

/** Canvas offscreen dipakai bergantian; semua akses diantrikan agar tidak saling timpa. */
function locked<T>(fn: () => Promise<T>): Promise<T> {
  const run = queue.then(fn, fn)
  queue = run.catch(() => {})
  return run
}
function getOffscreen(W: number, H: number) {
  setupFabric()
  if (!offscreen) offscreen = new StaticCanvas(document.createElement('canvas'), { renderOnAddRemove: false })
  offscreen.setDimensions({ width: W, height: H })
  return offscreen
}

/** Ubah seluruh spec menjadi data halaman fabric JSON. */
export function specToPages(spec: any, W: number, H: number): Promise<PageData[]> {
  return locked(async () => {
    const c = getOffscreen(W, H)
    const pages: PageData[] = []
    for (const [i, p] of (spec?.pages || []).entries()) {
      await renderSpecPage(c, p, W, H)
      pages.push({ id: uid('p'), name: p.name || `Halaman ${i + 1}`, fabric: c.toObject(SERIAL_PROPS) })
    }
    return pages
  })
}

/** Render JSON halaman ke data URL (untuk thumbnail & export). */
export function renderPageToDataURL(page: PageData, W: number, H: number, opts: { multiplier?: number; format?: 'png' | 'jpeg'; quality?: number } = {}) {
  return withPageCanvas(page, W, H, (c) => c.toDataURL({ format: opts.format || 'png', quality: opts.quality ?? 0.92, multiplier: opts.multiplier ?? 1 }))
}

export function withPageCanvas<T>(page: PageData, W: number, H: number, fn: (c: StaticCanvas) => Promise<T> | T): Promise<T> {
  return locked(async () => {
    const c = getOffscreen(W, H)
    await ensureFonts(fontsInJSON(page.fabric))
    await c.loadFromJSON(page.fabric)
    c.renderAll()
    return fn(c)
  })
}

/** Preview template/spec langsung ke gambar (halaman pertama). */
export async function specPreview(spec: any, W: number, H: number, width = 320) {
  const pages = await specToPages({ pages: (spec?.pages || []).slice(0, 1) }, W, H)
  if (!pages.length) return ''
  return renderPageToDataURL(pages[0], W, H, { format: 'jpeg', quality: 0.8, multiplier: width / W })
}

// ---------- fabric -> spec (dipakai fitur AI "perbaiki desain") ----------

const hex = (v: any) => (typeof v === 'string' ? v : undefined)

export function objectsToSpecPage(canvas: Canvas | StaticCanvas, name: string) {
  const bg = canvas.backgroundColor as any
  let background: any = { type: 'color', color: typeof bg === 'string' ? bg : '#ffffff' }
  if (bg && typeof bg === 'object' && bg.colorStops) {
    background = { type: 'gradient', colors: bg.colorStops.map((s: any) => s.color), angle: 135 }
  }
  const elements: any[] = []
  for (const o of canvas.getObjects() as any[]) {
    if (!o.visible) continue
    const r = o.getBoundingRect()
    const base = { x: Math.round(r.left), y: Math.round(r.top), w: Math.round(r.width), h: Math.round(r.height), opacity: o.opacity }
    if (o instanceof Textbox) {
      elements.push({
        type: 'text', text: o.text, x: Math.round(o.left), y: Math.round(o.top), w: Math.round(o.width * o.scaleX),
        fontSize: Math.round(o.fontSize * o.scaleY), fontFamily: o.fontFamily, fontWeight: String(o.fontWeight),
        italic: o.fontStyle === 'italic', color: hex(o.fill) || '#000000', align: o.textAlign,
        lineHeight: o.lineHeight, letterSpacing: o.charSpacing, role: o.role || undefined,
      })
    } else if (o instanceof FabricImage) {
      elements.push({ type: 'image', ...base, src: o.getSrc(), imagePrompt: o.imagePrompt || undefined, shape: o.frame?.type === 'circle' ? 'circle' : 'rect', radius: o.frame?.radius || 0 })
    } else if (o.imagePrompt) {
      elements.push({ type: 'image', ...base, imagePrompt: o.imagePrompt })
    } else if (o instanceof Rect) {
      elements.push({ type: 'rect', ...base, fill: hex(o.fill), radius: o.rx, stroke: hex(o.stroke), strokeWidth: o.strokeWidth })
    } else if (o instanceof Ellipse) {
      elements.push({ type: 'circle', ...base, fill: hex(o.fill) })
    } else if (o instanceof Triangle) {
      elements.push({ type: 'triangle', ...base, fill: hex(o.fill) })
    } else if (o instanceof Polygon) {
      elements.push({ type: 'star', ...base, fill: hex(o.fill) })
    } else if (o instanceof Line) {
      elements.push({ type: 'line', x: base.x, y: base.y, w: base.w, stroke: hex(o.stroke), strokeWidth: o.strokeWidth })
    }
  }
  return { name, background, elements }
}

/** Skalakan spec (mis. template) ke ukuran halaman lain secara proporsional & terpusat. */
export function scaleSpec(spec: any, fromW: number, fromH: number, toW: number, toH: number) {
  if (fromW === toW && fromH === toH) return spec
  const s = Math.min(toW / fromW, toH / fromH)
  const ox = (toW - fromW * s) / 2
  const oy = (toH - fromH * s) / 2
  const out = structuredClone(spec)
  for (const p of out.pages || []) {
    for (const el of p.elements || []) {
      if (typeof el.x === 'number') el.x = el.x * s + ox
      if (typeof el.y === 'number') el.y = el.y * s + oy
      for (const k of ['w', 'h', 'fontSize', 'radius', 'strokeWidth']) if (typeof el[k] === 'number') el[k] *= s
    }
  }
  return out
}
