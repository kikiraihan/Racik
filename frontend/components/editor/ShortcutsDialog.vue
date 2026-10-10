<script setup lang="ts">
// Daftar pintasan keyboard editor (dipindah dari topbar lama).
const emit = defineEmits<{ close: [] }>()
const KEYS = [
  ['V / H', 'Alat pilih / alat geser (tahan Spasi untuk menggeser board)'],
  ['Ctrl + Z / Ctrl + Y', 'Undo / Redo'], ['Ctrl + C / X / V', 'Salin / Potong / Tempel (juga gambar)'], ['Ctrl + D', 'Duplikat'],
  ['Ctrl + G / Ctrl + Shift + G', 'Grup / Pisah grup'], ['Ctrl + A', 'Pilih semua'], ['Ctrl + L', 'Kunci / buka kunci'],
  ['Ctrl + ] / [', 'Maju / mundur satu layer (+Alt: paling depan/belakang)'], ['Delete', 'Hapus'], ['Panah (+Shift)', 'Geser 1px (10px)'],
  ['T / R / C / L', 'Tambah teks / persegi / lingkaran / garis'], ['Scroll', 'Geser board'], ['Ctrl + scroll, Ctrl + + / -', 'Zoom'],
  ['Ctrl + 0', 'Pas ke layar'], ['Ctrl + S', 'Simpan'], ['Ctrl + \\', 'Buka / tutup sidebar'],
]
</script>

<template>
  <div class="modal-back" @click.self="emit('close')">
    <div class="modal">
      <h3>Pintasan keyboard</h3>
      <table class="keys">
        <tr v-for="[k, v] in KEYS" :key="k"><td><kbd>{{ k }}</kbd></td><td>{{ v }}</td></tr>
      </table>
    </div>
  </div>
</template>

<style scoped>
.keys { width: 100%; border-collapse: collapse; color: var(--ink); }
.keys td { padding: 6px 4px; border-bottom: 1px solid var(--line); font-size: 13px; }
kbd { background: var(--panel-2); border: 1px solid var(--line); border-radius: 6px; padding: 2px 6px; font-family: inherit; font-size: 12px; white-space: nowrap; }
</style>
