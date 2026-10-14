<script setup lang="ts">
// Dialog "Buat desain" (ala "Create a design" di Canva): kategori di kiri,
// pencarian + ukuran kanvas kosong + template di kanan. Ukuran kustom juga diatur di sini.
import { SIZE_PRESETS } from '~/lib/presets'
import type { Template } from '~/composables/useApi'

const api = useApi()
const { state, close } = useCreateDialog()
const { create, fromTemplate } = useDesigns()

const GROUP_ICON: Record<string, string> = { Presentasi: 'presentation', 'Media sosial': 'image', Video: 'play', Cetak: 'file', Lainnya: 'shapes' }
const TABS = [
  { key: 'for-you', text: 'Untuk kamu', icon: 'sparkles' },
  ...[...new Set(SIZE_PRESETS.map((p) => p.group))].map((g) => ({ key: g, text: g, icon: GROUP_ICON[g] || 'file' })),
  { key: 'custom', text: 'Ukuran kustom', icon: 'resize' },
]

const q = ref('')
const custom = reactive({ w: 1080, h: 1080 })
const templates = ref<Template[]>([])
const busy = ref(false)

const tab = computed(() => state.value.tab)
const presets = computed(() => {
  const s = q.value.trim().toLowerCase()
  return SIZE_PRESETS.filter((p) => (s ? p.label.toLowerCase().includes(s) || p.group.toLowerCase().includes(s) : tab.value === 'for-you' || p.group === tab.value))
})
const shownTemplates = computed(() => {
  const s = q.value.trim().toLowerCase()
  return templates.value.filter((t) => !s || t.title.toLowerCase().includes(s) || t.category.toLowerCase().includes(s)).slice(0, 8)
})

watch(() => state.value.open, async (o) => {
  if (!o) return (q.value = '')
  if (!templates.value.length) templates.value = await api.get<Template[]>('/api/templates').catch(() => [])
})

