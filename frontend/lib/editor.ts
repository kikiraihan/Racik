// Inti editor Racik: membungkus fabric.Canvas dengan state reaktif untuk UI Vue.
// - Multi halaman (setiap halaman disimpan sebagai JSON fabric)
// - Layer, undo/redo per halaman, clipboard, grup, align, snapping guide
// - Filter gambar, frame, gradien, background, mode gambar bebas
import {
  ActiveSelection, Canvas, Canvas2dFilterBackend, Ellipse, FabricImage, FabricObject, Group, IText, Line, Path,
  PencilBrush, Polygon, Rect, Shadow, Textbox, Triangle, filters, loadSVGFromString, setFilterBackend, util,
} from 'fabric'
import { markRaw, reactive } from 'vue'
import { ACCENT, setupFabric, uid } from './fabric-setup'
import { ensureFonts, fontsInJSON } from './fonts'
import {
  SERIAL_PROPS, applyFrame, coverImage, linearGradient, loadImage, objectsToSpecPage, renderPageToDataURL, renderSpecPage,
  specToPages, starPoints, type DesignData, type PageData,
} from './spec'

export type SelKind = 'none' | 'text' | 'image' | 'shape' | 'line' | 'group' | 'multi' | 'path' | 'placeholder'

export interface Fx {
  brightness: number
  contrast: number
  saturation: number
  blur: number
  hue: number
  noise: number
  pixelate: number
  grayscale: boolean
  sepia: boolean
  invert: boolean
  vintage: boolean
}

export const DEFAULT_FX: Fx = { brightness: 0, contrast: 0, saturation: 0, blur: 0, hue: 0, noise: 0, pixelate: 0, grayscale: false, sepia: false, invert: false, vintage: false }

export interface LayerItem {
  id: string
  name: string
  type: string
  visible: boolean
  locked: boolean
  selected: boolean
}

export interface SelInfo {
  kind: SelKind
  count: number
  locked: boolean
  left: number
  top: number
  width: number
  height: number
  angle: number
  opacity: number
  fill: string
  fillGradient: string[] | null
  stroke: string
  strokeWidth: number
  rx: number
  // teks
  text: string
  fontFamily: string
  fontSize: number
  fontWeight: string
  fontStyle: string
  underline: boolean
  linethrough: boolean
  textAlign: string
  lineHeight: number
  charSpacing: number
  textBackgroundColor: string
  shadow: { color: string; blur: number; offsetX: number; offsetY: number } | null
  // gambar
  fx: Fx
  flipX: boolean
  flipY: boolean
  frame: { type: 'rect' | 'circle'; radius: number }
  imagePrompt: string
  src: string
}

const emptySel = (): SelInfo => ({
  kind: 'none', count: 0, locked: false, left: 0, top: 0, width: 0, height: 0, angle: 0, opacity: 1,
  fill: '', fillGradient: null, stroke: '', strokeWidth: 0, rx: 0,
  text: '', fontFamily: 'Inter', fontSize: 32, fontWeight: 'normal', fontStyle: 'normal', underline: false, linethrough: false,
  textAlign: 'left', lineHeight: 1.2, charSpacing: 0, textBackgroundColor: '', shadow: null,
  fx: { ...DEFAULT_FX }, flipX: false, flipY: false, frame: { type: 'rect', radius: 0 }, imagePrompt: '', src: '',
})

export const TEXT_PRESETS = {
  heading: { text: 'Tambahkan judul', fontSize: 96, fontWeight: '800', fontFamily: 'Montserrat' },
  subheading: { text: 'Tambahkan subjudul', fontSize: 56, fontWeight: '600', fontFamily: 'Poppins' },
  body: { text: 'Tambahkan sedikit teks isi', fontSize: 32, fontWeight: 'normal', fontFamily: 'Inter' },
} as const

const HISTORY_LIMIT = 60
const SNAP = 6

export class Editor {
  canvas!: Canvas
  pages: PageData[] = []
  width = 1080
  height = 1080
  private history = new Map<string, { undo: string[]; redo: string[] }>()
  private loading = false
  private clipboard: FabricObject | null = null
  private guides: { v: number[]; h: number[] } = { v: [], h: [] }
  private histTimer: any = null
  private selRaf = 0
  onChange: () => void = () => {}

  state = reactive({
    width: 1080,
    height: 1080,
    zoom: 0.5,
    pageIndex: 0,
    pages: [] as { id: string; name: string; thumb: string; x?: number; y?: number }[],
    layers: [] as LayerItem[],
    sel: emptySel(),
    canUndo: false,
    canRedo: false,
    drawing: false,
    brushColor: '#8B3DFF',
    brushWidth: 8,
    background: '#ffffff' as string,
    backgroundGradient: null as string[] | null,
  })

  init(el: HTMLCanvasElement) {
    setupFabric()
    setFilterBackend(new Canvas2dFilterBackend())
    this.canvas = markRaw(new Canvas(el, {
      preserveObjectStacking: true,
      selectionColor: 'rgba(37,99,235,0.08)',
      selectionBorderColor: ACCENT,
      selectionLineWidth: 1,
      controlsAboveOverlay: true,
      stopContextMenu: true,
      fireRightClick: true,
      enableRetinaScaling: true,
    }))
    const c = this.canvas
    // Tambah/hapus objek dicatat per aksi; perubahan beruntun (slider, mengetik) digabung.
    const structural = () => this.changed(0)
    const changed = () => this.changed(250)
    c.on('object:added', structural)
    c.on('object:removed', structural)
    c.on('object:modified', changed)
    c.on('text:changed', changed)
    c.on('path:created', (e: any) => {
      e.path.set({ id: uid(), name: 'Gambar bebas' })
    })
    c.on('selection:created', () => this.refreshSel())
    c.on('selection:updated', () => this.refreshSel())
    c.on('selection:cleared', () => this.refreshSel())
    c.on('object:scaling', () => this.refreshSelSoon())
    c.on('object:rotating', () => this.refreshSelSoon())
    c.on('object:moving', (e) => {
      this.snap(e.target)
      this.refreshSelSoon()
    })
    c.on('mouse:up', () => {
      if (this.guides.v.length || this.guides.h.length) {
        this.guides = { v: [], h: [] }
        c.requestRenderAll()
      }
    })
    c.on('after:render', () => this.drawGuides())
  }

