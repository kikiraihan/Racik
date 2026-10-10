<script setup lang="ts">
import { SIZE_PRESETS } from '~/lib/presets'
import type { DesignSummary, Template } from '~/composables/useApi'

const api = useApi()
const toast = useToast()
const { token } = useSession()
const { create, fromTemplate } = useDesigns()

const designs = ref<DesignSummary[]>([])
const templates = ref<Template[]>([])
const tplCat = ref('')
const q = ref('')
const custom = reactive({ open: false, w: 1080, h: 1080 })

const categories = computed(() => [...new Set(templates.value.map((t) => t.category))])
const shownTemplates = computed(() => templates.value.filter((t) => !tplCat.value || t.category === tplCat.value))

async function loadDesigns() {
  if (!token.value) return
  try {
    designs.value = await api.get<DesignSummary[]>('/api/designs', { q: q.value })
  } catch {}
}

onMounted(async () => {
  loadDesigns()
  try {
    templates.value = await api.get<Template[]>('/api/templates')
  } catch (e: any) {
    toast.error('Backend tidak bisa dihubungi: ' + e.message)
  }
})

async function removeDesign(d: DesignSummary) {
  if (!confirm(`Hapus "${d.title}"?`)) return
  await api.del(`/api/designs/${d.id}`)
  designs.value = designs.value.filter((x) => x.id !== d.id)
}
async function duplicateDesign(d: DesignSummary) {
  const c = await api.post<DesignSummary>(`/api/designs/${d.id}/duplicate`)
  designs.value.unshift(c)
}
async function star(d: DesignSummary) {
  d.starred = !d.starred
  await api.put(`/api/designs/${d.id}`, { starred: d.starred })
}
function ago(s: string) {
  const m = Math.round((Date.now() - new Date(s).getTime()) / 60000)
  if (m < 1) return 'baru saja'
  if (m < 60) return `${m} menit lalu`
  if (m < 1440) return `${Math.round(m / 60)} jam lalu`
  return new Date(s).toLocaleDateString('id-ID')
}
</script>

