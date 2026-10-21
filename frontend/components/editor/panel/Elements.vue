<script setup lang="ts">
import { GRAPHIC_ICONS, iconSvg } from '~/lib/icons'

const editor = useEditor()
const q = ref('')
const iconColor = ref('#111111')

const SHAPES = [
  { k: 'rect', svg: '<rect x="4" y="4" width="40" height="40"/>' },
  { k: 'rounded', svg: '<rect x="4" y="4" width="40" height="40" rx="9"/>' },
  { k: 'circle', svg: '<circle cx="24" cy="24" r="20"/>' },
  { k: 'triangle', svg: '<path d="M24 4 L44 44 L4 44 Z"/>' },
  { k: 'star', svg: '<path d="M24 3l6 14 15 1-11.5 10 3.5 15L24 35l-13 8 3.5-15L3 18l15-1z"/>' },
  { k: 'pentagon', svg: '<path d="M24 3 L45 18 L37 44 L11 44 L3 18 Z"/>' },
  { k: 'hexagon', svg: '<path d="M24 3 L43 13.5 L43 34.5 L24 45 L5 34.5 L5 13.5 Z"/>' },
  { k: 'diamond', svg: '<path d="M24 3 L45 24 L24 45 L3 24 Z"/>' },
  { k: 'heart', svg: '<path d="M24 43 C10 33 3 25 3 15 C3 8 8 3 15 3 C19 3 22 5 24 9 C26 5 29 3 33 3 C40 3 45 8 45 15 C45 25 38 33 24 43 Z"/>' },
  { k: 'arrow', svg: '<path d="M3 18 H29 V7 L45 24 L29 41 V30 H3 Z"/>' },
  { k: 'speech', svg: '<path d="M7 4 H41 Q45 4 45 8 V30 Q45 34 41 34 H20 L10 44 L12 34 H7 Q3 34 3 30 V8 Q3 4 7 4 Z"/>' },
]
const LINES = [
  { k: 'line', svg: '<line x1="4" y1="24" x2="44" y2="24" stroke-width="4"/>' },
  { k: 'dashed', svg: '<line x1="4" y1="24" x2="44" y2="24" stroke-width="4" stroke-dasharray="8 5"/>' },
  { k: 'line-arrow', svg: '<path d="M4 24 H42 M34 16 L42 24 L34 32" stroke-width="4" fill="none"/>' },
]
const EMOJI = '😀 😍 🤩 😎 🥳 🔥 ✨ 🎉 💯 ❤️ 👍 🙏 👉 ✅ ⭐ 🌟 💡 📌 📣 🎯 🚀 🏆 💰 🛒 🎁 ☕ 🍔 🍕 🌿 🌸 🌈 ☀️ 🌙 📚 🎓 💻 📱 📷 🎵 ⚽'.split(' ')

const icons = computed(() => Object.entries(GRAPHIC_ICONS).filter(([k]) => !q.value || k.includes(q.value.toLowerCase())))
</script>

<template>
  <div>
    <input v-model="q" class="input sm" placeholder="Cari ikon (heart, rocket, chart…)" />
    <h4 style="margin-top: 14px">Bentuk</h4>
    <div class="g">
      <button v-for="s in SHAPES" :key="s.k" class="cell" :title="s.k" @click="editor.addShape(s.k)">
        <svg viewBox="0 0 48 48" fill="#8B3DFF" v-html="s.svg" />
      </button>
    </div>
    <h4>Garis</h4>
    <div class="g">
      <button v-for="s in LINES" :key="s.k" class="cell" :title="s.k" @click="editor.addShape(s.k)">
        <svg viewBox="0 0 48 48" stroke="#111" fill="none" v-html="s.svg" />
      </button>
    </div>
    <h4>Bingkai foto</h4>
    <div class="g">
      <button class="cell" title="Bingkai persegi" @click="editor.addFrame('rect')"><div class="fr" /></button>
      <button class="cell" title="Bingkai tumpul" @click="editor.addFrame('rounded')"><div class="fr" style="border-radius: 8px; aspect-ratio: 1" /></button>
      <button class="cell" title="Bingkai lingkaran" @click="editor.addFrame('circle')"><div class="fr" style="border-radius: 50%; aspect-ratio: 1" /></button>
    </div>
    <div class="row" style="margin-top: 16px"><h4 class="grow" style="margin: 0">Ikon</h4><ColorPicker v-model="iconColor" :size="22" /></div>
    <div class="g" style="margin-top: 8px">
      <button v-for="[k, d] in icons" :key="k" class="cell" :title="k" @click="editor.addSvg(iconSvg(d, iconColor), 'Ikon ' + k)">
        <svg viewBox="0 0 24 24" width="28" height="28" fill="none" :stroke="iconColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path :d="d" /></svg>
      </button>
    </div>
    <h4>Emoji</h4>
    <div class="em">
      <button v-for="e in EMOJI" :key="e" @click="editor.addEmoji(e)">{{ e }}</button>
    </div>
  </div>
</template>

<style scoped>
.g { display: grid; grid-template-columns: repeat(4, 1fr); gap: 8px; }
.cell { aspect-ratio: 1; border: 1px solid var(--line); background: var(--panel-2); border-radius: 10px; cursor: pointer; display: grid; place-items: center; padding: 12px; }
.cell:hover { border-color: var(--line-2); background: var(--panel); }
.cell svg { width: 100%; height: 100%; }
.fr { width: 100%; aspect-ratio: 4/3; background: #e4e4e7; border: 2px dashed #a1a1aa; }
.em { display: grid; grid-template-columns: repeat(7, 1fr); gap: 4px; }
.em button { font-size: 22px; border: 0; background: none; cursor: pointer; border-radius: 8px; padding: 4px 0; }
.em button:hover { background: var(--panel-2); }
</style>