  dispose() {
    this.canvas?.dispose()
  }

  // ---------- load / save ----------

  async load(data: DesignData, W: number, H: number) {
    this.width = this.state.width = W
    this.height = this.state.height = H
    if ((!data.pages || data.pages.length === 0) && data.spec) {
      data.pages = await specToPages(data.spec, W, H)
    }
    this.pages = data.pages?.length ? data.pages : [this.blankPage(1)]
    this.history.clear()
    await this.showPage(0)
    this.refreshThumbs()
  }

  blankPage(i: number): PageData {
    return { id: uid('p'), name: `Halaman ${i}`, fabric: { version: '6', objects: [], background: '#ffffff' } }
  }

  /** Simpan state canvas aktif ke array pages. */
  commitPage() {
    const p = this.pages[this.state.pageIndex]
    if (p && this.canvas) p.fabric = this.canvas.toObject(SERIAL_PROPS)
  }

  serialize(): DesignData {
    this.commitPage()
    return { pages: this.pages }
  }

  async thumbnail(width = 360) {
    this.commitPage()
    return renderPageToDataURL(this.pages[0], this.width, this.height, { format: 'jpeg', quality: 0.75, multiplier: width / this.width })
  }

  private async showPage(i: number) {
    const c = this.canvas
    this.loading = true
    c.discardActiveObject()
    await ensureFonts(fontsInJSON(this.pages[i].fabric))
    await c.loadFromJSON(this.pages[i].fabric)
    c.getObjects().forEach((o: any) => {
      if (!o.id) o.id = uid()
    })
    this.state.pageIndex = i
    this.loading = false
    this.applyZoom()
    c.requestRenderAll()
    this.readBackground()
    const h = this.hist()
    if (h.undo.length === 0) h.undo.push(this.snapshot())
    this.syncHistoryFlags()
    this.refreshLayers()
    this.refreshSel()
  }

  // ---------- halaman ----------

  async goTo(i: number) {
    if (i === this.state.pageIndex || i < 0 || i >= this.pages.length) return
    this.commitPage()
    this.updateThumb(this.state.pageIndex)
    await this.showPage(i)
  }

  async addPage(after = this.state.pageIndex) {
    this.commitPage()
    const p = this.blankPage(this.pages.length + 1)
    const bg = this.canvas.backgroundColor
    if (typeof bg === 'string') p.fabric.background = bg
    this.pages.splice(after + 1, 0, p)
    this.refreshThumbs()
    await this.showPage(after + 1)
    this.onChange()
  }

  async duplicatePage(i = this.state.pageIndex) {
    this.commitPage()
    const copy: PageData = { id: uid('p'), name: this.pages[i].name + ' (salinan)', fabric: structuredClone(this.pages[i].fabric) }
    this.pages.splice(i + 1, 0, copy)
    this.refreshThumbs()
    await this.showPage(i + 1)
    this.onChange()
  }

  async deletePage(i = this.state.pageIndex) {
    if (this.pages.length <= 1) return
    this.commitPage()
    this.history.delete(this.pages[i].id)
    this.pages.splice(i, 1)
    this.refreshThumbs()
    await this.showPage(Math.min(i, this.pages.length - 1))
    this.onChange()
  }

  async movePage(i: number, dir: -1 | 1) {
    const j = i + dir
    if (j < 0 || j >= this.pages.length) return
    this.commitPage()
    ;[this.pages[i], this.pages[j]] = [this.pages[j], this.pages[i]]
    this.refreshThumbs()
    await this.showPage(j)
    this.onChange()
  }

  renamePage(i: number, name: string) {
    this.pages[i].name = name
    this.state.pages[i].name = name
    this.onChange()
  }

  /** Pindahkan frame halaman di board (unit desain). */
  setPagePos(i: number, x: number, y: number) {
    const p = this.pages[i]
    if (!p) return
    p.x = Math.round(x)
    p.y = Math.round(y)
    const item = this.state.pages.find((s) => s.id === p.id)
    if (item) Object.assign(item, { x: p.x, y: p.y })
    this.onChange()
  }

  async updateThumb(i: number) {
    const p = this.pages[i]
    if (!p) return
    // Thumbnail juga dipakai sebagai pratinjau frame halaman non-aktif di board, jadi dibuat lebih tajam.
    const url = await renderPageToDataURL(p, this.width, this.height, { format: 'jpeg', quality: 0.8, multiplier: Math.min(1, 720 / this.width) })
    const item = this.state.pages.find((x) => x.id === p.id)
    if (item) item.thumb = url
  }

  async refreshThumbs() {
    const old = new Map(this.state.pages.map((p) => [p.id, p.thumb]))
    this.state.pages = this.pages.map((p) => ({ id: p.id, name: p.name, thumb: old.get(p.id) || '', x: p.x, y: p.y }))
    for (let i = 0; i < this.pages.length; i++) await this.updateThumb(i)
  }

  // ---------- history ----------

  private hist() {
    const id = this.pages[this.state.pageIndex]?.id || 'x'
    if (!this.history.has(id)) this.history.set(id, { undo: [], redo: [] })
    return this.history.get(id)!
  }

