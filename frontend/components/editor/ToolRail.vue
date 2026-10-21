<script setup lang="ts">
// Tool rail melayang di kiri kanvas: alat pilih/geser + pintasan ke panel alat (AI, template, elemen, …).
const props = defineProps<{ tool: 'select' | 'hand'; panel: string }>()
const emit = defineEmits<{ 'update:tool': ['select' | 'hand']; open: [string] }>()
const editor = useEditor()
const st = editor.state
const more = ref(false)
const showKeys = ref(false)
const root = ref<HTMLElement>()

const MAIN = [
  { key: 'ai', label: 'AI Magic', icon: 'sparkles' },
  { key: 'templates', label: 'Template', icon: 'template' },
  { key: 'elements', label: 'Elemen', icon: 'shapes' },
  { key: 'text', label: 'Teks', icon: 'text' },
  { key: 'uploads', label: 'Upload', icon: 'upload' },
]
const MORE = [
  { key: 'draw', label: 'Gambar bebas', icon: 'brush' },
  { key: 'background', label: 'Latar belakang', icon: 'palette' },
  { key: 'layers', label: 'Layer', icon: 'layers' },
  { key: 'brand', label: 'Brand Kit', icon: 'brand' },
]
const moreActive = computed(() => MORE.some((m) => m.key === props.panel))

function pick(key: string) {
  more.value = false
  emit('open', key)
}
function onDoc(e: MouseEvent) {
  if (more.value && root.value && !root.value.contains(e.target as Node)) more.value = false
}
onMounted(() => document.addEventListener('mousedown', onDoc))
onBeforeUnmount(() => document.removeEventListener('mousedown', onDoc))
</script>

<template>
  <nav ref="root" class="rail">
    <button class="it" :class="{ on: tool === 'select' }" title="Pilih (V)" @click="emit('update:tool', 'select')"><Icon name="pointer" :size="21" /></button>
    <button class="it" :class="{ on: tool === 'hand' }" title="Geser board (H, atau tahan Spasi)" @click="emit('update:tool', 'hand')"><Icon name="hand" :size="21" /></button>
    <span class="sp" />
    <button v-for="t in MAIN" :key="t.key" class="it" :class="{ cur: panel === t.key }" :title="t.label" @click="pick(t.key)"><Icon :name="t.icon" :size="20" /></button>
    <div class="wrap">
      <button class="it" :class="{ cur: moreActive || more }" title="Lainnya" @click="more = !more"><Icon name="more" :size="20" /></button>
      <div v-if="more" class="popover menu mp">
        <button v-for="m in MORE" :key="m.key" class="menu-item" @click="pick(m.key)"><Icon :name="m.icon" :size="15" /> {{ m.label }}</button>
        <div class="sep" style="margin: 4px 0" />
        <button class="menu-item" :disabled="!st.canUndo" @click="editor.undo()"><Icon name="undo" :size="15" /> Undo <span class="kbd">Ctrl+Z</span></button>
        <button class="menu-item" :disabled="!st.canRedo" @click="editor.redo()"><Icon name="redo" :size="15" /> Redo <span class="kbd">Ctrl+Y</span></button>
        <button class="menu-item" @click="more = false; showKeys = true"><Icon name="keyboard" :size="15" /> Pintasan keyboard</button>
      </div>
    </div>
    <!-- dibungkus .pub agar dialog di luar editor tetap memakai token warna halaman awal -->
    <Teleport to="body"><div class="pub tp"><EditorShortcutsDialog v-if="showKeys" @close="showKeys = false" /></div></Teleport>
  </nav>
</template>

<style scoped>
.rail { display: flex; flex-direction: column; align-items: center; gap: 4px; padding: 4px; background: var(--panel); border: 1px solid var(--line); border-radius: 10px; box-shadow: var(--shadow); }
.it { width: 42px; height: 42px; border: 0; border-radius: 7px; background: none; display: grid; place-items: center; cursor: pointer; color: var(--ink-2); }
.it:hover { background: var(--panel-2); color: var(--ink); }
.it.on { background: var(--ink); color: #fff; }
.it.cur { background: var(--panel-3); color: var(--ink); }
.sp { width: 24px; height: 1px; background: var(--line); margin: 2px 0; }
.wrap { position: relative; }
.mp { left: calc(100% + 12px); bottom: 0; }
.menu-item:disabled { opacity: .4; cursor: default; }
@media (max-height: 640px), (max-width: 640px) { .it { width: 38px; height: 38px; } }
.tp { display: contents; }
</style>
