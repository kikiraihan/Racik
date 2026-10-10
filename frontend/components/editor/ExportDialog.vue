<script setup lang="ts">
import { exportDesign, type ExportFormat } from '~/lib/export'

const props = defineProps<{ title: string }>()
const emit = defineEmits<{ close: [] }>()
const editor = useEditor()
const toast = useToast()
const st = editor.state

const FORMATS: { k: ExportFormat; label: string; desc: string }[] = [
  { k: 'png', label: 'PNG', desc: 'Gambar kualitas tinggi, bisa transparan' },
  { k: 'jpg', label: 'JPG', desc: 'Ukuran file kecil untuk dibagikan' },
  { k: 'pdf', label: 'PDF', desc: 'Dokumen siap cetak, multi halaman' },
  { k: 'pptx', label: 'PowerPoint (PPTX)', desc: 'Teks tetap bisa diedit di PowerPoint / Google Slides' },
  { k: 'svg', label: 'SVG', desc: 'Grafik vektor' },
  { k: 'json', label: 'Racik JSON', desc: 'Backup desain, bisa diimpor kembali' },
]
const format = ref<ExportFormat>(st.width > st.height ? 'pptx' : 'png')
const scale = ref(2)
const quality = ref(0.92)
const transparent = ref(false)
const editableText = ref(true)
const which = ref<'all' | 'current'>('all')
const busy = ref(false)

async function go() {
  busy.value = true
  try {
    const data = editor.serialize()
    const pages = which.value === 'all' ? data.pages.map((_, i) => i) : [st.pageIndex]
    await exportDesign(props.title, data.pages, st.width, st.height, { format: format.value, pages, scale: scale.value, quality: quality.value, transparent: transparent.value, editableText: editableText.value })
    toast.success('Download dimulai')
    emit('close')
  } catch (e: any) {
    toast.error('Export gagal: ' + e.message)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="modal-back" @click.self="emit('close')">
    <div class="modal" style="color: var(--ink)">
      <h3>Download desain</h3>
      <div class="fmts">
        <label v-for="f in FORMATS" :key="f.k" class="fm" :class="{ on: format === f.k }">
          <input v-model="format" type="radio" :value="f.k" hidden />
          <b>{{ f.label }}</b><span class="muted">{{ f.desc }}</span>
        </label>
      </div>
      <div class="grid2" style="margin-top: 14px">
        <label v-if="['png', 'jpg', 'pdf', 'pptx'].includes(format)"><span class="label">Resolusi</span>
          <select v-model.number="scale" class="select sm"><option :value="1">1x ({{ st.width }}px)</option><option :value="2">2x</option><option :value="3">3x</option><option :value="4">4x</option></select></label>
        <label v-if="['jpg', 'pdf'].includes(format)"><span class="label">Kualitas {{ Math.round(quality * 100) }}%</span>
          <input v-model.number="quality" type="range" min="0.4" max="1" step="0.01" /></label>
        <label v-if="format !== 'json'"><span class="label">Halaman</span>
          <select v-model="which" class="select sm"><option value="all">Semua ({{ st.pages.length }})</option><option value="current">Halaman ini saja</option></select></label>
      </div>
      <label v-if="format === 'png'" class="row" style="margin-top: 10px"><input v-model="transparent" type="checkbox" /> Latar transparan</label>
      <label v-if="format === 'pptx'" class="row" style="margin-top: 10px"><input v-model="editableText" type="checkbox" /> Teks bisa diedit di PowerPoint (matikan untuk tampilan 100% identik sebagai gambar)</label>
      <div class="row" style="justify-content: flex-end; margin-top: 18px">
        <button class="btn" @click="emit('close')">Batal</button>
        <button class="btn primary" :disabled="busy" @click="go"><span v-if="busy" class="spin" /><Icon v-else name="download" :size="16" /> Download</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.fmts { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; }
.fm { border: 1px solid var(--line); border-radius: 10px; padding: 10px 12px; cursor: pointer; display: flex; flex-direction: column; gap: 2px; font-size: 12px; }
.fm b { font-size: 14px; }
.fm.on { border-color: var(--accent); background: var(--accent-soft); }
</style>