  private snapshot() {
    return JSON.stringify(this.canvas.toObject(SERIAL_PROPS))
  }

  private changed(delay = 250) {
    if (this.loading) return
    clearTimeout(this.histTimer)
    this.histTimer = setTimeout(() => {
      const h = this.hist()
      const snap = this.snapshot()
      if (h.undo[h.undo.length - 1] !== snap) {
        h.undo.push(snap)
        if (h.undo.length > HISTORY_LIMIT) h.undo.shift()
        h.redo = []
      }
      this.syncHistoryFlags()
      this.refreshLayers()
      this.refreshSel()
      this.commitPage()
      this.updateThumb(this.state.pageIndex)
      this.onChange()
    }, delay)
  }

  private syncHistoryFlags() {
    const h = this.hist()
    this.state.canUndo = h.undo.length > 1
    this.state.canRedo = h.redo.length > 0
  }

  private async restore(snap: string) {
    this.loading = true
    this.canvas.discardActiveObject()
    const json = JSON.parse(snap)
    await ensureFonts(fontsInJSON(json))
    await this.canvas.loadFromJSON(json)
    this.loading = false
    this.canvas.requestRenderAll()
    this.readBackground()
    this.syncHistoryFlags()
    this.refreshLayers()
    this.refreshSel()
    this.commitPage()
    this.updateThumb(this.state.pageIndex)
    this.onChange()
  }

  async undo() {
    const h = this.hist()
    if (h.undo.length <= 1) return
    h.redo.push(h.undo.pop()!)
    await this.restore(h.undo[h.undo.length - 1])
  }

  async redo() {
    const h = this.hist()
    const s = h.redo.pop()
    if (!s) return
    h.undo.push(s)
    await this.restore(s)
  }

  // ---------- zoom ----------

  setZoom(z: number) {
    this.state.zoom = Math.min(Math.max(z, 0.05), 5)
    this.applyZoom()
  }

  zoomFit(container: HTMLElement) {
    const pad = 64
    const z = Math.min((container.clientWidth - pad) / this.width, (container.clientHeight - pad) / this.height)
    this.setZoom(z)
  }

  private applyZoom() {
    const z = this.state.zoom
    this.canvas.setDimensions({ width: Math.round(this.width * z), height: Math.round(this.height * z) })
    this.canvas.setZoom(z)
    this.canvas.requestRenderAll()
  }

  resize(W: number, H: number, scaleContent: boolean) {
    this.commitPage()
    const sx = W / this.width
    const sy = H / this.height
    const s = Math.min(sx, sy)
    if (scaleContent) {
      for (const p of this.pages) {
        for (const o of p.fabric.objects || []) {
          o.left = o.left * s + (W - this.width * s) / 2
          o.top = o.top * s + (H - this.height * s) / 2
          o.scaleX = (o.scaleX || 1) * s
          o.scaleY = (o.scaleY || 1) * s
        }
      }
    }
    this.width = this.state.width = W
    this.height = this.state.height = H
    this.history.clear()
    return this.showPage(this.state.pageIndex).then(() => {
      this.refreshThumbs()
      this.onChange()
    })
  }

  // ---------- snapping guides ----------

  private snap(obj?: FabricObject) {
    if (!obj) return
    const z = this.state.zoom
    const t = SNAP / z
    const r = obj.getBoundingRect()
    const v: number[] = []
    const h: number[] = []
    const xs = [0, this.width / 2, this.width]
    const ys = [0, this.height / 2, this.height]
    for (const o of this.canvas.getObjects()) {
      if (o === obj || !o.visible || (obj instanceof ActiveSelection && obj.contains(o))) continue
      const b = o.getBoundingRect()
      xs.push(b.left, b.left + b.width / 2, b.left + b.width)
      ys.push(b.top, b.top + b.height / 2, b.top + b.height)
    }
    const cand = (edges: number[], targets: number[], out: number[]) => {
      for (const e of edges) {
        for (const tg of targets) {
          if (Math.abs(e - tg) < t) {
            out.push(tg)
            return tg - e
          }
        }
      }
      return 0
    }
    const dx = cand([r.left + r.width / 2, r.left, r.left + r.width], xs, v)
    const dy = cand([r.top + r.height / 2, r.top, r.top + r.height], ys, h)
    if (dx || dy) obj.set({ left: obj.left + dx, top: obj.top + dy })
    this.guides = { v, h }
  }

  private drawGuides() {
    if (!this.canvas || (!this.guides.v.length && !this.guides.h.length)) return
    const ctx = this.canvas.getContext()
    const z = this.state.zoom
    ctx.save()
    ctx.strokeStyle = '#FF3EA5'
    ctx.lineWidth = 1
    ctx.setLineDash([4, 3])
    for (const x of this.guides.v) {
      ctx.beginPath()
      ctx.moveTo(x * z + 0.5, 0)
      ctx.lineTo(x * z + 0.5, this.height * z)
      ctx.stroke()
    }
    for (const y of this.guides.h) {
      ctx.beginPath()
      ctx.moveTo(0, y * z + 0.5)
      ctx.lineTo(this.width * z, y * z + 0.5)
      ctx.stroke()
    }
    ctx.restore()
  }

  // ---------- menambah objek ----------

  private place(obj: FabricObject, center = true) {
    if (!(obj as any).id) obj.set({ id: uid() } as any)
    if (center) {
      const r = obj.getBoundingRect()
      obj.set({ left: (this.width - r.width) / 2 + (obj.left - r.left), top: (this.height - r.height) / 2 + (obj.top - r.top) })
    }
    this.canvas.add(obj)
    obj.setCoords()
    this.canvas.setActiveObject(obj)
    this.canvas.requestRenderAll()
    return obj
  }

  private unit() {
    return Math.min(this.width, this.height)
  }

