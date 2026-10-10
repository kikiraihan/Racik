<script setup lang="ts">
// Halaman editor desain (mirip Canva).
import { Editor } from '~/lib/editor'
import { DocKey, EditorKey, type DocMeta } from '~/composables/useEditor'

const route = useRoute()
const api = useApi()
const toast = useToast()

const editor = new Editor()
const doc = reactive<DocMeta>({ id: String(route.params.id), title: '', saving: false, savedAt: 0, dirty: false, generationId: '', rated: false })
provide(EditorKey, editor)
provide(DocKey, doc)

const canvasEl = ref<HTMLCanvasElement>()
const workspace = ref<HTMLElement>()
const chatEl = ref<HTMLElement>()
const panel = ref<string>('') // panel alat melayang (AI, template, elemen, …); kosong = tertutup
const rightTab = ref<'history' | 'props'>('history')
const ready = ref(false)
const ctxMenu = reactive({ open: false, x: 0, y: 0 })
const dragOver = ref(false)
const st = editor.state
const brandColors = useState<string[]>('brandColors', () => [])
const turns = useAiTurns()
turns.value = []

// ---------- sidebar kiri & kanan (bisa ditutup, diingat per browser) ----------
const leftOpen = ref(true)
const rightOpen = ref(true)
const isNarrow = () => window.innerWidth < 900
function readPref(k: string, def: boolean) {
  try {
    const v = localStorage.getItem('racik_ed_' + k)
    return v === null ? def : v === '1'
  } catch {
    return def
  }
}
function writePref(k: string, v: boolean) {
  if (isNarrow()) return // di layar kecil sidebar berupa overlay, tidak disimpan
  try {
    localStorage.setItem('racik_ed_' + k, v ? '1' : '0')
  } catch {}
}
watch(leftOpen, (v) => writePref('left', v))
watch(rightOpen, (v) => writePref('right', v))

// ---------- autosave ----------
let saveTimer: any = null
async function save() {
  if (!ready.value) return
  clearTimeout(saveTimer)
  doc.saving = true
  try {
    const data: any = editor.serialize()
    if (doc.generationId) data.generationId = doc.generationId
    const thumbnail = await editor.thumbnail()
    await api.put(`/api/designs/${doc.id}`, { title: doc.title, width: st.width, height: st.height, data, thumbnail })
    doc.dirty = false
    doc.savedAt = Date.now()
  } catch (e: any) {
    toast.error('Gagal menyimpan: ' + e.message)
  } finally {
    doc.saving = false
  }
}
function scheduleSave() {
  doc.dirty = true
  clearTimeout(saveTimer)
  saveTimer = setTimeout(save, 1500)
}
watch(() => doc.title, () => ready.value && scheduleSave())

// ---------- lifecycle ----------
onMounted(async () => {
  leftOpen.value = !isNarrow() && readPref('left', true)
  rightOpen.value = !isNarrow() && readPref('right', true)
  try {
    await api.ensureSession()
    api.get<any>('/api/brand-kit').then((b) => (brandColors.value = b.colors || [])).catch(() => {})
    const d = await api.get<any>(`/api/designs/${doc.id}`)
    doc.title = d.title
    doc.generationId = d.data?.generationId || ''
    editor.init(canvasEl.value!)
    editor.onChange = scheduleSave
    const hadSpec = !d.data?.pages?.length && d.data?.spec
    await editor.load(d.data || { pages: [] }, d.width, d.height)
    ready.value = true
    await nextTick()
    fit()
    ro = new ResizeObserver(() => autoFit.value && fit())
    ro.observe(workspace.value!)
    if (hadSpec) save() // simpan hasil render spec AI/template sebagai halaman fabric
    const warn = sessionStorage.getItem('racik_warn_' + doc.id)
    if (warn) {
      sessionStorage.removeItem('racik_warn_' + doc.id)
      JSON.parse(warn).forEach((w: string) => toast.error(w))
    }
  } catch (e: any) {
    toast.error(e.message)
    navigateTo('/')
  }
  window.addEventListener('keydown', onKey)
  window.addEventListener('keyup', onKeyUp)
  window.addEventListener('paste', onPaste)
  window.addEventListener('beforeunload', onUnload)
})

onBeforeUnmount(() => {
  if (doc.dirty) save()
  window.removeEventListener('keydown', onKey)
  window.removeEventListener('keyup', onKeyUp)
  window.removeEventListener('pointermove', onPointerMove)
  window.removeEventListener('pointerup', onPointerUp)
  window.removeEventListener('paste', onPaste)
  window.removeEventListener('beforeunload', onUnload)
  ro?.disconnect()
  editor.dispose()
})

