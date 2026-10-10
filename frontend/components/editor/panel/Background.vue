<script setup lang="ts">
import { DEFAULT_COLORS } from '~/lib/presets'

const editor = useEditor()
const api = useApi()
const st = editor.state
const angle = ref(135)
const GRADS = [
  ['#00C4CC', '#7D2AE8'], ['#FF5F6D', '#FFC371'], ['#11998E', '#38EF7D'], ['#FC466B', '#3F5EFB'], ['#F7971E', '#FFD200'],
  ['#0F2027', '#2C5364'], ['#EE9CA7', '#FFDDE1'], ['#8E2DE2', '#4A00E0'], ['#1D2671', '#C33764'], ['#FDFBFB', '#EBEDEE'],
  ['#E0C3FC', '#8EC5FC'], ['#43E97B', '#38F9D7'],
]
async function uploadBg(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0]
  if (!f) return
  const a = await api.upload(f)
  await editor.setBackgroundImage(a.url)
}
</script>

<template>
  <div class="col">
    <span class="label">Warna</span>
    <div class="row">
      <ColorPicker :model-value="st.background" :gradient="st.backgroundGradient" allow-gradient :size="36"
        @update:model-value="(c) => editor.setBackgroundColor(c || '#ffffff')" @gradient="(g) => g && editor.setBackgroundGradient(g, angle)" />
      <span class="muted">Warna kustom</span>
    </div>
    <div class="g">
      <button v-for="c in DEFAULT_COLORS" :key="c" class="c" :style="{ background: c }" @click="editor.setBackgroundColor(c)" />
    </div>
    <span class="label" style="margin-top: 10px">Gradien</span>
    <div class="g">
      <button v-for="g in GRADS" :key="g.join()" class="c" :style="{ background: `linear-gradient(${angle}deg, ${g.join(',')})` }" @click="editor.setBackgroundGradient(g, angle)" />
    </div>
    <label><span class="label">Arah gradien: {{ angle }}°</span>
      <input v-model.number="angle" type="range" min="0" max="360" @change="st.backgroundGradient && editor.setBackgroundGradient(st.backgroundGradient, angle)" /></label>
    <span class="label" style="margin-top: 10px">Gambar latar</span>
    <label class="btn"><Icon name="upload" :size="16" /> Upload gambar latar<input type="file" accept="image/*" hidden @change="uploadBg" /></label>
    <button v-if="st.sel.kind === 'image'" class="btn" @click="editor.activeToBackground()"><Icon name="image" :size="16" /> Jadikan gambar terpilih sebagai latar</button>
    <p class="muted" style="font-size: 12px">Atau buat latar dengan AI di tab <b>AI Magic → Gambar AI</b>, lalu pilih “Jadikan latar” di panel Upload.</p>
  </div>
</template>

<style scoped>
.g { display: grid; grid-template-columns: repeat(6, 1fr); gap: 6px; }
.c { aspect-ratio: 1; border-radius: 8px; border: 1px solid rgba(0,0,0,.12); cursor: pointer; }
.c:hover { transform: scale(1.06); }
</style>