  async addText(preset: keyof typeof TEXT_PRESETS | { text: string; fontSize: number; fontWeight?: string; fontFamily?: string; fill?: string }, extra: any = {}) {
    const p = typeof preset === 'string' ? TEXT_PRESETS[preset] : preset
    await ensureFonts([p.fontFamily || 'Inter'])
    const k = this.unit() / 1080
    const t = new Textbox(p.text, {
      fontSize: Math.round(p.fontSize * k), fontWeight: p.fontWeight || 'normal', fontFamily: p.fontFamily || 'Inter',
      fill: (p as any).fill || '#111111', width: Math.min(this.width * 0.8, Math.max(p.text.length * p.fontSize * k * 0.6, 200)),
      textAlign: 'center', lineHeight: 1.2, name: 'Teks', ...extra,
    } as any)
    return this.place(t)
  }

  /** Kombinasi font siap pakai (judul + subjudul). */
  async addTextCombo(heading: string, sub: string, hf: string, sf: string) {
    await ensureFonts([hf, sf])
    const k = this.unit() / 1080
    const a = new Textbox(heading, { fontFamily: hf, fontSize: 84 * k, fontWeight: 'bold', width: 700 * k, textAlign: 'center', fill: '#111111', name: 'Judul' } as any)
    const b = new Textbox(sub, { fontFamily: sf, fontSize: 30 * k, width: 700 * k, textAlign: 'center', top: a.height + 16 * k, fill: '#444444', charSpacing: 200, name: 'Subjudul' } as any)
    const g = new Group([a, b], { name: 'Kombinasi teks' } as any)
    return this.place(g)
  }

  addShape(kind: string, color = '#8B3DFF') {
    const s = this.unit() * 0.3
    let o: FabricObject
    switch (kind) {
      case 'rect': o = new Rect({ width: s, height: s, fill: color, name: 'Persegi' } as any); break
      case 'rounded': o = new Rect({ width: s, height: s, rx: s * 0.18, ry: s * 0.18, fill: color, name: 'Persegi tumpul' } as any); break
      case 'circle': o = new Ellipse({ rx: s / 2, ry: s / 2, fill: color, name: 'Lingkaran' } as any); break
      case 'triangle': o = new Triangle({ width: s, height: s, fill: color, name: 'Segitiga' } as any); break
      case 'star': o = new Polygon(starPoints(s, s), { fill: color, name: 'Bintang' } as any); break
      case 'pentagon': o = new Polygon(regularPolygon(5, s), { fill: color, name: 'Pentagon' } as any); break
      case 'hexagon': o = new Polygon(regularPolygon(6, s), { fill: color, name: 'Heksagon' } as any); break
      case 'diamond': o = new Polygon([{ x: s / 2, y: 0 }, { x: s, y: s / 2 }, { x: s / 2, y: s }, { x: 0, y: s / 2 }], { fill: color, name: 'Belah ketupat' } as any); break
      case 'heart': o = new Path('M 50 90 C 20 70 0 50 0 28 C 0 12 12 0 27 0 C 38 0 46 6 50 15 C 54 6 62 0 73 0 C 88 0 100 12 100 28 C 100 50 80 70 50 90 Z', { fill: color, scaleX: s / 100, scaleY: s / 100, name: 'Hati' } as any); break
      case 'arrow': o = new Path('M 0 35 L 60 35 L 60 10 L 100 50 L 60 90 L 60 65 L 0 65 Z', { fill: color, scaleX: s / 100, scaleY: s / 100, name: 'Panah' } as any); break
      case 'speech': o = new Path('M 10 0 H 90 Q 100 0 100 10 V 60 Q 100 70 90 70 H 40 L 20 92 L 24 70 H 10 Q 0 70 0 60 V 10 Q 0 0 10 0 Z', { fill: color, scaleX: s / 100, scaleY: s / 100, name: 'Balon kata' } as any); break
      case 'line': o = new Line([0, 0, s * 1.5, 0], { stroke: '#111111', strokeWidth: Math.max(4, s / 40), name: 'Garis' } as any); break
      case 'dashed': o = new Line([0, 0, s * 1.5, 0], { stroke: '#111111', strokeWidth: Math.max(4, s / 40), strokeDashArray: [20, 12], name: 'Garis putus' } as any); break
      case 'line-arrow': o = new Path(`M 0 0 L ${s * 1.4} 0 M ${s * 1.4 - s * 0.15} ${-s * 0.1} L ${s * 1.4} 0 L ${s * 1.4 - s * 0.15} ${s * 0.1}`, { stroke: '#111111', strokeWidth: Math.max(4, s / 40), fill: '', strokeLineCap: 'round', strokeLineJoin: 'round', name: 'Garis panah' } as any); break
      default: return
    }
    return this.place(o)
  }

  /** Bingkai foto kosong: isi dengan upload / gambar AI (pilih bingkai lalu klik gambar). */
  addFrame(kind: 'rect' | 'rounded' | 'circle') {
    const s = this.unit() * 0.4
    const common = { fill: '#E4E4E7', stroke: '#A1A1AA', strokeWidth: 2, strokeDashArray: [12, 8], imagePrompt: 'Foto', name: 'Bingkai foto' } as any
    const o = kind === 'circle'
      ? new Ellipse({ rx: s / 2, ry: s / 2, ...common })
      : new Rect({ width: s, height: s * (kind === 'rect' ? 0.75 : 1), rx: kind === 'rounded' ? s * 0.12 : 0, ry: kind === 'rounded' ? s * 0.12 : 0, ...common })
    return this.place(o)
  }

  addEmoji(ch: string) {
    const t = new IText(ch, { fontSize: this.unit() * 0.2, fontFamily: 'sans-serif', name: 'Emoji ' + ch } as any)
    return this.place(t)
  }

