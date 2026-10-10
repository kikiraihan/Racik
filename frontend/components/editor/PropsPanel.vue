<script setup lang="ts">
// Panel properti kanan: posisi/ukuran presisi, rotasi, transparansi, bayangan, dan filter gambar.
import { DEFAULT_FX } from '~/lib/editor'

const emit = defineEmits<{ close: [] }>()
const editor = useEditor()
const s = computed(() => editor.state.sel)

const geo = (k: 'left' | 'top' | 'width' | 'height' | 'angle', e: Event) => editor.setGeometry({ [k]: +(e.target as HTMLInputElement).value })
const shadowOn = computed(() => !!s.value.shadow)
function setShadow(patch: Partial<NonNullable<typeof s.value.shadow>>) {
  const base = s.value.shadow || { color: 'rgba(0,0,0,0.35)', blur: 18, offsetX: 6, offsetY: 6 }
  editor.setShadow({ ...base, ...patch })
}

const SLIDERS = [
  { k: 'brightness', label: 'Kecerahan', min: -1, max: 1, step: 0.01 },
  { k: 'contrast', label: 'Kontras', min: -1, max: 1, step: 0.01 },
  { k: 'saturation', label: 'Saturasi', min: -1, max: 1, step: 0.01 },
  { k: 'hue', label: 'Rona (hue)', min: -1, max: 1, step: 0.01 },
  { k: 'blur', label: 'Blur', min: 0, max: 1, step: 0.01 },
  { k: 'noise', label: 'Noise', min: 0, max: 400, step: 5 },
  { k: 'pixelate', label: 'Pixelate', min: 0, max: 40, step: 1 },
] as const
const TOGGLES = [
  { k: 'grayscale', label: 'Hitam putih' }, { k: 'sepia', label: 'Sepia' }, { k: 'vintage', label: 'Vintage' }, { k: 'invert', label: 'Invert' },
] as const
const PRESETS: Record<string, Partial<typeof DEFAULT_FX>> = {
  Normal: { ...DEFAULT_FX },
  Cerah: { ...DEFAULT_FX, brightness: 0.12, saturation: 0.25, contrast: 0.08 },
  Dramatis: { ...DEFAULT_FX, contrast: 0.35, saturation: -0.2, brightness: -0.05 },
  Hangat: { ...DEFAULT_FX, hue: 0.04, saturation: 0.2, brightness: 0.05 },
  Dingin: { ...DEFAULT_FX, hue: -0.08, saturation: -0.1 },
  Noir: { ...DEFAULT_FX, grayscale: true, contrast: 0.3 },
  Retro: { ...DEFAULT_FX, vintage: true, noise: 60 },
}
</script>

