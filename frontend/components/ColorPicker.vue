<script setup lang="ts">
// Pemilih warna: swatch default, warna brand, warna di dokumen, input hex & gradien.
import { DEFAULT_COLORS } from '~/lib/presets'

const props = withDefaults(defineProps<{
  modelValue: string
  gradient?: string[] | null
  allowGradient?: boolean
  title?: string
  size?: number
}>(), { gradient: null, allowGradient: false, title: 'Warna', size: 28 })
const emit = defineEmits<{ 'update:modelValue': [string]; gradient: [string[] | null] }>()

const open = ref(false)
const root = ref<HTMLElement>()
const pos = ref({ top: '0px', left: '0px' })

// Popover diposisikan fixed agar tidak terpotong container yang overflow (toolbar, panel).
function toggleOpen() {
  open.value = !open.value
  if (!open.value || !root.value) return
  const r = root.value.getBoundingClientRect()
  const top = r.bottom + 360 > window.innerHeight ? Math.max(8, r.top - 370) : r.bottom + 6
  pos.value = { top: top + 'px', left: Math.max(8, Math.min(r.left, window.innerWidth - 260)) + 'px' }
}
const brand = useState<string[]>('brandColors', () => [])
const hex = ref(props.modelValue)
watch(() => props.modelValue, (v) => (hex.value = v))

const GRADIENTS = [
  ['#00C4CC', '#7D2AE8'], ['#FF5F6D', '#FFC371'], ['#11998E', '#38EF7D'], ['#FC466B', '#3F5EFB'],
  ['#F7971E', '#FFD200'], ['#0F2027', '#2C5364'], ['#EE9CA7', '#FFDDE1'], ['#8E2DE2', '#4A00E0'],
]

function pick(c: string) {
  emit('update:modelValue', c)
  if (props.gradient) emit('gradient', null)
}
function onHex() {
  if (/^#([0-9a-f]{3}|[0-9a-f]{6})$/i.test(hex.value)) pick(hex.value)
}
function onDoc(e: MouseEvent) {
  if (open.value && root.value && !root.value.contains(e.target as Node)) open.value = false
}
onMounted(() => document.addEventListener('mousedown', onDoc))
onBeforeUnmount(() => document.removeEventListener('mousedown', onDoc))

const swatchStyle = computed(() => ({
  width: props.size + 'px', height: props.size + 'px',
  background: props.gradient ? `linear-gradient(135deg, ${props.gradient.join(',')})` : props.modelValue || 'transparent',
}))
</script>

<template>
  <div ref="root" class="cp">
    <button class="sw" :style="swatchStyle" :title="title" @click="toggleOpen" :class="{ empty: !modelValue && !gradient }" />
    <div v-if="open" class="popover pop" :style="pos">
      <div class="label">Warna default</div>
      <div class="grid">
        <button v-for="c in DEFAULT_COLORS" :key="c" class="cell" :style="{ background: c }" :title="c" @click="pick(c)" />
      </div>
      <template v-if="brand.length">
        <div class="label" style="margin-top: 10px">Warna brand</div>
        <div class="grid">
          <button v-for="c in brand" :key="c" class="cell" :style="{ background: c }" :title="c" @click="pick(c)" />
        </div>
      </template>
      <template v-if="allowGradient">
        <div class="label" style="margin-top: 10px">Gradien</div>
        <div class="grid">
          <button v-for="g in GRADIENTS" :key="g.join()" class="cell" :style="{ background: `linear-gradient(135deg, ${g.join(',')})` }" @click="emit('gradient', g)" />
        </div>
      </template>
      <div class="row" style="margin-top: 10px">
        <input type="color" :value="modelValue?.startsWith('#') ? modelValue : '#000000'" @input="pick(($event.target as HTMLInputElement).value)" />
        <input v-model="hex" class="input sm grow" placeholder="#000000" @change="onHex" @keyup.enter="onHex" />
        <button class="btn sm" title="Transparan" @click="pick('')">∅</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.cp { position: relative; display: inline-block; }
.sw { border-radius: 6px; border: 1px solid rgba(0,0,0,.15); cursor: pointer; padding: 0; display: block; }
/* padding/border/radius diulang agar tidak tertimpa .pub .empty (kotak kosong halaman public) */
.sw.empty { background: repeating-conic-gradient(#ddd 0 25%, #fff 0 50%) 0 0/10px 10px !important; padding: 0; border: 1px solid rgba(0,0,0,.15); border-radius: 6px; }
.pop { width: 244px; position: fixed; z-index: 120; }
.grid { display: grid; grid-template-columns: repeat(6, 1fr); gap: 6px; }
.cell { aspect-ratio: 1; border-radius: 6px; border: 1px solid rgba(0,0,0,.12); cursor: pointer; padding: 0; }
.cell:hover { transform: scale(1.08); }
</style>