  async addSvg(svg: string, name = 'Ikon', color?: string) {
    const { objects, options } = await loadSVGFromString(svg)
    const objs = objects.filter(Boolean) as FabricObject[]
    if (color) objs.forEach((o) => { if (o.fill && o.fill !== 'none') o.set('fill', color); if (o.stroke && o.stroke !== 'none') o.set('stroke', color) })
    const g = util.groupSVGElements(objs, options)
    const s = (this.unit() * 0.25) / Math.max(g.width, g.height)
    g.set({ scaleX: s, scaleY: s, name } as any)
    return this.place(g)
  }

  async addImage(url: string, opts: { name?: string; imagePrompt?: string } = {}) {
    const img = await loadImage(url)
    const max = this.unit() * 0.6
    const s = Math.min(1, max / Math.max(img.width, img.height))
    img.set({ scaleX: s, scaleY: s, name: opts.name || 'Gambar', imagePrompt: opts.imagePrompt || '' } as any)
    return this.place(img)
  }

  /** Ganti gambar/placeholder yang terseleksi dengan gambar baru (ukuran & posisi dipertahankan). */
  async replaceActiveImage(url: string, imagePrompt = '') {
    const o = this.canvas.getActiveObject() as any
    if (!o) return this.addImage(url, { imagePrompt })
    const r = o.getBoundingRect()
    const img = coverImage(await loadImage(url), r.width, r.height)
    const idx = this.canvas.getObjects().indexOf(o)
    img.set({ left: r.left, top: r.top, id: o.id || uid(), name: 'Gambar', imagePrompt: imagePrompt || o.imagePrompt || '' } as any)
    if (o.frame?.type === 'circle' || (o instanceof Ellipse)) applyFrame(img, 'circle')
    else if ((o.frame?.radius || o.rx) > 0) applyFrame(img, 'rect', o.frame?.radius || o.rx)
    this.canvas.remove(o)
    this.canvas.insertAt(idx, img)
    this.canvas.setActiveObject(img)
    this.canvas.requestRenderAll()
  }

  async setBackgroundImage(url: string) {
    const img = coverImage(await loadImage(url), this.width, this.height)
    img.set({ left: 0, top: 0, name: 'Gambar latar', id: uid(), lockMovementX: true, lockMovementY: true, locked: true, hasControls: false } as any)
    const old = this.canvas.getObjects().find((o: any) => o.name === 'Gambar latar')
    if (old) this.canvas.remove(old)
    this.canvas.insertAt(0, img)
    this.canvas.requestRenderAll()
  }

  /** Jadikan gambar terseleksi sebagai latar halaman. */
  async activeToBackground() {
    const o = this.canvas.getActiveObject()
    if (!(o instanceof FabricImage)) return
    const src = o.getSrc()
    this.canvas.remove(o)
    await this.setBackgroundImage(src)
  }

  setBackgroundColor(color: string) {
    this.canvas.backgroundColor = color
    this.canvas.requestRenderAll()
    this.readBackground()
    this.canvas.fire('object:modified', {} as any)
  }

  setBackgroundGradient(colors: string[], angle = 135) {
    this.canvas.backgroundColor = linearGradient(colors, angle, this.width, this.height) as any
    this.canvas.requestRenderAll()
    this.readBackground()
    this.canvas.fire('object:modified', {} as any)
  }

  private readBackground() {
    const bg = this.canvas.backgroundColor as any
    if (bg && typeof bg === 'object' && bg.colorStops) {
      this.state.backgroundGradient = bg.colorStops.map((s: any) => s.color)
      this.state.background = bg.colorStops[0]?.color || '#ffffff'
    } else {
      this.state.backgroundGradient = null
      this.state.background = typeof bg === 'string' && bg ? bg : '#ffffff'
    }
  }

  // ---------- seleksi & properti ----------

  active() {
    return this.canvas?.getActiveObject() || null
  }

  private refreshSelSoon() {
    cancelAnimationFrame(this.selRaf)
    this.selRaf = requestAnimationFrame(() => this.refreshSel())
  }

  refreshSel() {
    const o = this.active() as any
    const s = emptySel()
    if (!o) {
      this.state.sel = s
      this.markLayerSelection()
      return
    }
    const r = o.getBoundingRect()
    Object.assign(s, {
      count: o instanceof ActiveSelection ? o.size() : 1,
      locked: !!o.locked,
      left: Math.round(o.left), top: Math.round(o.top),
      width: Math.round(o.getScaledWidth?.() ?? r.width), height: Math.round(o.getScaledHeight?.() ?? r.height),
      angle: Math.round(o.angle || 0), opacity: o.opacity ?? 1,
      stroke: typeof o.stroke === 'string' ? o.stroke : '', strokeWidth: o.strokeWidth || 0,
      flipX: !!o.flipX, flipY: !!o.flipY,
      shadow: o.shadow ? { color: o.shadow.color, blur: o.shadow.blur, offsetX: o.shadow.offsetX, offsetY: o.shadow.offsetY } : null,
      imagePrompt: o.imagePrompt || '',
    })
    if (o.fill && typeof o.fill === 'object' && o.fill.colorStops) {
      s.fillGradient = o.fill.colorStops.map((c: any) => c.color)
      s.fill = s.fillGradient![0]
    } else s.fill = typeof o.fill === 'string' ? o.fill : ''

    if (o instanceof ActiveSelection) s.kind = 'multi'
    else if (o instanceof Textbox || o instanceof IText) {
      s.kind = 'text'
      Object.assign(s, {
        text: o.text, fontFamily: o.fontFamily, fontSize: Math.round(o.fontSize * (o.scaleY || 1)), fontWeight: String(o.fontWeight),
        fontStyle: o.fontStyle, underline: o.underline, linethrough: o.linethrough, textAlign: o.textAlign,
        lineHeight: o.lineHeight, charSpacing: o.charSpacing, textBackgroundColor: o.textBackgroundColor || '',
      })
    } else if (o instanceof FabricImage) {
      s.kind = 'image'
      s.fx = { ...DEFAULT_FX, ...(o.fx || {}) }
      s.frame = o.frame || { type: 'rect', radius: 0 }
      s.src = o.getSrc()
    } else if (o.imagePrompt) s.kind = 'placeholder'
    else if (o instanceof Line) s.kind = 'line'
    else if (o instanceof Group) s.kind = 'group'
    else if (o instanceof Path && !o.fill) s.kind = 'path'
    else {
      s.kind = 'shape'
      s.rx = o.rx || 0
    }
    this.state.sel = s
    this.markLayerSelection()
  }