function onUnload(e: BeforeUnloadEvent) {
  if (doc.dirty) {
    save()
    e.preventDefault()
  }
}

// ---------- board: frame halaman berjajar, bisa digeser (pan), di-zoom, dan dipindah ----------
// Zoom otomatis menyesuaikan area kerja sampai user mengatur zoom / menggeser board sendiri.
const autoFit = ref(true)
let ro: ResizeObserver | null = null
const pan = reactive({ x: 0, y: 0 })
const tool = ref<'select' | 'hand'>('select')
const spaceDown = ref(false)
const panning = ref(false)
const handMode = computed(() => tool.value === 'hand' || spaceDown.value)
const pageDrag = ref<{ i: number; x: number; y: number } | null>(null)

// Posisi frame (unit desain). Halaman tanpa posisi tersimpan berjajar ke kanan.
const gap = computed(() => Math.round(Math.max(st.width, st.height) * 0.12) + 40)
const layout = computed(() => {
  let right = -gap.value
  return st.pages.map((p, i) => {
    const d = pageDrag.value
    const r = d && d.i === i ? { x: d.x, y: d.y } : p.x != null ? { x: p.x, y: p.y ?? 0 } : { x: right + gap.value, y: 0 }
    right = Math.max(right, r.x + st.width)
    return r
  })
})
function frameStyle(i: number) {
  const p = layout.value[i] || { x: 0, y: 0 }
  return {
    left: Math.round(pan.x + p.x * st.zoom) + 'px',
    top: Math.round(pan.y + p.y * st.zoom) + 'px',
    width: Math.round(st.width * st.zoom) + 'px',
    height: Math.round(st.height * st.zoom) + 'px',
  }
}
const gridStyle = computed(() => ({ backgroundPosition: `${pan.x}px ${pan.y}px` }))
const zoomPct = computed(() => Math.round(st.zoom * 100))

/** Area board yang tidak tertutup UI melayang (rail, toolbar, chat). */
function viewport() {
  const el = workspace.value!
  const W = el.clientWidth
  const H = el.clientHeight
  const narrow = W < 640
  const L = narrow ? 76 : 104
  const R = narrow ? 16 : 40
  const T = 104
  const B = (chatEl.value?.offsetHeight || 120) + 44
  return { L, T, w: Math.max(80, W - L - R), h: Math.max(80, H - T - B) }
}

/** Pas-kan halaman i ke layar dan posisikan di tengah area kerja. */
function fitPage(i = st.pageIndex) {
  if (!workspace.value || !ready.value) return
  const v = viewport()
  editor.setZoom(Math.min(v.w / st.width, v.h / st.height))
  const p = layout.value[i] || { x: 0, y: 0 }
  pan.x = v.L + (v.w - st.width * st.zoom) / 2 - p.x * st.zoom
  pan.y = v.T + (v.h - st.height * st.zoom) / 2 - p.y * st.zoom
}
function fit() {
  autoFit.value = true
  fitPage()
}
function focusPage(i: number) {
  autoFit.value = true
  fitPage(i)
}

/** Zoom dengan titik (cx, cy) relatif board tetap di tempat. */
function zoomAt(z: number, cx?: number, cy?: number) {
  const el = workspace.value
  if (!el || !ready.value) return
  if (cx == null || cy == null) {
    const v = viewport()
    cx = v.L + v.w / 2
    cy = v.T + v.h / 2
  }
  const z0 = st.zoom
  editor.setZoom(z)
  const k = st.zoom / z0
  pan.x = cx - (cx - pan.x) * k
  pan.y = cy - (cy - pan.y) * k
  autoFit.value = false
}

