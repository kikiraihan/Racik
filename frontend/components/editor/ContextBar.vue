<script setup lang="ts">
// Toolbar kontekstual di atas kanvas, berubah sesuai jenis objek terpilih (seperti Canva).
import { FONTS } from '~/lib/presets'
import type { Asset } from '~/composables/useApi'

const emit = defineEmits<{ 'toggle-props': []; 'open-props': [] }>()
const editor = useEditor()
const api = useApi()
const toast = useToast()
const st = editor.state
const s = computed(() => st.sel)
const pop = ref('')
const busy = ref(false)
const root = ref<HTMLElement>()

function toggle(p: string) {
  pop.value = pop.value === p ? '' : p
}
function onDoc(e: MouseEvent) {
  if (pop.value && root.value && !root.value.contains(e.target as Node)) pop.value = ''
}
onMounted(() => document.addEventListener('mousedown', onDoc))
onBeforeUnmount(() => document.removeEventListener('mousedown', onDoc))
watch(() => s.value.kind, () => (pop.value = ''))

const isBold = computed(() => ['bold', '600', '700', '800', '900'].includes(s.value.fontWeight))
const fontSize = (v: number) => editor.set({ fontSize: Math.max(4, Math.round(v)) })
const ALIGNS = ['left', 'center', 'right', 'justify']
const alignIcon = computed(() => ({ left: 'textLeft', center: 'textCenter', right: 'textRight', justify: 'textJustify' } as any)[s.value.textAlign] || 'textLeft')
function cycleAlign() {
  editor.set({ textAlign: ALIGNS[(ALIGNS.indexOf(s.value.textAlign) + 1) % ALIGNS.length] })
}
function toggleList() {
  const lines = s.value.text.split('\n')
  const on = lines.every((l) => l.startsWith('• '))
  editor.set({ text: lines.map((l) => (on ? l.replace(/^• /, '') : '• ' + l)).join('\n') })
}
function toggleCase() {
  const t = s.value.text
  editor.set({ text: t === t.toUpperCase() ? t.toLowerCase().replace(/(^\s*\w|[.!?]\s+\w)/g, (m) => m.toUpperCase()) : t.toUpperCase() })
}
function setShadow(on: boolean) {
  editor.setShadow(on ? { color: 'rgba(0,0,0,0.35)', blur: 18, offsetX: 6, offsetY: 6 } : null)
}
function setOutline(on: boolean) {
  editor.set(on ? { stroke: '#000000', strokeWidth: Math.max(1, Math.round(s.value.fontSize / 24)), paintFirst: 'stroke' } : { stroke: null, strokeWidth: 0 })
}

async function fillPlaceholder() {
  busy.value = true
  try {
    const res = await api.post<{ asset: Asset }>('/api/ai/image', { prompt: s.value.imagePrompt, width: s.value.width, height: s.value.height })
    await editor.replaceActiveImage(res.asset.url, s.value.imagePrompt)
  } catch (e: any) {
    toast.error(e.message)
  } finally {
    busy.value = false
    api.refreshUser() // saldo terpotong per gambar AI
  }
}
async function uploadInto(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0]
  if (!f) return
  const a = await api.upload(f)
  await editor.replaceActiveImage(a.url)
}
</script>