  /** Set properti pada objek terseleksi (atau semua objek dalam multi-seleksi). */
  async setFont(fontFamily: string) {
    await ensureFonts([fontFamily])
    this.set({ fontFamily })
  }

  set(props: Record<string, any>) {
    const o = this.active() as any
    if (!o) return
    const targets: any[] = o instanceof ActiveSelection ? o.getObjects() : [o]
    for (const t of targets) {
      const p = { ...props }
      if ('fontSize' in p && t.scaleY && t.scaleY !== 1 && t instanceof Textbox) {
        // normalisasi skala teks agar fontSize sesuai yang terlihat
        t.set({ width: t.width * t.scaleX, scaleX: 1, scaleY: 1 })
      }
      if ('rx' in p) p.ry = p.rx
      if (t instanceof Group && !(t instanceof ActiveSelection) && ('fill' in p)) {
        t.getObjects().forEach((c: any) => c.set({ fill: p.fill }))
        delete p.fill
      }
      if (t instanceof Line && 'fill' in p) {
        p.stroke = p.fill
        delete p.fill
      }
      t.set(p)
      if (t instanceof Textbox) t.initDimensions()
      t.setCoords()
    }
    o.setCoords?.()
    this.canvas.requestRenderAll()
    this.canvas.fire('object:modified', { target: o } as any)
  }

  setFillGradient(colors: string[] | null, angle = 135) {
    if (!colors) return this.set({ fill: this.state.sel.fill || '#8B3DFF' })
    const o = this.active() as any
    if (!o) return
    const targets: any[] = o instanceof ActiveSelection ? o.getObjects() : [o]
    for (const t of targets) t.set('fill', linearGradient(colors, angle, 1, 1, 'percentage'))
    this.canvas.requestRenderAll()
    this.canvas.fire('object:modified', { target: o } as any)
  }

  setShadow(sh: SelInfo['shadow']) {
    this.set({ shadow: sh ? new Shadow(sh) : null })
  }

  /** Atur posisi/ukuran via panel (nilai absolut dalam px halaman). */
  setGeometry(g: Partial<{ left: number; top: number; width: number; height: number; angle: number }>) {
    const o = this.active() as any
    if (!o) return
    if (g.left !== undefined) o.set('left', g.left)
    if (g.top !== undefined) o.set('top', g.top)
    if (g.angle !== undefined) o.rotate(g.angle)
    if (g.width !== undefined) {
      if (o instanceof Textbox) o.set({ width: g.width / (o.scaleX || 1) })
      else o.set('scaleX', g.width / o.width)
    }
    if (g.height !== undefined && !(o instanceof Textbox)) o.set('scaleY', g.height / o.height)
    o.setCoords()
    this.canvas.requestRenderAll()
    this.canvas.fire('object:modified', { target: o } as any)
  }

  setFx(patch: Partial<Fx>) {
    const o = this.active() as any
    if (!(o instanceof FabricImage)) return
    const fx: Fx = { ...DEFAULT_FX, ...((o as any).fx || {}), ...patch }
    const f: any[] = []
    if (fx.brightness) f.push(new filters.Brightness({ brightness: fx.brightness }))
    if (fx.contrast) f.push(new filters.Contrast({ contrast: fx.contrast }))
    if (fx.saturation) f.push(new filters.Saturation({ saturation: fx.saturation }))
    if (fx.hue) f.push(new filters.HueRotation({ rotation: fx.hue }))
    if (fx.grayscale) f.push(new filters.Grayscale())
    if (fx.sepia) f.push(new filters.Sepia())
    if (fx.vintage) f.push(new filters.Vintage())
    if (fx.invert) f.push(new filters.Invert())
    if (fx.noise) f.push(new filters.Noise({ noise: fx.noise }))
    if (fx.pixelate) f.push(new filters.Pixelate({ blocksize: fx.pixelate }))
    if (fx.blur) f.push(new filters.Blur({ blur: fx.blur }))
    o.filters = f
    o.fx = fx
    o.applyFilters()
    this.canvas.requestRenderAll()
    this.canvas.fire('object:modified', { target: o } as any)
  }

  setFrame(type: 'rect' | 'circle', radius = 0) {
    const o = this.active()
    if (!(o instanceof FabricImage)) return
    applyFrame(o, type, radius)
    this.canvas.requestRenderAll()
    this.canvas.fire('object:modified', { target: o } as any)
  }

  flip(axis: 'x' | 'y') {
    const o = this.active() as any
    if (!o) return
    this.set(axis === 'x' ? { flipX: !o.flipX } : { flipY: !o.flipY })
  }

  // ---------- arrange ----------

  arrange(where: 'forward' | 'backward' | 'front' | 'back') {
    const o = this.active()
    if (!o) return
    const c = this.canvas
    if (where === 'forward') c.bringObjectForward(o)
    if (where === 'backward') c.sendObjectBackwards(o)
    if (where === 'front') c.bringObjectToFront(o)
    if (where === 'back') c.sendObjectToBack(o)
    c.requestRenderAll()
    c.fire('object:modified', { target: o } as any)
  }

