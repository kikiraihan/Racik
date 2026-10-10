<script setup lang="ts">
const editor = useEditor()
const st = editor.state
const COLORS = ['#111111', '#FF3131', '#FFDE59', '#00BF63', '#38B6FF', '#8B3DFF', '#FF66C4', '#FFFFFF']
onBeforeUnmount(() => editor.setDrawing(false))
</script>

<template>
  <div class="col">
    <button class="btn" :class="st.drawing ? 'active' : 'primary'" @click="editor.setDrawing(!st.drawing)">
      <Icon name="brush" :size="16" /> {{ st.drawing ? 'Selesai menggambar' : 'Mulai menggambar' }}
    </button>
    <span class="label" style="margin-top: 8px">Warna kuas</span>
    <div class="row" style="flex-wrap: wrap">
      <button v-for="c in COLORS" :key="c" class="dot" :class="{ on: st.brushColor === c }" :style="{ background: c }" @click="editor.setBrush(c)" />
      <ColorPicker :model-value="st.brushColor" @update:model-value="(c) => editor.setBrush(c)" />
    </div>
    <span class="label" style="margin-top: 8px">Ketebalan: {{ st.brushWidth }}px</span>
    <input type="range" min="1" max="80" :value="st.brushWidth" @input="editor.setBrush(undefined, +($event.target as HTMLInputElement).value)" />
    <p class="muted" style="font-size: 12px">Setiap goresan menjadi layer tersendiri yang bisa dipindah, diwarnai ulang, atau dihapus.</p>
  </div>
</template>

<style scoped>
.dot { width: 28px; height: 28px; border-radius: 50%; border: 2px solid #fff; box-shadow: 0 0 0 1px var(--line); cursor: pointer; }
.dot.on { box-shadow: 0 0 0 2px var(--accent); }
</style>
