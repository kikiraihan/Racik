<script setup lang="ts">
// Konfigurasi AI: admin memilih model untuk tiap fitur. User tidak melihat pilihan ini.
interface AIConfig {
  textModel: string
  imageModel: string
  editModel: string
}

const api = useApi()
const toast = useToast()
const data = ref<{
  configured: boolean
  saved: AIConfig
  active: AIConfig
  textModels: string[]
  imageModels: string[]
  editModels: string[]
  labels: Record<string, string>
  defaults: { textModel: string; imageModel: string }
} | null>(null)
const form = reactive<AIConfig>({ textModel: '', imageModel: '', editModel: '' })
const saving = ref(false)

async function load() {
  try {
    data.value = await api.get('/api/admin/ai-config')
    Object.assign(form, data.value!.saved)
  } catch (e: any) {
    toast.error(e.message)
  }
}
onMounted(load)
// load dipanggil ulang dari halaman setelah model OpenRouter ditambah/dihapus.
// (Sekarang katalog ada di halaman sendiri, /admin/models; dropdown dimuat saat halaman dibuka.)
defineExpose({ load })

async function save() {
  saving.value = true
  try {
    data.value = await api.put('/api/admin/ai-config', form)
    Object.assign(form, data.value!.saved)
    toast.success('Konfigurasi AI tersimpan — langsung berlaku untuk semua user')
  } catch (e: any) {
    toast.error(e.message)
  } finally {
    saving.value = false
  }
}

const label = (m: string) => data.value?.labels[m] ?? m
const FIELDS = [
  { key: 'textModel', title: 'Model teks / layout', desc: 'Magic Design, Perbaiki desain, Copywriting', list: 'textModels' },
  { key: 'imageModel', title: 'Model gambar', desc: 'Gambar AI & gambar di dalam Magic Design', list: 'imageModels' },
  { key: 'editModel', title: 'Model edit gambar', desc: 'Edit gambar terpilih dengan instruksi', list: 'editModels' },
] as const
</script>

<template>
  <div class="adm">
    <div v-if="data" class="card sec col" style="gap: 14px">
      <div v-if="!data.configured" class="note">Belum ada provider AI aktif. Isi API key provider (mis. <code>KENARI_API_KEY</code>) di env backend lalu restart.</div>
      <p class="muted small" style="margin: 0">
        User tidak bisa memilih atau melihat model yang dipakai. API key provider tetap disimpan di env backend (tidak lewat browser);
        daftar model di bawah berasal dari provider yang API key-nya diisi.
        Pilihannya diambil dari <NuxtLink to="/admin/models" class="lnk">Katalog model</NuxtLink> — hanya model yang aktif di sana yang muncul;
        tambah atau nonaktifkan model lewat halaman itu.
      </p>
      <div v-for="f in FIELDS" :key="f.key" class="field">
        <div>
          <b>{{ f.title }}</b>
          <div class="muted small">{{ f.desc }}</div>
        </div>
        <div>
          <select v-model="form[f.key]" class="select">
            <option value="">Default env ({{ label(f.key === 'editModel' ? data.active.editModel : data.defaults[f.key as 'textModel' | 'imageModel'] || '-') }})</option>
            <option v-for="m in data[f.list]" :key="m" :value="m">{{ label(m) }}</option>
          </select>
          <div class="muted small" style="margin-top: 4px">Aktif sekarang: <span class="tag acc">{{ label(data.active[f.key]) || '—' }}</span></div>
        </div>
      </div>
      <div class="row" style="justify-content: flex-end">
        <button class="btn primary" :disabled="saving" @click="save"><span v-if="saving" class="spin" />Simpan</button>
      </div>
    </div>
    <div v-else class="muted">Memuat…</div>
  </div>
</template>

<style scoped>
.lnk { color: var(--accent); font-weight: 600; }
.field { display: grid; grid-template-columns: 1fr 1.4fr; gap: 16px; align-items: start; padding: 12px 0; border-top: 1px solid var(--line); }
@media (max-width: 720px) { .field { grid-template-columns: 1fr; gap: 8px; } }
</style>