// Geser board (alat geser, Spasi, klik tengah, atau seret area kosong) & pindah frame lewat label.
let drag: { kind: 'pan' | 'page'; i: number; sx: number; sy: number; ox: number; oy: number; moved: boolean } | null = null
function onBoardDown(e: PointerEvent) {
  ctxMenu.open = false
  const onEmpty = e.target === workspace.value
  if (!(e.button === 1 || (e.button === 0 && (handMode.value || onEmpty)))) return
  e.preventDefault()
  drag = { kind: 'pan', i: -1, sx: e.clientX, sy: e.clientY, ox: pan.x, oy: pan.y, moved: false }
  panning.value = true
  window.addEventListener('pointermove', onPointerMove)
  window.addEventListener('pointerup', onPointerUp)
}
function onLabelDown(i: number, e: PointerEvent) {
  if (e.button !== 0 || handMode.value) return
  e.stopPropagation()
  const p = layout.value[i]
  drag = { kind: 'page', i, sx: e.clientX, sy: e.clientY, ox: p.x, oy: p.y, moved: false }
  window.addEventListener('pointermove', onPointerMove)
  window.addEventListener('pointerup', onPointerUp)
}
function onPointerMove(e: PointerEvent) {
  if (!drag) return
  const dx = e.clientX - drag.sx
  const dy = e.clientY - drag.sy
  if (!drag.moved && Math.abs(dx) + Math.abs(dy) < 4) return
  drag.moved = true
  if (drag.kind === 'pan') {
    pan.x = drag.ox + dx
    pan.y = drag.oy + dy
    autoFit.value = false
  } else {
    pageDrag.value = { i: drag.i, x: drag.ox + dx / st.zoom, y: drag.oy + dy / st.zoom }
  }
}
function onPointerUp() {
  window.removeEventListener('pointermove', onPointerMove)
  window.removeEventListener('pointerup', onPointerUp)
  const d = drag
  drag = null
  panning.value = false
  if (!d) return
  if (d.kind === 'page') {
    const p = pageDrag.value
    pageDrag.value = null
    if (d.moved && p) editor.setPagePos(d.i, p.x, p.y)
    else editor.goTo(d.i)
  } else if (!d.moved && !handMode.value) {
    // klik area kosong board = batalkan seleksi
    editor.canvas.discardActiveObject()
    editor.canvas.requestRenderAll()
  }
}

/** Indeks halaman di bawah titik layar (untuk drop gambar ke frame mana pun). */
function pageAt(clientX: number, clientY: number) {
  const r = workspace.value!.getBoundingClientRect()
  const x = (clientX - r.left - pan.x) / st.zoom
  const y = (clientY - r.top - pan.y) / st.zoom
  return layout.value.findIndex((p) => x >= p.x && x <= p.x + st.width && y >= p.y && y <= p.y + st.height)
}

// ---------- keyboard ----------
function isTyping(e: KeyboardEvent) {
  const t = e.target as HTMLElement
  if (t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || t.tagName === 'SELECT' || t.isContentEditable)) return true
  const a = editor.active() as any
  return !!a?.isEditing
}

async function onKey(e: KeyboardEvent) {
  if (!ready.value || isTyping(e)) return
  const mod = e.ctrlKey || e.metaKey
  const k = e.key.toLowerCase()
  const step = e.shiftKey ? 10 : 1
  if (e.code === 'Space') {
    e.preventDefault()
    spaceDown.value = true
    return
  }
  if (mod && e.key === '\\') {
    e.preventDefault()
    const open = !(leftOpen.value || rightOpen.value)
    leftOpen.value = rightOpen.value = open
    return
  }
  if (mod && k === 'z' && !e.shiftKey) { e.preventDefault(); editor.undo() }
  else if (mod && (k === 'y' || (k === 'z' && e.shiftKey))) { e.preventDefault(); editor.redo() }
  else if (mod && k === 'c') { editor.copy() }
  else if (mod && k === 'x') { await editor.copy(); editor.remove() }
  else if (mod && k === 'v') { /* ditangani event paste (bisa gambar dari clipboard) */ }
  else if (mod && k === 'd') { e.preventDefault(); editor.duplicate() }
  else if (mod && k === 'a') { e.preventDefault(); editor.selectAll() }
  else if (mod && k === 'g' && e.shiftKey) { e.preventDefault(); editor.ungroup() }
  else if (mod && k === 'g') { e.preventDefault(); editor.group() }
  else if (mod && k === 's') { e.preventDefault(); save() }
  else if (mod && k === 'l') { e.preventDefault(); editor.toggleLock() }
  else if (mod && e.key === ']') { e.preventDefault(); editor.arrange(e.altKey ? 'front' : 'forward') }
  else if (mod && e.key === '[') { e.preventDefault(); editor.arrange(e.altKey ? 'back' : 'backward') }
  else if (mod && (e.key === '=' || e.key === '+')) { e.preventDefault(); zoomAt(st.zoom * 1.15) }
  else if (mod && e.key === '-') { e.preventDefault(); zoomAt(st.zoom / 1.15) }
  else if (mod && e.key === '0') { e.preventDefault(); fit() }
  else if (k === 'delete' || k === 'backspace') { e.preventDefault(); editor.remove() }
  else if (k === 'arrowleft') { e.preventDefault(); editor.nudge(-step, 0) }
  else if (k === 'arrowright') { e.preventDefault(); editor.nudge(step, 0) }
  else if (k === 'arrowup') { e.preventDefault(); editor.nudge(0, -step) }
  else if (k === 'arrowdown') { e.preventDefault(); editor.nudge(0, step) }
  else if (k === 'escape') { editor.canvas.discardActiveObject(); editor.canvas.requestRenderAll(); editor.setDrawing(false); ctxMenu.open = false }
  else if (!mod && k === 'v') tool.value = 'select'
  else if (!mod && k === 'h') tool.value = 'hand'
  else if (!mod && k === 't') editor.addText('body')
  else if (!mod && k === 'r') editor.addShape('rect')
  else if (!mod && k === 'c') editor.addShape('circle')
  else if (!mod && k === 'l') editor.addShape('line')
}

