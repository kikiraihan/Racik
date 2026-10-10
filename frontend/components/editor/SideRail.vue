<script setup lang="ts">
defineProps<{ tab: string; open: boolean }>()
const emit = defineEmits<{ select: [string] }>()

const TABS = [
  { key: 'ai', label: 'AI Magic', icon: 'sparkles' },
  { key: 'templates', label: 'Template', icon: 'template' },
  { key: 'elements', label: 'Elemen', icon: 'shapes' },
  { key: 'text', label: 'Teks', icon: 'text' },
  { key: 'uploads', label: 'Upload', icon: 'upload' },
  { key: 'draw', label: 'Gambar', icon: 'brush' },
  { key: 'background', label: 'Latar', icon: 'palette' },
  { key: 'layers', label: 'Layer', icon: 'layers' },
  { key: 'brand', label: 'Brand', icon: 'brand' },
]
</script>

<template>
  <nav class="rail">
    <button v-for="t in TABS" :key="t.key" class="it" :class="{ on: open && tab === t.key, ai: t.key === 'ai' }" @click="emit('select', t.key)">
      <span class="ic"><Icon :name="t.icon" :size="20" /></span>
      <span class="lb">{{ t.label }}</span>
    </button>
  </nav>
</template>

<style scoped>
.rail { width: 72px; flex: 0 0 72px; background: #fff; border-right: 1px solid var(--line); display: flex; flex-direction: column; padding: 6px 0; overflow-y: auto; }
.it { border: 0; background: none; display: flex; flex-direction: column; align-items: center; gap: 4px; padding: 9px 0; cursor: pointer; color: var(--ink-2); font-size: 11px; font-weight: 500; }
.ic { width: 40px; height: 32px; display: grid; place-items: center; border-radius: 9px; }
.it:hover .ic { background: var(--panel-2); }
.it.on { color: var(--accent); }
.it.on .ic { background: var(--accent-soft); }
.it.ai .ic { background: var(--grad); color: #fff; }
@media (max-width: 900px) { .rail { width: 64px; flex-basis: 64px; } }
</style>
