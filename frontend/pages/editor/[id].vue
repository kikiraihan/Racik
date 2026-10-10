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
const tab = ref<string>('ai')
const panelOpen = ref(true)
const showProps = ref(true)
const ready = ref(false)
const ctxMenu = reactive({ open: false, x: 0, y: 0 })
const dragOver = ref(false)
const st = editor.state
const brandColors = useState<string[]>('brandColors', () => [])

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
  window.addEventListener('paste', onPaste)
  window.addEventListener('beforeunload', onUnload)
})

onBeforeUnmount(() => {
  if (doc.dirty) save()
  window.removeEventListener('keydown', onKey)
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

// Zoom otomatis menyesuaikan area kerja sampai user mengatur zoom sendiri.
const autoFit = ref(true)
function fit() {
  autoFit.value = true
  if (workspace.value) editor.zoomFit(workspace.value)
}
let ro: ResizeObserver | null = null
watch(() => st.zoom, (z) => {
  if (workspace.value && Math.abs(z - Math.min((workspace.value.clientWidth - 64) / st.width, (workspace.value.clientHeight - 64) / st.height)) > 0.001) autoFit.value = false
})

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
  else if (mod && (e.key === '=' || e.key === '+')) { e.preventDefault(); editor.setZoom(st.zoom * 1.15) }
  else if (mod && e.key === '-') { e.preventDefault(); editor.setZoom(st.zoom / 1.15) }
  else if (mod && e.key === '0') { e.preventDefault(); fit() }
  else if (k === 'delete' || k === 'backspace') { e.preventDefault(); editor.remove() }
  else if (k === 'arrowleft') { e.preventDefault(); editor.nudge(-step, 0) }
  else if (k === 'arrowright') { e.preventDefault(); editor.nudge(step, 0) }
  else if (k === 'arrowup') { e.preventDefault(); editor.nudge(0, -step) }
  else if (k === 'arrowdown') { e.preventDefault(); editor.nudge(0, step) }
  else if (k === 'escape') { editor.canvas.discardActiveObject(); editor.canvas.requestRenderAll(); editor.setDrawing(false); ctxMenu.open = false }
  else if (!mod && k === 't') editor.addText('body')
  else if (!mod && k === 'r') editor.addShape('rect')
  else if (!mod && k === 'c') editor.addShape('circle')
  else if (!mod && k === 'l') editor.addShape('line')
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
  const url = e.dataTransfer?.getData('text/racik-image')
  if (url) return editor.addImage(url)
  for (const f of [...(e.dataTransfer?.files || [])]) await uploadAndAdd(f)
}

function onContext(e: MouseEvent) {
  e.preventDefault()
  const target = editor.canvas.findTarget(e as any)
  if (target && !editor.canvas.getActiveObjects().includes(target as any)) editor.canvas.setActiveObject(target as any)
  editor.canvas.requestRenderAll()
  editor.refreshSel()
  ctxMenu.x = e.clientX
  ctxMenu.y = e.clientY
  ctxMenu.open = true
}

function onWheel(e: WheelEvent) {
  if (e.ctrlKey || e.metaKey) {
    e.preventDefault()
    editor.setZoom(st.zoom * (e.deltaY < 0 ? 1.08 : 1 / 1.08))
  }
}

function openTab(t: string) {
  if (tab.value === t) panelOpen.value = !panelOpen.value
  else {
    tab.value = t
    panelOpen.value = true
  }
}
</script>

<template>
  <div class="ed">
    <EditorTopbar @save="save" @fit="fit" />
    <div class="body">
      <EditorSideRail :tab="tab" :open="panelOpen" @select="openTab" />
      <aside v-show="panelOpen" class="panel scroll">
        <EditorSidePanel v-if="ready" :tab="tab" @close="panelOpen = false" />
      </aside>
      <div class="center">
        <EditorContextBar v-if="ready" @toggle-props="showProps = !showProps" @open-props="showProps = true" />
        <div ref="workspace" class="workspace scroll" :class="{ drag: dragOver }" @wheel="onWheel"
          @dragover.prevent="dragOver = true" @dragleave="dragOver = false" @drop.prevent="onDrop" @contextmenu="onContext"
          @mousedown="ctxMenu.open = false">
          <div class="stage">
            <div class="page" :style="{ width: Math.round(st.width * st.zoom) + 'px', height: Math.round(st.height * st.zoom) + 'px' }">
              <canvas ref="canvasEl" />
            </div>
          </div>
          <div v-if="!ready" class="loading"><span class="spin" /> Memuat desain…</div>
        </div>
        <EditorPagesBar v-if="ready" @fit="fit" />
      </div>
      <aside v-if="ready && showProps && st.sel.kind !== 'none'" class="props scroll">
        <EditorPropsPanel @close="showProps = false" />
      </aside>
    </div>
    <EditorContextMenu v-if="ctxMenu.open" :x="ctxMenu.x" :y="ctxMenu.y" @close="ctxMenu.open = false" />
    <EditorAiFeedback v-if="ready && doc.generationId && !doc.rated" />
  </div>
</template>

<style scoped>
.ed { height: 100vh; display: flex; flex-direction: column; overflow: hidden; }
.body { flex: 1; display: flex; min-height: 0; }
.panel { width: 340px; flex: 0 0 340px; background: #fff; border-right: 1px solid var(--line); }
.props { width: 270px; flex: 0 0 270px; background: #fff; border-left: 1px solid var(--line); }
.center { flex: 1; display: flex; flex-direction: column; min-width: 0; position: relative; }
.workspace { flex: 1; position: relative; background: #ebecf0; }
.workspace.drag { outline: 3px dashed var(--accent); outline-offset: -8px; }
.stage { min-width: 100%; min-height: 100%; display: flex; align-items: center; justify-content: center; padding: 32px; width: max-content; }
.page { box-shadow: 0 2px 12px rgba(0,0,0,.12); background: #fff; flex: 0 0 auto; }
.loading { position: absolute; inset: 0; display: grid; place-items: center; gap: 8px; color: var(--muted); }
@media (max-width: 900px) {
  .panel { position: absolute; z-index: 40; left: 64px; top: 56px; bottom: 0; width: min(340px, calc(100vw - 64px)); box-shadow: var(--shadow-lg); }
  .props { display: none; }
}
</style>