function onKeyUp(e: KeyboardEvent) {
  if (e.code === 'Space') spaceDown.value = false
}

// ---------- paste & drop gambar ----------
async function uploadAndAdd(file: File) {
  if (!file.type.startsWith('image/')) return
  try {
    const a = await api.upload(file)
    await editor.addImage(a.url, { name: file.name })
  } catch (e: any) {
    toast.error(e.message)
  }
}

async function onPaste(e: ClipboardEvent) {
  if (!ready.value || isTyping(e as any)) return
  const files = [...(e.clipboardData?.files || [])].filter((f) => f.type.startsWith('image/'))
  if (files.length) {
    e.preventDefault()
    for (const f of files) await uploadAndAdd(f)
    return
  }
  const text = e.clipboardData?.getData('text/plain')
  if (text && !editor.hasClipboard) {
    editor.addText({ text: text.slice(0, 2000), fontSize: 36 })
    return
  }
  editor.paste()
}

async function onDrop(e: DragEvent) {
  dragOver.value = false
  const i = pageAt(e.clientX, e.clientY)
  if (i >= 0 && i !== st.pageIndex) await editor.goTo(i)
  const url = e.dataTransfer?.getData('text/racik-image')
  if (url) return editor.addImage(url)
  for (const f of [...(e.dataTransfer?.files || [])]) await uploadAndAdd(f)
}

function onContext(e: MouseEvent) {
  e.preventDefault()
  // klik kanan di luar kanvas aktif (area board) tidak memilih objek
  const target = (e.target as HTMLElement).closest?.('.live') ? editor.canvas.findTarget(e as any) : undefined
  if (target && !editor.canvas.getActiveObjects().includes(target as any)) editor.canvas.setActiveObject(target as any)
  editor.canvas.requestRenderAll()
  editor.refreshSel()
  ctxMenu.x = e.clientX
  ctxMenu.y = e.clientY
  ctxMenu.open = true
}

function onWheel(e: WheelEvent) {
  e.preventDefault()
  if (e.ctrlKey || e.metaKey) {
    // Ctrl+scroll / pinch trackpad = zoom di posisi kursor
    const r = workspace.value!.getBoundingClientRect()
    zoomAt(st.zoom * Math.exp(-Math.max(-50, Math.min(50, e.deltaY)) * 0.01), e.clientX - r.left, e.clientY - r.top)
  } else {
    // scroll biasa = geser board (Shift = horizontal)
    pan.x -= e.shiftKey && !e.deltaX ? e.deltaY : e.deltaX
    pan.y -= e.shiftKey && !e.deltaX ? 0 : e.deltaY
    autoFit.value = false
  }
}

function openTab(t: string) {
  panel.value = panel.value === t ? '' : t
}
function openProps() {
  rightOpen.value = true
  rightTab.value = 'props'
}
// Seleksi baru → tampilkan tab Properti di sidebar kanan.
watch(() => st.sel.kind, (k, prev) => {
  if (k !== 'none' && prev === 'none') rightTab.value = 'props'
})
// Sidebar dibuka/ditutup → area kerja berubah; pas-kan ulang kalau masih mode otomatis.
watch([leftOpen, rightOpen], () => nextTick(() => autoFit.value && fitPage()))
</script>