async function run(fn: () => Promise<any>) {
  busy.value = true
  try {
    await fn()
    close()
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div v-if="state.open" class="modal-back" @click.self="close()">
    <div class="cd" role="dialog" aria-label="Buat desain">
      <button class="btn ghost icon x" title="Tutup" @click="close()"><Icon name="x" :size="18" /></button>
      <aside class="cd-side">
        <h2>Buat desain</h2>
        <nav class="cd-tabs">
          <button v-for="t in TABS" :key="t.key" class="cd-tab" :class="{ on: tab === t.key && !q }" @click="state.tab = t.key; q = ''">
            <Icon :name="t.icon" :size="17" :stroke="1.8" /> {{ t.text }}
          </button>
        </nav>
      </aside>

      <section class="cd-main scroll">
        <div class="search">
          <Icon name="search" :size="16" />
          <input v-model="q" class="input" placeholder="Mau bikin apa? mis. poster, story, presentasi…" />
        </div>

        <template v-if="tab === 'custom' && !q">
          <h3>Ukuran kustom</h3>
          <div class="custom">
            <label><span class="label">Lebar (px)</span><input v-model.number="custom.w" type="number" class="input" min="50" max="8000" /></label>
            <span class="muted times">×</span>
            <label><span class="label">Tinggi (px)</span><input v-model.number="custom.h" type="number" class="input" min="50" max="8000" /></label>
            <button class="btn primary" :disabled="busy" @click="run(() => create('Desain tanpa judul', custom.w, custom.h))">
              <span v-if="busy" class="spin" /> Buat desain
            </button>
          </div>
        </template>

        <template v-else>
          <h3>Kanvas kosong</h3>
          <div v-if="presets.length" class="tiles">
            <PresetTile v-for="p in presets" :key="p.key" :label="p.label" :sub="`${p.width} × ${p.height}`" :width="p.width" :height="p.height"
              :disabled="busy" @click="run(() => create('Desain tanpa judul', p.width, p.height))" />
          </div>
          <p v-else class="muted">Tidak ada ukuran yang cocok. Coba <a href="#" class="lnk" @click.prevent="state.tab = 'custom'; q = ''">ukuran kustom</a>.</p>

          <template v-if="(tab === 'for-you' || q) && shownTemplates.length">
            <div class="row t-head">
              <h3 class="grow">Template</h3>
              <NuxtLink to="/templates" class="see-all" @click="close()">Lihat semua</NuxtLink>
            </div>
            <div class="masonry">
              <button v-for="t in shownTemplates" :key="t.id" class="tpl" :disabled="busy" @click="run(() => fromTemplate(t))">
                <SpecThumb :spec="t.spec" :width="t.width" :height="t.height" />
                <span class="tpl-title">{{ t.title }}</span>
              </button>
            </div>
          </template>
        </template>
      </section>
    </div>
  </div>
</template>

<style scoped>
.cd { position: relative; width: min(1080px, 100%); height: min(720px, 92vh); background: var(--panel); border-radius: 14px; display: grid; grid-template-columns: 240px minmax(0, 1fr); overflow: hidden; box-shadow: var(--shadow-lg); }
.x { position: absolute; top: 12px; right: 12px; z-index: 2; }
.cd-side { min-width: 0; padding: 24px 14px; border-right: 1px solid var(--line); background: #fbfbfb; }
.cd-side h2 { margin: 0 0 18px 8px; font-size: 20px; font-weight: 600; letter-spacing: -.02em; }
.cd-tabs { display: flex; flex-direction: column; gap: 2px; }
.cd-tab { display: flex; align-items: center; gap: 10px; height: 36px; padding: 0 10px; border: 0; border-radius: 8px; background: none; cursor: pointer; color: var(--ink-2); font-weight: 500; text-align: left; white-space: nowrap; }
.cd-tab:hover { background: var(--panel-2); }
.cd-tab.on { background: var(--panel-3); color: var(--ink); }
.cd-tab svg { color: var(--muted); }
.cd-main { padding: 24px 28px 28px; }
.cd-main .search { margin-right: 40px; }
h3 { font-size: 15px; font-weight: 600; margin: 24px 0 12px; }
.tiles { display: grid; grid-template-columns: repeat(auto-fill, minmax(150px, 1fr)); gap: 16px; }
.custom { display: flex; align-items: flex-end; gap: 10px; flex-wrap: wrap; }
.custom label { width: 140px; }
.times { padding-bottom: 9px; }
.t-head h3 { margin-bottom: 12px; }
.masonry { columns: 4 160px; column-gap: 14px; }
.masonry > * { break-inside: avoid; margin-bottom: 14px; }
.tpl { display: block; width: 100%; padding: 0; border: 0; background: none; cursor: pointer; text-align: left; }
.tpl :deep(.thumb) { border: 1px solid var(--line); border-radius: 8px; }
.tpl:hover :deep(.thumb) { border-color: var(--ink-2); }
.tpl-title { display: block; font-size: 12px; color: var(--ink-2); margin-top: 6px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }

@media (max-width: 720px) {
  .cd { grid-template-columns: minmax(0, 1fr); grid-template-rows: auto minmax(0, 1fr); height: 92vh; }
  .cd-side { padding: 18px 16px 10px; border-right: 0; border-bottom: 1px solid var(--line); }
  .cd-side h2 { margin: 0 0 12px; font-size: 18px; }
  .cd-tabs { flex-direction: row; overflow-x: auto; scrollbar-width: none; }
  .cd-tab { height: 32px; border: 1px solid var(--line); border-radius: 999px; flex: 0 0 auto; }
  .cd-tab.on { background: var(--ink); color: #fff; border-color: var(--ink); }
  .cd-tab.on svg { color: #fff; }
  .cd-main { padding: 16px; }
  .cd-main .search { margin-right: 0; }
  .tiles { grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }
  .masonry { columns: 2; }
}
</style>
