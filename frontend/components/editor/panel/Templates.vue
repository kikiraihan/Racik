<script setup lang="ts">
import { scaleSpec } from '~/lib/spec'
import type { Template } from '~/composables/useApi'

const editor = useEditor()
const api = useApi()
const toast = useToast()
const templates = ref<Template[]>([])
const q = ref('')
const mode = ref<'append' | 'replace-page'>('append')

onMounted(async () => {
  templates.value = await api.get<Template[]>('/api/templates')
})

const shown = computed(() => {
  const s = q.value.toLowerCase()
  const r = editor.state.width / editor.state.height
  // Template dengan rasio mirip ditampilkan lebih dulu
  return templates.value
    .filter((t) => !s || (t.title + t.category).toLowerCase().includes(s))
    .sort((a, b) => Math.abs(a.width / a.height - r) - Math.abs(b.width / b.height - r))
})

async function apply(t: Template) {
  const spec = scaleSpec(t.spec, t.width, t.height, editor.state.width, editor.state.height)
  await editor.applySpec(spec, mode.value)
  toast.success(`Template "${t.title}" diterapkan`)
}
</script>

<template>
  <div class="col">
    <input v-model="q" class="input sm" placeholder="Cari template…" />
    <select v-model="mode" class="select sm">
      <option value="append">Tambahkan sebagai halaman baru</option>
      <option value="replace-page">Ganti halaman ini</option>
    </select>
    <div class="grid">
      <button v-for="t in shown" :key="t.id" class="t" @click="apply(t)">
        <SpecThumb :spec="t.spec" :width="t.width" :height="t.height" />
        <span class="nm">{{ t.title }}</span>
      </button>
    </div>
  </div>
</template>

<style scoped>
.grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; align-items: start; }
.t { border: 0; background: none; padding: 0; cursor: pointer; text-align: left; }
.t:hover :deep(.thumb) { outline: 2px solid var(--accent); outline-offset: 1px; }
.nm { font-size: 12px; font-weight: 600; display: block; margin-top: 4px; }
</style>