  moveLayerTo(id: string, index: number) {
    const o = this.byId(id)
    if (!o) return
    this.canvas.moveObjectTo(o, index)
    this.canvas.requestRenderAll()
    this.canvas.fire('object:modified', { target: o } as any)
  }

  /** Rata kiri/tengah/kanan/atas/tengah/bawah terhadap halaman (1 objek) atau terhadap seleksi (multi). */
  align(where: 'left' | 'center' | 'right' | 'top' | 'middle' | 'bottom') {
    const o = this.active()
    if (!o) return
    const c = this.canvas
    let objs: FabricObject[]
    let box: { left: number; top: number; width: number; height: number }
    if (o instanceof ActiveSelection) {
      box = o.getBoundingRect()
      objs = o.getObjects()
      c.discardActiveObject()
    } else {
      objs = [o]
      box = { left: 0, top: 0, width: this.width, height: this.height }
    }
    for (const t of objs) {
      const r = t.getBoundingRect()
      let dx = 0
      let dy = 0
      if (where === 'left') dx = box.left - r.left
      if (where === 'center') dx = box.left + box.width / 2 - (r.left + r.width / 2)
      if (where === 'right') dx = box.left + box.width - (r.left + r.width)
      if (where === 'top') dy = box.top - r.top
      if (where === 'middle') dy = box.top + box.height / 2 - (r.top + r.height / 2)
      if (where === 'bottom') dy = box.top + box.height - (r.top + r.height)
      t.set({ left: t.left + dx, top: t.top + dy })
      t.setCoords()
    }
    if (o instanceof ActiveSelection) c.setActiveObject(new ActiveSelection(objs, { canvas: c }))
    c.requestRenderAll()
    c.fire('object:modified', {} as any)
  }

  distribute(axis: 'h' | 'v') {
    const o = this.active()
    if (!(o instanceof ActiveSelection) || o.size() < 3) return
    const c = this.canvas
    const objs = o.getObjects()
    c.discardActiveObject()
    const rects = objs.map((t) => ({ t, r: t.getBoundingRect() }))
    const key = axis === 'h' ? 'left' : 'top'
    const size = axis === 'h' ? 'width' : 'height'
    rects.sort((a, b) => a.r[key] - b.r[key])
    const start = rects[0].r[key]
    const end = rects[rects.length - 1].r[key] + rects[rects.length - 1].r[size]
    const total = rects.reduce((s, x) => s + x.r[size], 0)
    const gap = (end - start - total) / (rects.length - 1)
    let pos = start
    for (const { t, r } of rects) {
      t.set(key, t[key] + (pos - r[key]))
      t.setCoords()
      pos += r[size] + gap
    }
    c.setActiveObject(new ActiveSelection(objs, { canvas: c }))
    c.requestRenderAll()
    c.fire('object:modified', {} as any)
  }

  group() {
    const o = this.active()
    if (!(o instanceof ActiveSelection)) return
    const c = this.canvas
    const objs = o.getObjects()
    c.discardActiveObject()
    this.loading = true
    objs.forEach((x) => c.remove(x))
    const g = new Group(objs, { id: uid(), name: 'Grup' } as any)
    this.loading = false
    c.add(g)
    c.setActiveObject(g)
    c.requestRenderAll()
  }

  ungroup() {
    const o = this.active()
    if (!(o instanceof Group) || o instanceof ActiveSelection) return
    const c = this.canvas
    this.loading = true
    const objs = o.removeAll()
    c.remove(o)
    objs.forEach((x: any) => {
      if (!x.id) x.id = uid()
      c.add(x)
    })
    this.loading = false
    c.setActiveObject(new ActiveSelection(objs, { canvas: c }))
    c.requestRenderAll()
    c.fire('object:modified', {} as any)
  }

  toggleLock(target?: FabricObject) {
    const o = (target || this.active()) as any
    if (!o) return
    const lock = !o.locked
    o.set({ locked: lock, lockMovementX: lock, lockMovementY: lock, lockScalingX: lock, lockScalingY: lock, lockRotation: lock, hasControls: !lock, editable: !lock })
    this.canvas.requestRenderAll()
    this.canvas.fire('object:modified', { target: o } as any)
  }

  toggleVisible(id: string) {
    const o = this.byId(id)
    if (!o) return
    o.visible = !o.visible
    if (!o.visible && this.active() === o) this.canvas.discardActiveObject()
    this.canvas.requestRenderAll()
    this.canvas.fire('object:modified', { target: o } as any)
  }

  remove() {
    const c = this.canvas
    const objs = c.getActiveObjects().filter((o: any) => !o.locked)
    if (!objs.length) return
    c.discardActiveObject()
    objs.forEach((o) => c.remove(o))
    c.requestRenderAll()
  }

  get hasClipboard() {
    return !!this.clipboard
  }

  async copy() {
    const o = this.active()
    if (o) this.clipboard = await o.clone(SERIAL_PROPS)
  }

  async paste(source?: FabricObject | null) {
    const src = source || this.clipboard
    if (!src) return
    const c = this.canvas
    const cloned = await src.clone(SERIAL_PROPS)
    c.discardActiveObject()
    cloned.set({ left: cloned.left + 24, top: cloned.top + 24, evented: true })
    if (cloned instanceof ActiveSelection) {
      cloned.canvas = c
      cloned.forEachObject((x: any) => {
        x.set({ id: uid() })
        c.add(x)
      })
      cloned.setCoords()
    } else {
      cloned.set({ id: uid() } as any)
      c.add(cloned)
    }
    if (!source) this.clipboard = await cloned.clone(SERIAL_PROPS)
    c.setActiveObject(cloned)
    c.requestRenderAll()
  }