<template>
  <div class="ed" :class="{ 'l-open': leftOpen, 'r-open': rightOpen }">
    <!-- sidebar kiri: judul, token desain, layer, halaman -->
    <div v-if="leftOpen" class="side left">
      <EditorLeftSidebar v-if="ready" @collapse="leftOpen = false" @focus-page="focusPage" />
    </div>

    <main class="main">
      <!-- board: semua halaman sebagai frame; halaman aktif = kanvas fabric yang bisa diedit -->
      <div ref="workspace" class="ws" :class="{ drag: dragOver, hand: handMode, panning }" :style="gridStyle"
        @pointerdown="onBoardDown" @wheel="onWheel" @auxclick.prevent
        @dragover.prevent="dragOver = true" @dragleave="dragOver = false" @drop.prevent="onDrop" @contextmenu="onContext">
        <template v-if="ready">
          <template v-for="(p, i) in st.pages" :key="p.id">
            <div class="flabel" :class="{ on: i === st.pageIndex }" :style="{ left: frameStyle(i).left, top: `calc(${frameStyle(i).top} - 34px)`, maxWidth: frameStyle(i).width }"
              title="Klik untuk mengedit · seret untuk memindah frame" @pointerdown="onLabelDown(i, $event)">
              <b>{{ p.name }}</b><span>/{{ i + 1 }}</span>
            </div>
            <div v-if="i !== st.pageIndex" class="frame" :style="frameStyle(i)" @click="editor.goTo(i)">
              <img v-if="p.thumb" :src="p.thumb" alt="" draggable="false" />
            </div>
          </template>
        </template>
        <div class="frame live" :class="{ on: ready }" :style="frameStyle(st.pageIndex)">
          <canvas ref="canvasEl" />
        </div>
        <div v-if="handMode" class="panlayer" />
        <div v-if="!ready" class="loading"><span class="spin" /> Memuat desain…</div>
      </div>

      <!-- pil saat sidebar tertutup -->
      <div v-if="!leftOpen" class="pill l">
        <NuxtLink to="/" class="pb" title="Beranda"><EditorLogoMark :size="28" /></NuxtLink>
        <button class="pb" title="Buka sidebar (Ctrl+\)" @click="leftOpen = true"><Icon name="panelLeft" :size="21" /></button>
      </div>
      <div v-if="!rightOpen" class="pill r">
        <button class="pb" title="Buka sidebar" @click="rightOpen = true"><Icon name="panelRight" :size="21" /></button>
      </div>

      <div v-if="ready" class="ctx">
        <EditorContextBar @toggle-props="rightOpen && rightTab === 'props' ? (rightTab = 'history') : openProps()" @open-props="openProps" />
      </div>

      <EditorToolRail v-if="ready" v-model:tool="tool" class="rail" :panel="panel" @open="openTab" />
      <div v-if="ready && panel" class="toolpanel scroll">
        <EditorSidePanel :tab="panel" @close="panel = ''" />
      </div>

      <div v-if="ready" class="zoom">
        <button title="Perbesar (Ctrl +)" @click="zoomAt(st.zoom * 1.2)"><Icon name="plus" :size="20" /></button>
        <button class="pct" title="Zoom 100%" @click="zoomAt(1)">{{ zoomPct }}%</button>
        <button title="Perkecil (Ctrl -)" @click="zoomAt(st.zoom / 1.2)"><Icon name="minus" :size="20" /></button>
        <button title="Pas ke layar (Ctrl+0)" @click="fit"><Icon name="fit" :size="18" /></button>
      </div>

      <div v-if="ready" ref="chatEl" class="chatwrap"><EditorAiChat /></div>
      <EditorAiFeedback v-if="ready && !rightOpen && doc.generationId && !doc.rated" />
    </main>

    <!-- sidebar kanan: akun, export, riwayat AI, properti -->
    <div v-if="rightOpen" class="side right">
      <EditorRightSidebar v-if="ready" v-model:tab="rightTab" @collapse="rightOpen = false" @save="save" @fit="fit" @open-tool="openTab" />
    </div>
    <div v-if="(leftOpen || rightOpen) && ready" class="scrim" @click="leftOpen = rightOpen = false" />

    <EditorContextMenu v-if="ctxMenu.open" :x="ctxMenu.x" :y="ctxMenu.y" @close="ctxMenu.open = false" />
  </div>
</template>

