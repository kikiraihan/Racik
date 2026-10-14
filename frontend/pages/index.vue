<script setup lang="ts">
import { SIZE_PRESETS } from '~/lib/presets'
import type { DesignSummary, Template } from '~/composables/useApi'

definePageMeta({ layout: 'app' })

const api = useApi()
const toast = useToast()
const { token } = useSession()
const { create, fromTemplate, remove, duplicate, toggleStar } = useDesigns()
const createDialog = useCreateDialog()

const designs = ref<DesignSummary[]>([])
const templates = ref<Template[]>([])
const tplCat = ref('')

// Pintasan ukuran di Beranda (daftar lengkap ada di dialog "Buat desain").
const QUICK = ['presentation', 'instagram-post', 'story', 'poster', 'youtube-thumbnail', 'facebook-post']
const quickSizes = computed(() => SIZE_PRESETS.filter((p) => QUICK.includes(p.key)))

const categories = computed(() => [...new Set(templates.value.map((t) => t.category))])
const shownTemplates = computed(() => templates.value.filter((t) => !tplCat.value || t.category === tplCat.value).slice(0, 10))

async function loadDesigns() {
  if (!token.value) return
  try {
    designs.value = (await api.get<DesignSummary[]>('/api/designs')).slice(0, 5)
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
  if (await remove(d)) designs.value = designs.value.filter((x) => x.id !== d.id)
}
async function duplicateDesign(d: DesignSummary) {
  designs.value.unshift(await duplicate(d))
  designs.value = designs.value.slice(0, 5)
}
</script>

<template>
  <div class="page">
    <section class="hero">
      <h1>Mau bikin desain apa hari ini?</h1>
      <p class="sub">Tulis idemu, AI langsung menyusun desain siap edit — presentasi, poster, konten sosmed.</p>
      <div class="pbox"><AiPromptBox /></div>
    </section>

    <section>
      <div class="sec-head">
        <h2>Mulai dari kanvas kosong</h2>
        <a href="#" class="see-all" @click.prevent="createDialog.open()">Semua ukuran</a>
      </div>
      <div class="sizes scroll">
        <PresetTile v-for="p in quickSizes" :key="p.key" class="size" :label="p.label" :sub="`${p.width} × ${p.height}`" :width="p.width" :height="p.height"
          @click="create('Desain tanpa judul', p.width, p.height)" />
        <PresetTile class="size" label="Ukuran kustom" sub="Atur sendiri" icon="plus" @click="createDialog.open('custom')" />
      </div>
    </section>

    <section v-if="token">
      <div class="sec-head">
        <h2>Desain terbaru</h2>
        <NuxtLink v-if="designs.length" to="/designs" class="see-all">Lihat semua</NuxtLink>
      </div>
      <div v-if="!designs.length" class="empty"><b>Belum ada desain</b>Coba generate dengan AI di atas, atau mulai dari kanvas kosong.</div>
      <div v-else class="dgrid">
        <DesignCard v-for="d in designs" :key="d.id" :d="d" @star="toggleStar(d)" @duplicate="duplicateDesign(d)" @remove="removeDesign(d)" />
      </div>
    </section>

    <section>
      <div class="sec-head">
        <h2>Template</h2>
        <NuxtLink to="/templates" class="see-all">Lihat semua</NuxtLink>
      </div>
      <div class="cats">
        <button class="chip" :class="{ on: !tplCat }" @click="tplCat = ''">Semua</button>
        <button v-for="c in categories" :key="c" class="chip" :class="{ on: tplCat === c }" @click="tplCat = c">{{ c }}</button>
      </div>
      <div class="masonry">
        <button v-for="t in shownTemplates" :key="t.id" class="tcard" @click="fromTemplate(t)">
          <SpecThumb :spec="t.spec" :width="t.width" :height="t.height" />
          <div class="ttitle">{{ t.title }}</div>
          <div class="muted small">{{ t.category }} · {{ t.spec?.pages?.length || 1 }} halaman</div>
        </button>
      </div>
    </section>
  </div>
</template>

<style scoped>
.hero { text-align: center; padding: 28px 0 8px; }
.hero h1 { font-size: clamp(26px, 4vw, 38px); font-weight: 600; letter-spacing: -.03em; margin: 0 0 8px; }
.sub { color: var(--muted); max-width: 560px; margin: 0 auto 24px; font-size: 15px; line-height: 1.5; }
.pbox { max-width: 860px; margin: 0 auto; }
.sizes { display: flex; gap: 16px; padding-bottom: 6px; }
.size { flex: 0 0 168px; }
.cats { display: flex; gap: 6px; flex-wrap: wrap; margin-bottom: 16px; }
.tcard { display: block; width: 100%; text-align: left; background: none; border: 0; padding: 0; cursor: pointer; }
.tcard :deep(.thumb) { border: 1px solid var(--line); border-radius: 10px; transition: border-color .15s; }
.tcard:hover :deep(.thumb) { border-color: var(--ink-2); }
.ttitle { font-weight: 500; margin-top: 8px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
@media (max-width: 720px) {
  .hero { padding-top: 8px; text-align: left; }
  .sub { margin-left: 0; }
  .size { flex-basis: 132px; }
  .cats { flex-wrap: nowrap; overflow-x: auto; scrollbar-width: none; }
  .cats .chip { flex: 0 0 auto; }
}
</style>
