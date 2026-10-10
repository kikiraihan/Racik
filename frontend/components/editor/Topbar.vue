<script setup lang="ts">
const emit = defineEmits<{ save: []; fit: [] }>()
const editor = useEditor()
const doc = useDoc()
const st = editor.state
const showExport = ref(false)
const showResize = ref(false)
const showKeys = ref(false)

const status = computed(() => (doc.saving ? 'Menyimpan…' : doc.dirty ? 'Belum tersimpan' : doc.savedAt ? 'Tersimpan' : ''))
</script>

<template>
  <header class="top">
    <NuxtLink to="/" class="btn ghost icon home" title="Beranda"><Icon name="home" /></NuxtLink>
    <span class="logo">Racik</span>
    <div class="vsep" />
    <button class="btn ghost sm" @click="showResize = true"><Icon name="resize" :size="16" /> Ubah ukuran</button>
    <button class="btn ghost sm icon" :disabled="!st.canUndo" title="Undo (Ctrl+Z)" @click="editor.undo()"><Icon name="undo" :size="16" /></button>
    <button class="btn ghost sm icon" :disabled="!st.canRedo" title="Redo (Ctrl+Y)" @click="editor.redo()"><Icon name="redo" :size="16" /></button>
    <span class="status">{{ status }}</span>
    <div class="grow" />
    <input v-model="doc.title" class="title" placeholder="Desain tanpa judul" />
    <div class="grow" />
    <button class="btn ghost sm icon" title="Pintasan keyboard" @click="showKeys = true"><Icon name="keyboard" :size="16" /></button>
    <button class="btn sm white" @click="emit('save')"><Icon name="check" :size="16" /> Simpan</button>
    <button class="btn sm primary" @click="showExport = true"><Icon name="download" :size="16" /> Download</button>
    <EditorExportDialog v-if="showExport" :title="doc.title" @close="showExport = false" />
    <EditorResizeDialog v-if="showResize" @close="showResize = false; emit('fit')" />
    <div v-if="showKeys" class="modal-back" @click.self="showKeys = false">
      <div class="modal">
        <h3>Pintasan keyboard</h3>
        <table class="keys">
          <tr v-for="[k, v] in [
            ['Ctrl + Z / Ctrl + Y', 'Undo / Redo'], ['Ctrl + C / X / V', 'Salin / Potong / Tempel (juga gambar)'], ['Ctrl + D', 'Duplikat'],
            ['Ctrl + G / Ctrl + Shift + G', 'Grup / Pisah grup'], ['Ctrl + A', 'Pilih semua'], ['Ctrl + L', 'Kunci / buka kunci'],
            ['Ctrl + ] / [', 'Maju / mundur satu layer (+Alt: paling depan/belakang)'], ['Delete', 'Hapus'], ['Panah (+Shift)', 'Geser 1px (10px)'],
            ['T / R / C / L', 'Tambah teks / persegi / lingkaran / garis'], ['Ctrl + scroll, Ctrl + + / -', 'Zoom'], ['Ctrl + 0', 'Pas ke layar'], ['Ctrl + S', 'Simpan'],
          ]" :key="k"><td><kbd>{{ k }}</kbd></td><td>{{ v }}</td></tr>
        </table>
      </div>
    </div>
  </header>
</template>

<style scoped>
.top { height: 56px; flex: 0 0 56px; display: flex; align-items: center; gap: 6px; padding: 0 12px; background: var(--grad); color: #fff; }
.top .btn.ghost { color: #fff; }
.top .btn.ghost:hover { background: rgba(255,255,255,.15); }
.top .btn.ghost:disabled { opacity: .45; }
.logo { font-weight: 800; font-size: 18px; letter-spacing: -.02em; }
.vsep { background: rgba(255,255,255,.35); }
.status { font-size: 12px; opacity: .85; margin-left: 6px; min-width: 100px; }
.title { background: transparent; border: 1px solid transparent; color: #fff; font-weight: 600; text-align: center; padding: 6px 10px; border-radius: 8px; width: min(360px, 30vw); outline: none; }
.title:hover, .title:focus { border-color: rgba(255,255,255,.5); background: rgba(255,255,255,.1); }
.title::placeholder { color: rgba(255,255,255,.7); }
.white { background: #fff; color: var(--ink); border: 0; }
.keys { width: 100%; border-collapse: collapse; color: var(--ink); }
.keys td { padding: 6px 4px; border-bottom: 1px solid var(--line); font-size: 13px; }
kbd { background: var(--panel-2); border: 1px solid var(--line); border-radius: 6px; padding: 2px 6px; font-family: inherit; font-size: 12px; }
@media (max-width: 900px) { .status, .logo, .vsep { display: none; } }
</style>
