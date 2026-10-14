<script setup lang="ts">
// Galeri template: pencarian + filter kategori, tampilan masonry seperti Canva.
import type { Template } from '~/composables/useApi'

definePageMeta({ layout: 'app' })

const api = useApi()
const toast = useToast()
const { fromTemplate } = useDesigns()

const templates = ref<Template[]>([])
const loaded = ref(false)
const tplCat = ref('')
const q = ref('')

const categories = computed(() => [...new Set(templates.value.map((t) => t.category))])
const shown = computed(() => {
  const s = q.value.trim().toLowerCase()
  return templates.value.filter((t) => (!tplCat.value || t.category === tplCat.value) && (!s || t.title.toLowerCase().includes(s) || t.category.toLowerCase().includes(s)))
})

onMounted(async () => {
  try {
    templates.value = await api.get<Template[]>('/api/templates')
  } catch (e: any) {
    toast.error('Backend tidak bisa dihubungi: ' + e.message)
  }
  loaded.value = true
})
</script>

<template>
  <div class="page">
    <div class="page-head">
      <div class="grow">
        <h1>Template</h1>
        <p>Pilih template, lalu ubah teks, warna, dan gambarnya di editor.</p>
      </div>
    </div>

    <div class="toolbar">
      <div class="search">
        <Icon name="search" :size="16" />
        <input v-model="q" class="input" placeholder="Cari template…" />
      </div>
    </div>
    <div class="cats">
      <button class="chip" :class="{ on: !tplCat }" @click="tplCat = ''">Semua</button>
      <button v-for="c in categories" :key="c" class="chip" :class="{ on: tplCat === c }" @click="tplCat = c">{{ c }}</button>
    </div>

    <div v-if="!loaded" class="muted">Memuat…</div>
    <div v-else-if="!shown.length" class="empty"><b>Template tidak ditemukan</b>Coba kata kunci atau kategori lain.</div>
    <div v-else class="masonry">
      <button v-for="t in shown" :key="t.id" class="tcard" @click="fromTemplate(t)">
        <SpecThumb :spec="t.spec" :width="t.width" :height="t.height" />
        <div class="ttitle">{{ t.title }}</div>
        <div class="muted small">{{ t.category }} · {{ t.spec?.pages?.length || 1 }} halaman</div>
      </button>
    </div>
  </div>
</template>

<style scoped>
.toolbar .search { max-width: 560px; }
.cats { display: flex; gap: 6px; flex-wrap: wrap; margin-bottom: 20px; }
.tcard { display: block; width: 100%; text-align: left; background: none; border: 0; padding: 0; cursor: pointer; }
.tcard :deep(.thumb) { border: 1px solid var(--line); border-radius: 10px; transition: border-color .15s; }
.tcard:hover :deep(.thumb) { border-color: var(--ink-2); }
.ttitle { font-weight: 500; margin-top: 8px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
@media (max-width: 720px) {
  .cats { flex-wrap: nowrap; overflow-x: auto; scrollbar-width: none; }
  .cats .chip { flex: 0 0 auto; }
}
</style>