<template>
  <div>
    <AppHeader />
    <section class="hero">
      <h1>Mau bikin desain apa hari ini?</h1>
      <p class="sub">Tulis idemu, AI langsung menyusun desain siap edit — presentasi, poster, konten sosmed. Lalu rapikan sesukamu seperti di Canva.</p>
      <div class="pbox"><AiPromptBox /></div>
    </section>

    <main class="wrap">
      <section>
        <h2>Atau mulai dari kanvas kosong</h2>
        <div class="sizes scroll">
          <button v-for="p in SIZE_PRESETS" :key="p.key" class="size card" @click="create('Desain tanpa judul', p.width, p.height)">
            <div class="ratio" :style="{ aspectRatio: `${p.width}/${p.height}` }" />
            <b>{{ p.label }}</b>
            <span class="muted">{{ p.width }} × {{ p.height }} px</span>
          </button>
          <button class="size card" @click="custom.open = true">
            <div class="ratio plus"><Icon name="plus" :size="22" /></div>
            <b>Ukuran kustom</b>
            <span class="muted">Atur sendiri</span>
          </button>
        </div>
      </section>

      <section v-if="token">
        <div class="row">
          <h2 class="grow">Desain terbaru</h2>
          <input v-model="q" class="input sm" style="max-width: 240px" placeholder="Cari desain…" @input="loadDesigns" />
        </div>
        <div v-if="!designs.length" class="empty muted">Belum ada desain. Coba generate dengan AI di atas ✨</div>
        <div class="grid">
          <div v-for="d in designs" :key="d.id" class="dcard">
            <NuxtLink :to="`/editor/${d.id}`" class="dthumb" :style="{ aspectRatio: `${d.width}/${d.height}` }">
              <img v-if="d.thumbnail" :src="d.thumbnail" alt="" />
            </NuxtLink>
            <div class="row dmeta">
              <div class="grow">
                <div class="dtitle">{{ d.title }}</div>
                <div class="muted small">{{ d.width }}×{{ d.height }} · {{ ago(d.updatedAt) }}</div>
              </div>
              <button class="btn ghost sm icon" :title="d.starred ? 'Hapus bintang' : 'Beri bintang'" @click="star(d)" :style="{ color: d.starred ? '#f5a524' : '' }"><Icon name="star" :size="16" /></button>
              <button class="btn ghost sm icon" title="Duplikat" @click="duplicateDesign(d)"><Icon name="copy" :size="16" /></button>
              <button class="btn ghost sm icon danger" title="Hapus" @click="removeDesign(d)"><Icon name="trash" :size="16" /></button>
            </div>
          </div>
        </div>
      </section>

      <section>
        <div class="row" style="flex-wrap: wrap">
          <h2 class="grow">Template</h2>
          <button class="chip" :class="{ on: !tplCat }" @click="tplCat = ''">Semua</button>
          <button v-for="c in categories" :key="c" class="chip" :class="{ on: tplCat === c }" @click="tplCat = c">{{ c }}</button>
        </div>
        <div class="grid">
          <button v-for="t in shownTemplates" :key="t.id" class="tcard" @click="fromTemplate(t)">
            <SpecThumb :spec="t.spec" :width="t.width" :height="t.height" />
            <div class="dtitle">{{ t.title }}</div>
            <div class="muted small">{{ t.category }} · {{ t.spec?.pages?.length || 1 }} halaman</div>
          </button>
        </div>
      </section>
    </main>

    <div v-if="custom.open" class="modal-back" @click.self="custom.open = false">
      <div class="modal">
        <h3>Ukuran kustom</h3>
        <div class="grid2">
          <label><span class="label">Lebar (px)</span><input v-model.number="custom.w" type="number" class="input" min="50" max="8000" /></label>
          <label><span class="label">Tinggi (px)</span><input v-model.number="custom.h" type="number" class="input" min="50" max="8000" /></label>
        </div>
        <div class="row" style="justify-content: flex-end; margin-top: 16px">
          <button class="btn" @click="custom.open = false">Batal</button>
          <button class="btn primary" @click="create('Desain tanpa judul', custom.w, custom.h)">Buat desain</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.hero { background: radial-gradient(1200px 400px at 50% -10%, rgba(139,61,255,.25), transparent), linear-gradient(180deg, #fff, var(--bg)); padding: 56px 20px 40px; text-align: center; }
.hero h1 { font-size: clamp(28px, 5vw, 48px); margin: 0 0 10px; letter-spacing: -.03em; background: var(--grad); -webkit-background-clip: text; background-clip: text; color: transparent; }
.sub { color: var(--ink-2); max-width: 640px; margin: 0 auto 26px; font-size: 16px; line-height: 1.5; }
.pbox { max-width: 860px; margin: 0 auto; }
.wrap { max-width: 1200px; margin: 0 auto; padding: 10px 20px 60px; }
h2 { font-size: 20px; margin: 28px 0 14px; }
.sizes { display: flex; gap: 12px; padding-bottom: 8px; }
.size { flex: 0 0 150px; padding: 12px; display: flex; flex-direction: column; gap: 4px; align-items: flex-start; cursor: pointer; text-align: left; box-shadow: none; }
.size:hover { border-color: var(--accent); }
.size b { font-size: 13px; }
.size .muted { font-size: 12px; }
.ratio { width: auto; height: 70px; max-width: 100%; background: linear-gradient(135deg, #ece3ff, #d9f6f7); border-radius: 6px; margin-bottom: 6px; align-self: center; }
.ratio.plus { aspect-ratio: 1; display: grid; place-items: center; color: var(--accent); }
.grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(210px, 1fr)); gap: 18px; }
.dcard, .tcard { text-align: left; background: none; border: 0; padding: 0; cursor: pointer; }
.dthumb { display: block; width: 100%; max-height: 260px; border-radius: 10px; overflow: hidden; background: #e9eaef; border: 1px solid var(--line); }
.dthumb img { width: 100%; height: 100%; object-fit: cover; display: block; }
.dthumb:hover, .tcard:hover :deep(.thumb) { outline: 2px solid var(--accent); outline-offset: 2px; }
.dmeta { margin-top: 6px; gap: 2px; }
.dtitle { font-weight: 600; margin-top: 6px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.small { font-size: 12px; }
.empty { padding: 20px; border: 1px dashed var(--line); border-radius: 12px; text-align: center; }
</style>