<template>
  <div ref="root" class="bar">
    <!-- tidak ada seleksi -->
    <template v-if="s.kind === 'none'">
      <span class="muted small">Latar halaman</span>
      <ColorPicker :model-value="st.background" :gradient="st.backgroundGradient" allow-gradient
        @update:model-value="(c) => editor.setBackgroundColor(c || '#ffffff')" @gradient="(g) => g && editor.setBackgroundGradient(g)" />
      <div class="vsep" />
      <span class="muted small">Pilih elemen untuk mengedit · klik kanan untuk menu · seret gambar ke kanvas untuk upload</span>
    </template>

    <!-- teks -->
    <template v-if="s.kind === 'text'">
      <select class="select sm font" :value="s.fontFamily" :style="{ fontFamily: s.fontFamily }" @change="editor.setFont(($event.target as HTMLSelectElement).value)">
        <option v-for="f in FONTS" :key="f" :value="f" :style="{ fontFamily: f }">{{ f }}</option>
      </select>
      <div class="num">
        <button class="btn ghost sm icon" @click="fontSize(s.fontSize - 2)"><Icon name="minus" :size="14" /></button>
        <input class="input sm" type="number" :value="s.fontSize" @change="fontSize(+($event.target as HTMLInputElement).value)" />
        <button class="btn ghost sm icon" @click="fontSize(s.fontSize + 2)"><Icon name="plus" :size="14" /></button>
      </div>
      <ColorPicker :model-value="s.fill" :gradient="s.fillGradient" allow-gradient title="Warna teks" @update:model-value="(c) => editor.set({ fill: c || '#000000' })" @gradient="(g) => editor.setFillGradient(g)" />
      <button class="btn ghost sm icon" :class="{ active: isBold }" title="Tebal" @click="editor.set({ fontWeight: isBold ? 'normal' : 'bold' })"><Icon name="bold" :size="16" /></button>
      <button class="btn ghost sm icon" :class="{ active: s.fontStyle === 'italic' }" title="Miring" @click="editor.set({ fontStyle: s.fontStyle === 'italic' ? 'normal' : 'italic' })"><Icon name="italic" :size="16" /></button>
      <button class="btn ghost sm icon" :class="{ active: s.underline }" title="Garis bawah" @click="editor.set({ underline: !s.underline })"><Icon name="underline" :size="16" /></button>
      <button class="btn ghost sm icon" :class="{ active: s.linethrough }" title="Coret" @click="editor.set({ linethrough: !s.linethrough })"><Icon name="strike" :size="16" /></button>
      <button class="btn ghost sm icon" title="Huruf besar/kecil" @click="toggleCase"><Icon name="caseIcon" :size="16" /></button>
      <button class="btn ghost sm icon" title="Perataan" @click="cycleAlign"><Icon :name="alignIcon" :size="16" /></button>
      <button class="btn ghost sm icon" title="Daftar poin" @click="toggleList"><Icon name="list" :size="16" /></button>
      <div class="wrap">
        <button class="btn ghost sm" @click="toggle('spacing')">Spasi</button>
        <div v-if="pop === 'spacing'" class="popover p">
          <label><span class="label">Jarak huruf: {{ s.charSpacing }}</span><input type="range" min="-100" max="800" :value="s.charSpacing" @input="editor.set({ charSpacing: +($event.target as HTMLInputElement).value })" /></label>
          <label><span class="label">Jarak baris: {{ s.lineHeight.toFixed(2) }}</span><input type="range" min="0.5" max="3" step="0.05" :value="s.lineHeight" @input="editor.set({ lineHeight: +($event.target as HTMLInputElement).value })" /></label>
        </div>
      </div>
      <div class="wrap">
        <button class="btn ghost sm" @click="toggle('effects')">Efek</button>
        <div v-if="pop === 'effects'" class="popover p">
          <label class="row"><input type="checkbox" :checked="!!s.shadow" @change="setShadow(($event.target as HTMLInputElement).checked)" /> Bayangan</label>
          <label class="row"><input type="checkbox" :checked="s.strokeWidth > 0" @change="setOutline(($event.target as HTMLInputElement).checked)" /> Outline</label>
          <div v-if="s.strokeWidth > 0" class="row">
            <ColorPicker :model-value="s.stroke" @update:model-value="(c) => editor.set({ stroke: c })" />
            <input type="range" min="1" max="30" :value="s.strokeWidth" @input="editor.set({ strokeWidth: +($event.target as HTMLInputElement).value })" />
          </div>
          <div class="row">
            <span class="grow">Sorotan</span>
            <ColorPicker :model-value="s.textBackgroundColor" @update:model-value="(c) => editor.set({ textBackgroundColor: c })" />
          </div>
        </div>
      </div>
    </template>

    <!-- bentuk -->
    <template v-if="s.kind === 'shape' || s.kind === 'group'">
      <ColorPicker :model-value="s.fill" :gradient="s.fillGradient" allow-gradient title="Warna isi" @update:model-value="(c) => editor.set({ fill: c })" @gradient="(g) => editor.setFillGradient(g)" />
      <template v-if="s.kind === 'shape'">
        <div class="wrap">
          <button class="btn ghost sm" @click="toggle('border')"><span class="bd" :style="{ borderColor: s.stroke || '#bbb' }" /> Garis tepi</button>
          <div v-if="pop === 'border'" class="popover p">
            <div class="row"><span class="grow">Warna</span><ColorPicker :model-value="s.stroke" @update:model-value="(c) => editor.set({ stroke: c, strokeWidth: s.strokeWidth || 4 })" /></div>
            <label><span class="label">Ketebalan: {{ s.strokeWidth }}</span><input type="range" min="0" max="60" :value="s.strokeWidth" @input="editor.set({ strokeWidth: +($event.target as HTMLInputElement).value })" /></label>
            <div class="row">
              <button class="btn sm" @click="editor.set({ strokeDashArray: null })">Solid</button>
              <button class="btn sm" @click="editor.set({ strokeDashArray: [s.strokeWidth * 3, s.strokeWidth * 2] })">Putus</button>
              <button class="btn sm" @click="editor.set({ strokeDashArray: [s.strokeWidth, s.strokeWidth * 1.5], strokeLineCap: 'round' })">Titik</button>
            </div>
          </div>
        </div>
        <label class="row small" title="Sudut tumpul">
          <Icon name="crop" :size="14" />
          <input type="range" min="0" max="300" :value="s.rx" style="width: 90px" @input="editor.set({ rx: +($event.target as HTMLInputElement).value })" />
        </label>
      </template>
      <button v-if="s.kind === 'group'" class="btn ghost sm" @click="editor.ungroup()"><Icon name="ungroup" :size="16" /> Pisahkan grup</button>
    </template>

    <!-- garis & gambar bebas -->
    <template v-if="s.kind === 'line' || s.kind === 'path'">
      <ColorPicker :model-value="s.stroke" title="Warna garis" @update:model-value="(c) => editor.set({ stroke: c })" />
      <label class="row small">Tebal <input type="range" min="1" max="60" :value="s.strokeWidth" style="width: 110px" @input="editor.set({ strokeWidth: +($event.target as HTMLInputElement).value })" /></label>
      <button class="btn ghost sm" @click="editor.set({ strokeDashArray: null })">Solid</button>
      <button class="btn ghost sm" @click="editor.set({ strokeDashArray: [s.strokeWidth * 3, s.strokeWidth * 2] })">Putus-putus</button>
    </template>

    <!-- gambar -->
    <template v-if="s.kind === 'image'">
      <button class="btn ghost sm" @click="emit('open-props')"><Icon name="sparkles" :size="16" /> Filter & penyesuaian</button>
      <div class="wrap">
        <button class="btn ghost sm" @click="toggle('frame')"><Icon name="crop" :size="16" /> Bingkai</button>
        <div v-if="pop === 'frame'" class="popover p">
          <div class="row">
            <button class="btn sm" @click="editor.setFrame('rect', 0)">Tanpa</button>
            <button class="btn sm" @click="editor.setFrame('rect', Math.min(s.width, s.height) * 0.08)">Tumpul</button>
            <button class="btn sm" @click="editor.setFrame('circle')">Lingkaran</button>
          </div>
          <label><span class="label">Radius sudut</span><input type="range" min="0" :max="Math.min(s.width, s.height) / 2" :value="s.frame.radius" @input="editor.setFrame('rect', +($event.target as HTMLInputElement).value)" /></label>
        </div>
      </div>
      <button class="btn ghost sm icon" title="Balik horizontal" @click="editor.flip('x')"><Icon name="flipH" :size="16" /></button>
      <button class="btn ghost sm icon" title="Balik vertikal" @click="editor.flip('y')"><Icon name="flipV" :size="16" /></button>
      <button class="btn ghost sm" @click="editor.activeToBackground()"><Icon name="image" :size="16" /> Jadikan latar</button>
      <label class="btn ghost sm"><Icon name="upload" :size="16" /> Ganti<input type="file" accept="image/*" hidden @change="uploadInto" /></label>
    </template>

    <!-- placeholder gambar dari AI -->
    <template v-if="s.kind === 'placeholder'">
      <span class="small muted ph">“{{ s.imagePrompt }}”</span>
      <button class="btn grad sm" :disabled="busy" @click="fillPlaceholder"><span v-if="busy" class="spin" /><Icon v-else name="sparkles" :size="16" /> Generate gambar AI</button>
      <label class="btn sm"><Icon name="upload" :size="16" /> Upload untuk mengisi<input type="file" accept="image/*" hidden @change="uploadInto" /></label>
    </template>

    <!-- multi -->
    <template v-if="s.kind === 'multi'">
      <span class="small muted">{{ s.count }} objek</span>
      <button class="btn ghost sm" @click="editor.group()"><Icon name="group" :size="16" /> Grup</button>
      <ColorPicker :model-value="s.fill" title="Warna semua" @update:model-value="(c) => editor.set({ fill: c })" />
    </template>

    <div class="grow" />

    <!-- umum -->
    <template v-if="s.kind !== 'none'">
      <div class="wrap">
        <button class="btn ghost sm" @click="toggle('pos')">Posisi</button>
        <div v-if="pop === 'pos'" class="popover p right">
          <span class="label">Rata {{ s.kind === 'multi' ? 'terhadap seleksi' : 'terhadap halaman' }}</span>
          <div class="grid3">
            <button class="btn sm" @click="editor.align('left')"><Icon name="alignLeft" :size="15" /></button>
            <button class="btn sm" @click="editor.align('center')"><Icon name="alignCenter" :size="15" /></button>
            <button class="btn sm" @click="editor.align('right')"><Icon name="alignRight" :size="15" /></button>
            <button class="btn sm" @click="editor.align('top')"><Icon name="alignTop" :size="15" /></button>
            <button class="btn sm" @click="editor.align('middle')"><Icon name="alignMiddle" :size="15" /></button>
            <button class="btn sm" @click="editor.align('bottom')"><Icon name="alignBottom" :size="15" /></button>
          </div>
          <template v-if="s.kind === 'multi' && s.count >= 3">
            <span class="label" style="margin-top: 10px">Distribusi</span>
            <div class="grid2"><button class="btn sm" @click="editor.distribute('h')">Horizontal</button><button class="btn sm" @click="editor.distribute('v')">Vertikal</button></div>
          </template>
          <span class="label" style="margin-top: 10px">Urutan layer</span>
          <div class="grid2">
            <button class="btn sm" @click="editor.arrange('forward')">Maju</button><button class="btn sm" @click="editor.arrange('backward')">Mundur</button>
            <button class="btn sm" @click="editor.arrange('front')">Paling depan</button><button class="btn sm" @click="editor.arrange('back')">Paling belakang</button>
          </div>
        </div>
      </div>
      <div class="wrap">
        <button class="btn ghost sm icon" title="Transparansi" @click="toggle('opacity')"><Icon name="opacity" :size="16" /></button>
        <div v-if="pop === 'opacity'" class="popover p right">
          <span class="label">Transparansi: {{ Math.round(s.opacity * 100) }}%</span>
          <input type="range" min="0" max="1" step="0.01" :value="s.opacity" @input="editor.set({ opacity: +($event.target as HTMLInputElement).value })" />
        </div>
      </div>
      <button class="btn ghost sm icon" :title="s.locked ? 'Buka kunci' : 'Kunci'" @click="editor.toggleLock()"><Icon :name="s.locked ? 'lock' : 'unlock'" :size="16" /></button>
      <button class="btn ghost sm icon" title="Duplikat (Ctrl+D)" @click="editor.duplicate()"><Icon name="copy" :size="16" /></button>
      <button class="btn ghost sm icon danger" title="Hapus (Del)" @click="editor.remove()"><Icon name="trash" :size="16" /></button>
      <button class="btn ghost sm icon" title="Panel properti" @click="emit('toggle-props')"><Icon name="more" :size="16" /></button>
    </template>
  </div>
</template>

<style scoped>
.bar { height: 48px; flex: 0 0 48px; display: flex; align-items: center; gap: 4px; padding: 0 10px; background: #fff; border-bottom: 1px solid var(--line); overflow-x: auto; overflow-y: visible; scrollbar-width: none; }
.small { font-size: 12px; }
.font { width: 150px; }
.num { display: flex; align-items: center; border: 1px solid var(--line); border-radius: 8px; }
.num .input { width: 48px; border: 0; text-align: center; padding: 0; -moz-appearance: textfield; }
.num .input::-webkit-inner-spin-button { display: none; }
.wrap { position: relative; }
.p { position: fixed; top: 108px; width: 250px; display: flex; flex-direction: column; gap: 10px; }
.p.right { right: 16px; }
.bd { width: 14px; height: 14px; border: 3px solid; border-radius: 3px; display: inline-block; }
.ph { max-width: 220px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>