<template>
  <div class="pp">
    <div class="row hd"><b class="grow">Properti</b><button class="btn ghost sm icon" @click="emit('close')"><Icon name="x" :size="16" /></button></div>

    <section>
      <span class="label">Posisi & ukuran</span>
      <div class="grid2">
        <label class="f"><span>X</span><input class="input sm" type="number" :value="s.left" @change="geo('left', $event)" /></label>
        <label class="f"><span>Y</span><input class="input sm" type="number" :value="s.top" @change="geo('top', $event)" /></label>
        <label class="f"><span>L</span><input class="input sm" type="number" :value="s.width" @change="geo('width', $event)" /></label>
        <label class="f"><span>T</span><input class="input sm" type="number" :value="s.height" :disabled="s.kind === 'text'" @change="geo('height', $event)" /></label>
        <label class="f"><span>⟳</span><input class="input sm" type="number" :value="s.angle" @change="geo('angle', $event)" /></label>
      </div>
      <div class="row al">
        <button class="btn sm icon" title="Rata kiri" @click="editor.align('left')"><Icon name="alignLeft" :size="15" /></button>
        <button class="btn sm icon" title="Tengah" @click="editor.align('center')"><Icon name="alignCenter" :size="15" /></button>
        <button class="btn sm icon" title="Rata kanan" @click="editor.align('right')"><Icon name="alignRight" :size="15" /></button>
        <button class="btn sm icon" title="Atas" @click="editor.align('top')"><Icon name="alignTop" :size="15" /></button>
        <button class="btn sm icon" title="Tengah vertikal" @click="editor.align('middle')"><Icon name="alignMiddle" :size="15" /></button>
        <button class="btn sm icon" title="Bawah" @click="editor.align('bottom')"><Icon name="alignBottom" :size="15" /></button>
      </div>
    </section>

    <section>
      <span class="label">Transparansi: {{ Math.round(s.opacity * 100) }}%</span>
      <input type="range" min="0" max="1" step="0.01" :value="s.opacity" @input="editor.set({ opacity: +($event.target as HTMLInputElement).value })" />
    </section>

    <section v-if="s.kind !== 'multi'">
      <label class="row"><input type="checkbox" :checked="shadowOn" @change="($event.target as HTMLInputElement).checked ? setShadow({}) : editor.setShadow(null)" /> <b>Bayangan</b></label>
      <template v-if="s.shadow">
        <div class="row" style="margin-top: 8px"><span class="grow small">Warna</span><ColorPicker :model-value="s.shadow.color" @update:model-value="(c) => setShadow({ color: c || 'rgba(0,0,0,0.35)' })" /></div>
        <label class="small">Blur {{ s.shadow.blur }}<input type="range" min="0" max="100" :value="s.shadow.blur" @input="setShadow({ blur: +($event.target as HTMLInputElement).value })" /></label>
        <label class="small">Geser X {{ s.shadow.offsetX }}<input type="range" min="-100" max="100" :value="s.shadow.offsetX" @input="setShadow({ offsetX: +($event.target as HTMLInputElement).value })" /></label>
        <label class="small">Geser Y {{ s.shadow.offsetY }}<input type="range" min="-100" max="100" :value="s.shadow.offsetY" @input="setShadow({ offsetY: +($event.target as HTMLInputElement).value })" /></label>
      </template>
    </section>

    <section v-if="s.kind === 'image'">
      <span class="label">Filter cepat</span>
      <div class="pres">
        <button v-for="(fx, name) in PRESETS" :key="name" class="chip" @click="editor.setFx(fx)">{{ name }}</button>
      </div>
      <span class="label" style="margin-top: 12px">Penyesuaian</span>
      <label v-for="sl in SLIDERS" :key="sl.k" class="small sl">
        <span class="row"><span class="grow">{{ sl.label }}</span><span class="muted">{{ s.fx[sl.k] }}</span></span>
        <input type="range" :min="sl.min" :max="sl.max" :step="sl.step" :value="s.fx[sl.k]" @change="editor.setFx({ [sl.k]: +($event.target as HTMLInputElement).value })" />
      </label>
      <div class="grid2" style="margin-top: 6px">
        <label v-for="t in TOGGLES" :key="t.k" class="row small"><input type="checkbox" :checked="s.fx[t.k]" @change="editor.setFx({ [t.k]: ($event.target as HTMLInputElement).checked })" /> {{ t.label }}</label>
      </div>
      <button class="btn sm" style="margin-top: 10px; width: 100%" @click="editor.setFx({ ...DEFAULT_FX })">Reset filter</button>
    </section>
  </div>
</template>

<style scoped>
.pp { padding: 0 14px 20px; }
.hd { position: sticky; top: 0; background: #fff; padding: 12px 0; z-index: 2; }
section { padding: 12px 0; border-bottom: 1px solid var(--line); display: flex; flex-direction: column; gap: 6px; }
.f { display: flex; align-items: center; gap: 6px; font-size: 12px; color: var(--muted); }
.f span { width: 14px; }
.al { gap: 4px; margin-top: 6px; }
.small { font-size: 12px; }
.sl { display: flex; flex-direction: column; gap: 2px; }
.pres { display: flex; flex-wrap: wrap; gap: 6px; }
.pres .chip { height: 26px; font-size: 12px; }
</style>
