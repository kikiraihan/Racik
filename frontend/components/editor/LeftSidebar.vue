<script setup lang="ts">
// Sidebar kiri editor: logo, judul desain, tab "Desain" (token warna & font + layer) dan "Halaman".
const emit = defineEmits<{ collapse: []; 'focus-page': [number] }>()
const editor = useEditor()
const doc = useDoc()
const toast = useToast()
const st = editor.state

const tab = ref<'design' | 'pages'>('design')
const titleEl = ref<HTMLInputElement>()
const query = ref('')
const searching = ref(false)
const searchEl = ref<HTMLInputElement>()
const openSec = reactive({ colors: true, fonts: true, layers: true })
const renaming = ref(-1)

// ---------- token desain: warna & font yang dipakai di semua halaman ----------
const tokens = reactive({ colors: [] as { value: string; count: number }[], fonts: [] as { value: string; count: number }[] })
const COLOR_RE = /^(#[0-9a-f]{3,8}|rgba?\([^)]*\))$/i

function collect() {
  const colors = new Map<string, number>()
  const fonts = new Map<string, number>()
  const addColor = (c: any) => {
    if (typeof c !== 'string' || !COLOR_RE.test(c.trim())) return
    const k = c.trim().toLowerCase()
    colors.set(k, (colors.get(k) || 0) + 1)
  }
  const walk = (objs: any[] = []) => {
    for (const o of objs) {
      addColor(o.fill)
      if (o.strokeWidth) addColor(o.stroke)
      if (o.fontFamily) fonts.set(o.fontFamily, (fonts.get(o.fontFamily) || 0) + 1)
      if (o.objects) walk(o.objects)
    }
  }
  for (const p of editor.serialize().pages) {
    addColor(p.fabric?.background)
    walk(p.fabric?.objects)
  }
  const sorted = (m: Map<string, number>) => [...m].map(([value, count]) => ({ value, count })).sort((a, b) => b.count - a.count)
  tokens.colors = sorted(colors)
  tokens.fonts = sorted(fonts)
}
let tokTimer: any = null
watch(() => [st.layers, st.pages.length, st.background], () => {
  clearTimeout(tokTimer)
  tokTimer = setTimeout(collect, 300)
}, { immediate: true })
onBeforeUnmount(() => clearTimeout(tokTimer))

const q = computed(() => query.value.trim().toLowerCase())
const colors = computed(() => tokens.colors.filter((c) => !q.value || c.value.includes(q.value)))
const fonts = computed(() => tokens.fonts.filter((f) => !q.value || f.value.toLowerCase().includes(q.value)))
const tokenCount = computed(() => tokens.colors.length + tokens.fonts.length)

function toggleSearch() {
  searching.value = !searching.value
  if (searching.value) nextTick(() => searchEl.value?.focus())
  else query.value = ''
}

async function useColor(c: string) {
  // Ada seleksi → jadikan warna isi; tidak ada → salin kode warna.
  if (st.sel.kind !== 'none') return editor.set({ fill: c })
  try {
    await navigator.clipboard.writeText(c)
    toast.success(`${c} disalin`)
  } catch {}
}
function useFont(f: string) {
  if (st.sel.kind === 'text') editor.setFont(f)
  else toast.error('Pilih teks dulu untuk memakai font ini')
}

// ---------- halaman ----------
function openPage(i: number) {
  editor.goTo(i)
  emit('focus-page', i)
}
function finishRename(i: number, e: Event) {
  const v = (e.target as HTMLInputElement).value.trim()
  if (v) editor.renamePage(i, v)
  renaming.value = -1
}
</script>

<template>
  <aside class="ls">
    <div class="hd">
      <NuxtLink to="/" class="brand" title="Beranda"><EditorLogoMark :size="26" /><span>Racik</span></NuxtLink>
      <button class="ib" title="Tutup sidebar (Ctrl+\)" @click="emit('collapse')"><Icon name="panelLeft" :size="20" /></button>
    </div>

    <div class="title">
      <input ref="titleEl" v-model="doc.title" placeholder="Tidak ada judul" />
      <button class="ib sm" title="Ubah judul" @click="titleEl?.focus(); titleEl?.select()"><Icon name="edit" :size="16" /></button>
    </div>

    <div class="seg">
      <button :class="{ on: tab === 'design' }" @click="tab = 'design'">Desain</button>
      <button :class="{ on: tab === 'pages' }" @click="tab = 'pages'">Halaman</button>
    </div>

    <!-- Desain: token warna/font + layer -->
    <div v-if="tab === 'design'" class="body scroll">
      <div class="tk">
        <input v-if="searching" ref="searchEl" v-model="query" class="input sm grow" placeholder="Cari warna atau font…" @keyup.esc="toggleSearch" />
        <span v-else class="grow muted">{{ tokenCount }} token</span>
        <button class="ib sm" :title="searching ? 'Tutup pencarian' : 'Cari token'" @click="toggleSearch"><Icon :name="searching ? 'x' : 'search'" :size="17" /></button>
      </div>

      <button class="sec" @click="openSec.colors = !openSec.colors"><Icon :name="openSec.colors ? 'down' : 'right'" :size="15" /> Warna <span class="muted cnt">{{ colors.length }}</span></button>
      <div v-if="openSec.colors" class="items">
        <button v-for="c in colors" :key="c.value" class="tok" :title="st.sel.kind !== 'none' ? 'Pakai sebagai warna isi' : 'Salin kode warna'" @click="useColor(c.value)">
          <span class="sw" :style="{ background: c.value }" />
          <span class="grow nm">{{ c.value }}</span>
          <span class="muted cnt">{{ c.count }}×</span>
        </button>
        <p v-if="!colors.length" class="muted kosong">Belum ada warna.</p>
      </div>

      <button class="sec" @click="openSec.fonts = !openSec.fonts"><Icon :name="openSec.fonts ? 'down' : 'right'" :size="15" /> Font <span class="muted cnt">{{ fonts.length }}</span></button>
      <div v-if="openSec.fonts" class="items">
        <button v-for="f in fonts" :key="f.value" class="tok" title="Pakai di teks terpilih" @click="useFont(f.value)">
          <span class="sw ft" :style="{ fontFamily: f.value }">Aa</span>
          <span class="grow nm" :style="{ fontFamily: f.value }">{{ f.value }}</span>
          <span class="muted cnt">{{ f.count }}×</span>
        </button>
        <p v-if="!fonts.length" class="muted kosong">Belum ada teks.</p>
      </div>

      <button class="sec" @click="openSec.layers = !openSec.layers"><Icon :name="openSec.layers ? 'down' : 'right'" :size="15" /> Layer <span class="muted cnt">{{ st.layers.length }}</span></button>
      <div v-if="openSec.layers" class="items lay"><EditorPanelLayers /></div>
    </div>

    <!-- Halaman -->
    <div v-else class="body scroll">
      <div class="pages">
        <div v-for="(p, i) in st.pages" :key="p.id" class="pg" :class="{ on: i === st.pageIndex }" @click="openPage(i)" @dblclick="renaming = i">
          <div class="th" :style="{ aspectRatio: `${st.width}/${st.height}` }"><img v-if="p.thumb" :src="p.thumb" alt="" /></div>
          <div class="grow meta">
            <input v-if="renaming === i" class="input sm" :value="p.name" autofocus @click.stop @blur="finishRename(i, $event)" @keyup.enter="($event.target as HTMLInputElement).blur()" />
            <b v-else class="nm">{{ p.name }}</b>
            <span class="muted small">Halaman {{ i + 1 }} · {{ st.width }}×{{ st.height }}</span>
            <div class="pa" @click.stop>
              <button title="Naik" :disabled="i === 0" @click="editor.movePage(i, -1)"><Icon name="up" :size="13" /></button>
              <button title="Turun" :disabled="i === st.pages.length - 1" @click="editor.movePage(i, 1)"><Icon name="down" :size="13" /></button>
              <button title="Ganti nama" @click="renaming = i"><Icon name="edit" :size="13" /></button>
              <button title="Duplikat halaman" @click="editor.duplicatePage(i)"><Icon name="copy" :size="13" /></button>
              <button title="Hapus halaman" :disabled="st.pages.length < 2" @click="editor.deletePage(i)"><Icon name="trash" :size="13" /></button>
            </div>
          </div>
        </div>
      </div>
      <button class="btn add" @click="editor.addPage(st.pages.length - 1).then(() => emit('focus-page', st.pageIndex))"><Icon name="plus" :size="16" /> Tambah halaman</button>
    </div>

    <NuxtLink to="/brand" target="_blank" class="ft-link"><Icon name="brand" :size="18" /> <span class="grow">Brand Kit</span> <Icon name="external" :size="16" /></NuxtLink>
  </aside>
</template>

<style scoped>
.ls { width: 100%; height: 100%; display: flex; flex-direction: column; background: var(--panel); border-right: 1px solid var(--line); }
.hd { display: flex; align-items: center; justify-content: space-between; padding: 16px 16px 8px; }
.brand { display: flex; align-items: center; gap: 8px; font-weight: 700; font-size: 18px; letter-spacing: -.02em; }
.ib { border: 0; background: none; width: 34px; height: 34px; border-radius: 7px; display: grid; place-items: center; cursor: pointer; color: var(--ink-2); flex: 0 0 auto; }
.ib:hover { background: var(--panel-2); color: var(--ink); }
.ib.sm { width: 30px; height: 30px; }
.title { display: flex; align-items: center; gap: 6px; padding: 10px 12px 10px 16px; }
.title input { flex: 1; min-width: 0; border: 1px solid transparent; border-radius: 8px; padding: 6px 6px 6px 0; font-size: 16px; font-weight: 600; letter-spacing: -.01em; outline: none; background: none; }
.title input:focus { border-color: var(--line); padding-left: 6px; }
.title input::placeholder { color: var(--ink); }
.seg { margin: 4px 16px 10px; background: var(--panel-2); border-radius: 8px; padding: 3px; display: grid; grid-template-columns: 1fr 1fr; gap: 2px; }
.seg button { border: 0; background: none; height: 30px; border-radius: 6px; cursor: pointer; font-weight: 500; color: var(--muted); }
.seg button.on { background: var(--panel); color: var(--ink); box-shadow: 0 1px 2px rgba(0,0,0,.08); }
.body { flex: 1; min-height: 0; padding: 0 12px 12px; }
.tk { display: flex; align-items: center; gap: 6px; padding: 6px 4px 8px; min-height: 44px; }
.sec { display: flex; align-items: center; gap: 8px; width: 100%; border: 0; background: none; padding: 10px 4px; cursor: pointer; font-weight: 600; font-size: 14px; text-align: left; }
.sec .cnt { margin-left: auto; font-weight: 500; }
.items { display: flex; flex-direction: column; gap: 2px; padding-bottom: 6px; }
.items.lay { padding: 2px 4px 8px; }
.tok { display: flex; align-items: center; gap: 12px; border: 0; background: none; padding: 7px 6px; border-radius: 8px; cursor: pointer; text-align: left; }
.tok:hover { background: var(--panel-2); }
.sw { width: 28px; height: 28px; border-radius: 7px; box-shadow: inset 0 0 0 1px rgba(0,0,0,.1); flex: 0 0 28px; }
.sw.ft { display: grid; place-items: center; font-size: 13px; background: var(--panel-2); }
.nm { font-size: 14px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.cnt { font-size: 12px; }
.kosong { font-size: 12px; margin: 2px 6px 6px; }
.small { font-size: 12px; }
.pages { display: flex; flex-direction: column; gap: 6px; }
.pg { display: flex; gap: 10px; padding: 8px; border-radius: 8px; cursor: pointer; border: 1px solid transparent; }
.pg:hover { background: var(--panel-2); }
.pg.on { border-color: var(--line-2); background: var(--panel-2); }
.th { width: 84px; flex: 0 0 84px; max-height: 84px; border-radius: 6px; overflow: hidden; background: #fff; box-shadow: 0 0 0 1px var(--line); align-self: flex-start; }
.th img { width: 100%; height: 100%; object-fit: cover; display: block; }
.meta { display: flex; flex-direction: column; gap: 2px; }
.meta .nm { font-size: 13px; }
.pa { display: flex; gap: 2px; margin-top: 4px; }
.pa button { border: 0; background: rgba(0,0,0,.04); border-radius: 6px; width: 24px; height: 22px; display: grid; place-items: center; cursor: pointer; padding: 0; color: var(--ink-2); }
.pa button:hover:not(:disabled) { background: var(--panel); color: var(--ink); }
.pa button:disabled { opacity: .35; cursor: default; }
.add { width: 100%; margin-top: 10px; border-style: dashed; }
.ft-link { display: flex; align-items: center; gap: 10px; padding: 16px 18px; border-top: 1px solid var(--line); color: var(--ink-2); font-weight: 500; }
.ft-link:hover { color: var(--ink); background: var(--panel-2); }
</style>