<style scoped>
.ed { --board: #efedf6; height: 100vh; display: flex; overflow: hidden; background: var(--board); }
.side { flex: 0 0 auto; height: 100%; z-index: 30; }
.side.left { width: 290px; }
.side.right { width: 320px; }
.main { flex: 1; min-width: 0; position: relative; }
.ws { position: absolute; inset: 0; overflow: hidden; background-color: var(--board); background-image: radial-gradient(rgba(80,60,140,.13) 1px, transparent 1.2px); background-size: 22px 22px; touch-action: none; }
.ws.drag { outline: 3px dashed var(--accent); outline-offset: -8px; }
.ws.hand { cursor: grab; }
.ws.panning, .ws.panning * { cursor: grabbing !important; }
.frame { position: absolute; background: #fff; box-shadow: 0 1px 2px rgba(16,24,40,.06), 0 8px 28px rgba(60,40,120,.10); cursor: pointer; border-radius: 2px; }
.frame:not(.live):hover { outline: 2px solid rgba(139,61,255,.45); }
.frame img { width: 100%; height: 100%; display: block; object-fit: cover; pointer-events: none; }
.frame.live { cursor: default; visibility: hidden; }
.frame.live.on { visibility: visible; outline: 2px dashed rgba(139,61,255,.55); outline-offset: 3px; }
.flabel { position: absolute; height: 28px; display: flex; align-items: baseline; gap: 6px; white-space: nowrap; overflow: hidden; font-size: 17px; color: #6b6880; cursor: grab; user-select: none; padding-right: 4px; }
.flabel b { font-weight: 600; overflow: hidden; text-overflow: ellipsis; }
.flabel span { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 16px; opacity: .8; }
.flabel.on { color: var(--accent); }
.panlayer { position: absolute; inset: 0; z-index: 5; }
.loading { position: absolute; inset: 0; display: grid; place-items: center; gap: 8px; color: var(--muted); }

/* UI melayang di atas board */
.pill { position: absolute; top: 16px; z-index: 20; display: flex; gap: 4px; padding: 6px; background: #fff; border: 1px solid var(--line); border-radius: 16px; box-shadow: var(--shadow); }
.pill.l { left: 16px; }
.pill.r { right: 16px; }
.pb { width: 42px; height: 42px; border: 0; background: none; border-radius: 11px; display: grid; place-items: center; cursor: pointer; color: var(--ink-2); }
.pb:hover { background: var(--panel-2); color: var(--ink); }
.ctx { position: absolute; top: 16px; left: 0; right: 0; margin: 0 auto; width: fit-content; max-width: calc(100% - 240px); z-index: 15; }
.ctx :deep(.bar) { border: 1px solid var(--line); border-radius: 14px; box-shadow: var(--shadow); max-width: 100%; white-space: nowrap; }
.rail { position: absolute; left: 16px; top: 50%; transform: translateY(-50%); z-index: 20; }
.toolpanel { position: absolute; left: 104px; top: 80px; bottom: 20px; width: 340px; max-height: 680px; z-index: 25; background: #fff; border: 1px solid var(--line); border-radius: 18px; box-shadow: var(--shadow-lg); }
.toolpanel :deep(.hd) { border-radius: 18px 18px 0 0; }
.zoom { position: absolute; left: 16px; bottom: 20px; z-index: 20; display: flex; flex-direction: column; background: #fff; border: 1px solid var(--line); border-radius: 14px; box-shadow: var(--shadow); overflow: hidden; }
.zoom button { width: 46px; height: 42px; border: 0; background: none; display: grid; place-items: center; cursor: pointer; color: var(--ink-2); }
.zoom button:hover { background: var(--panel-2); color: var(--ink); }
.zoom .pct { font-size: 11px; font-weight: 600; height: 26px; color: var(--muted); }
.chatwrap { position: absolute; left: 96px; right: 24px; bottom: 20px; margin: 0 auto; max-width: 820px; z-index: 18; }
.scrim { display: none; }

@media (max-width: 900px) {
  .side { position: fixed; top: 0; bottom: 0; z-index: 60; box-shadow: var(--shadow-lg); }
  .side.left { left: 0; width: min(290px, 86vw); }
  .side.right { right: 0; width: min(320px, 90vw); }
  .scrim { display: block; position: fixed; inset: 0; z-index: 55; background: rgba(14,19,24,.3); }
  .ctx { top: 72px; max-width: calc(100% - 32px); }
  .toolpanel { left: 76px; right: 12px; width: auto; top: 128px; }
}
@media (max-width: 640px) {
  .rail { left: 10px; }
  .zoom { left: 10px; }
  .chatwrap { left: 66px; right: 10px; bottom: 12px; }
}
</style>