  async duplicate() {
    const o = this.active()
    if (o) await this.paste(o)
  }

  selectAll() {
    const c = this.canvas
    const objs = c.getObjects().filter((o: any) => o.visible && !o.locked)
    if (!objs.length) return
    c.discardActiveObject()
    c.setActiveObject(objs.length === 1 ? objs[0] : new ActiveSelection(objs, { canvas: c }))
    c.requestRenderAll()
  }

  nudge(dx: number, dy: number) {
    const o = this.active() as any
    if (!o || o.locked) return
    o.set({ left: o.left + dx, top: o.top + dy })
    o.setCoords()
    this.canvas.requestRenderAll()
    this.canvas.fire('object:modified', { target: o } as any)
  }

  // ---------- layers ----------

  byId(id: string) {
    return this.canvas.getObjects().find((o: any) => o.id === id) || null
  }

  selectLayer(id: string, additive = false) {
    const o = this.byId(id)
    if (!o || !o.visible) return
    const c = this.canvas
    if (additive) {
      const cur = c.getActiveObjects()
      const set = cur.includes(o) ? cur.filter((x) => x !== o) : [...cur, o]
      c.discardActiveObject()
      if (set.length === 1) c.setActiveObject(set[0])
      else if (set.length > 1) c.setActiveObject(new ActiveSelection(set, { canvas: c }))
    } else c.setActiveObject(o)
    c.requestRenderAll()
    this.refreshSel()
  }

  renameLayer(id: string, name: string) {
    const o = this.byId(id) as any
    if (!o) return
    o.name = name
    this.canvas.fire('object:modified', { target: o } as any)
  }

  refreshLayers() {
    if (!this.canvas) return
    const active = new Set(this.canvas.getActiveObjects())
    this.state.layers = this.canvas.getObjects().map((o: any) => ({
      id: o.id,
      name: o.name || (o instanceof Textbox ? o.text?.slice(0, 24) : o.type),
      type: o instanceof Textbox || o instanceof IText ? 'text' : o instanceof FabricImage ? 'image' : o instanceof Group ? 'group' : o.type,
      visible: o.visible !== false,
      locked: !!o.locked,
      selected: active.has(o),
    })).reverse()
  }

  private markLayerSelection() {
    const active = new Set(this.canvas.getActiveObjects().map((o: any) => o.id))
    this.state.layers.forEach((l) => (l.selected = active.has(l.id)))
  }

  // ---------- drawing ----------

  setDrawing(on: boolean) {
    const c = this.canvas
    c.isDrawingMode = on
    this.state.drawing = on
    if (on) {
      const b = new PencilBrush(c)
      b.color = this.state.brushColor
      b.width = this.state.brushWidth
      c.freeDrawingBrush = b
    }
  }

  setBrush(color?: string, width?: number) {
    if (color) this.state.brushColor = color
    if (width) this.state.brushWidth = width
    if (this.canvas.freeDrawingBrush) {
      this.canvas.freeDrawingBrush.color = this.state.brushColor
      this.canvas.freeDrawingBrush.width = this.state.brushWidth
    }
  }

  // ---------- AI ----------

  /** Spec halaman aktif (untuk dikirim ke AI "perbaiki desain"). */
  currentPageSpec() {
    return objectsToSpecPage(this.canvas, this.pages[this.state.pageIndex]?.name || 'Halaman')
  }

  allPagesSpec() {
    return { pages: this.pages.map((p, i) => (i === this.state.pageIndex ? this.currentPageSpec() : { name: p.name, elements: [] })) }
  }

  /** Terapkan spec hasil AI: ganti halaman aktif, atau tambahkan semua halaman di belakang. */
  async applySpec(spec: any, mode: 'replace-page' | 'append' | 'replace-all') {
    this.commitPage()
    const newPages = await specToPages(spec, this.width, this.height)
    if (!newPages.length) return
    if (mode === 'replace-page') {
      const cur = this.pages[this.state.pageIndex]
      // Simpan ke history agar bisa di-undo
      this.loading = true
      await renderSpecPage(this.canvas, spec.pages[0], this.width, this.height)
      this.loading = false
      cur.fabric = this.canvas.toObject(SERIAL_PROPS)
      if (newPages.length > 1) this.pages.splice(this.state.pageIndex + 1, 0, ...newPages.slice(1))
      this.readBackground()
      this.changed()
      this.refreshThumbs()
      return
    }
    if (mode === 'replace-all') {
      this.pages = newPages
      this.history.clear()
      this.refreshThumbs()
      await this.showPage(0)
    } else {
      const at = this.pages.length
      this.pages.push(...newPages)
      this.refreshThumbs()
      await this.showPage(at)
    }
    this.onChange()
  }

  /** Data URL objek terseleksi (untuk AI edit gambar). */
  activeDataURL() {
    const o = this.active()
    if (!o) return ''
    if (o instanceof FabricImage) {
      const el = o.getElement() as HTMLImageElement
      const cv = document.createElement('canvas')
      cv.width = el.naturalWidth || el.width
      cv.height = el.naturalHeight || el.height
      cv.getContext('2d')!.drawImage(el, 0, 0)
      return cv.toDataURL('image/png')
    }
    return o.toDataURL({ format: 'png', multiplier: 1 })
  }
}

function regularPolygon(sides: number, size: number) {
  const pts = []
  for (let i = 0; i < sides; i++) {
    const a = (2 * Math.PI * i) / sides - Math.PI / 2
    pts.push({ x: (size / 2) * (1 + Math.cos(a)), y: (size / 2) * (1 + Math.sin(a)) })
  }
  return pts
}

