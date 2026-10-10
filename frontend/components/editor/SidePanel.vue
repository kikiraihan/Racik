<script setup lang="ts">
defineProps<{ tab: string }>()
const emit = defineEmits<{ close: [] }>()
const TITLES: Record<string, string> = {
  ai: 'AI Magic', templates: 'Template', elements: 'Elemen', text: 'Teks', uploads: 'Upload',
  draw: 'Gambar bebas', background: 'Latar belakang', layers: 'Layer', brand: 'Brand Kit',
}
</script>

<template>
  <div class="sp">
    <div class="hd row">
      <b class="grow">{{ TITLES[tab] }}</b>
      <button class="btn ghost sm icon" title="Tutup panel" @click="emit('close')"><Icon name="x" :size="16" /></button>
    </div>
    <div class="ct">
      <EditorPanelAi v-if="tab === 'ai'" />
      <EditorPanelTemplates v-else-if="tab === 'templates'" />
      <EditorPanelElements v-else-if="tab === 'elements'" />
      <EditorPanelText v-else-if="tab === 'text'" />
      <EditorPanelUploads v-else-if="tab === 'uploads'" />
      <EditorPanelDraw v-else-if="tab === 'draw'" />
      <EditorPanelBackground v-else-if="tab === 'background'" />
      <EditorPanelLayers v-else-if="tab === 'layers'" />
      <EditorPanelBrand v-else-if="tab === 'brand'" />
    </div>
  </div>
</template>

<style scoped>
.hd { position: sticky; top: 0; background: #fff; padding: 12px 14px; border-bottom: 1px solid var(--line); z-index: 2; font-size: 15px; }
.ct { padding: 14px; }
.ct :deep(h4) { margin: 16px 0 8px; font-size: 13px; font-weight: 700; }
.ct :deep(h4:first-child) { margin-top: 0; }
</style>
