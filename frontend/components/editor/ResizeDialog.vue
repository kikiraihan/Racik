<script setup lang="ts">
import { SIZE_PRESETS } from '~/lib/presets'

const emit = defineEmits<{ close: [] }>()
const editor = useEditor()
const w = ref(editor.state.width)
const h = ref(editor.state.height)
const scaleContent = ref(true)

async function apply() {
  await editor.resize(Math.max(50, w.value), Math.max(50, h.value), scaleContent.value)
  emit('close')
}
</script>

<template>
  <div class="modal-back" @click.self="emit('close')">
    <div class="modal" style="color: var(--ink)">
      <h3>Ubah ukuran (Magic Resize)</h3>
      <div class="list scroll">
        <button v-for="p in SIZE_PRESETS" :key="p.key" class="it" :class="{ on: w === p.width && h === p.height }" @click="w = p.width; h = p.height">
          <span class="grow">{{ p.label }}</span><span class="muted">{{ p.width }}×{{ p.height }}</span>
        </button>
      </div>
      <div class="grid2" style="margin-top: 12px">
        <label><span class="label">Lebar</span><input v-model.number="w" class="input" type="number" /></label>
        <label><span class="label">Tinggi</span><input v-model.number="h" class="input" type="number" /></label>
      </div>
      <label class="row" style="margin-top: 10px"><input v-model="scaleContent" type="checkbox" /> Skalakan isi desain secara proporsional</label>
      <div class="row" style="justify-content: flex-end; margin-top: 16px">
        <button class="btn" @click="emit('close')">Batal</button>
        <button class="btn primary" @click="apply">Ubah ukuran</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.list { max-height: 260px; display: flex; flex-direction: column; gap: 4px; }
.it { display: flex; gap: 8px; padding: 8px 10px; border: 1px solid var(--line); border-radius: 8px; background: #fff; cursor: pointer; font-size: 13px; text-align: left; }
.it.on { border-color: var(--accent); background: var(--accent-soft); }
</style>
