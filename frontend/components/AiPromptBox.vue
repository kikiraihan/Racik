<script setup lang="ts">
// Kotak prompt utama: tujuan pertama user = auto-bikin desain dari AI.
import { SIZE_PRESETS, STYLE_PRESETS, findPreset } from '~/lib/presets'
import { rupiah, type AIModels } from '~/composables/useApi'

const api = useApi()
const toast = useToast()
const { generate } = useDesigns()

const TYPES = [
  { key: 'presentation', label: 'Presentasi', icon: 'presentation' },
  { key: 'instagram-post', label: 'Instagram', icon: 'image' },
  { key: 'story', label: 'Story', icon: 'image' },
  { key: 'poster', label: 'Poster', icon: 'file' },
  { key: 'youtube-thumbnail', label: 'Thumbnail', icon: 'image' },
  { key: 'facebook-post', label: 'Banner', icon: 'image' },
  { key: 'certificate', label: 'Sertifikat', icon: 'file' },
]

const EXAMPLES = [
  'Presentasi skripsi tentang dampak media sosial terhadap produktivitas mahasiswa',
  'Poster promo kopi susu gula aren diskon 30% untuk pembukaan cabang baru',
  'Konten Instagram tips hemat untuk anak kos, gaya ceria',
  'Pitch deck startup aplikasi laundry on-demand untuk investor',
]

const prompt = ref('')
const type = ref('presentation')
const pages = ref(6)
const style = ref('')
const withImages = ref(true)
// Model AI diatur admin; user hanya melihat status layanan & tarif.
const models = ref<AIModels | null>(null)
const loading = ref(false)
const lowBalance = ref(false)
const step = ref(0)
const STEPS = ['Memahami brief kamu…', 'AI menyusun layout & copy…', 'Membuat gambar dengan AI…', 'Merapikan di editor…']

onMounted(async () => {
  try {
    models.value = await api.get<AIModels>('/api/ai/models')
  } catch {}
})

// Perkiraan biaya: 1x susun layout + per gambar yang berhasil dibuat (maks N gambar).
const costHint = computed(() => {
  const p = models.value?.prices
  if (!p) return ''
  return withImages.value
    ? `Biaya ${rupiah(p.text)} + ${rupiah(p.image)}/gambar (maks ${models.value!.maxImagesPerDesign} gambar)`
    : `Biaya ${rupiah(p.text)}`
})

const preset = computed(() => findPreset(type.value) || SIZE_PRESETS[0])
const isMulti = computed(() => type.value === 'presentation')

let timer: any
async function submit() {
  if (!prompt.value.trim()) {
    toast.error('Tulis dulu desain yang kamu mau')
    return
  }
  loading.value = true
  lowBalance.value = false
  step.value = 0
  timer = setInterval(() => (step.value = Math.min(step.value + 1, withImages.value ? 3 : 1)), withImages.value ? 6000 : 4000)
  try {
    await generate({
      prompt: prompt.value, designType: type.value, width: preset.value.width, height: preset.value.height,
      pages: isMulti.value ? pages.value : 1, style: style.value, generateImages: withImages.value,
    })
  } catch (e: any) {
    lowBalance.value = e.code === 'insufficient_balance'
    toast.error(e.message)
  } finally {
    clearInterval(timer)
    loading.value = false
  }
}
</script>

<template>
  <div class="box card">
    <textarea v-model="prompt" class="ta" rows="3" placeholder="Deskripsikan desain yang kamu mau… mis. 'Presentasi 8 slide tentang energi terbarukan untuk kelas SMA'"
      @keydown.enter.exact.prevent="submit" :disabled="loading" />
    <div class="types">
      <button v-for="t in TYPES" :key="t.key" class="chip" :class="{ on: type === t.key }" @click="type = t.key">
        <Icon :name="t.icon" :size="14" /> {{ t.label }}
      </button>
    </div>
    <div class="opts">
      <label v-if="isMulti" class="row opt">
        <span class="muted">Slide</span>
        <input v-model.number="pages" type="number" min="1" max="15" class="input sm" style="width: 64px" />
      </label>
      <select v-model="style" class="select sm opt" style="width: 170px">
        <option value="">Gaya: otomatis</option>
        <option v-for="s in STYLE_PRESETS" :key="s" :value="s">{{ s }}</option>
      </select>
      <label class="row opt tog">
        <input v-model="withImages" type="checkbox" /> Buat gambar AI
      </label>
      <span v-if="costHint" class="muted cost">{{ costHint }}</span>
      <div class="grow" />
      <button class="btn grad go" :disabled="loading" @click="submit">
        <span v-if="loading" class="spin" />
        <Icon v-else name="sparkles" />
        {{ loading ? 'Membuat…' : 'Generate desain' }}
      </button>
    </div>
    <div v-if="models && !models.configured" class="warn">
      Layanan AI belum tersedia. Hubungi admin.
    </div>
    <div v-if="lowBalance" class="warn">
      Saldo kamu tidak cukup untuk generate desain. <NuxtLink to="/wallet" class="lnk">Lihat saldo &amp; top up →</NuxtLink>
    </div>
    <div v-if="loading" class="progress">
      <div v-for="(s, i) in STEPS" :key="s" class="st" :class="{ done: i < step, cur: i === step }">
        <span class="dot" /> {{ s }}
      </div>
    </div>
    <div v-else class="ex">
      <span class="muted">Coba:</span>
      <button v-for="e in EXAMPLES" :key="e" class="exb" @click="prompt = e">{{ e }}</button>
    </div>
  </div>
</template>

<style scoped>
.box { padding: 16px; text-align: left; }
.ta { width: 100%; border: 0; outline: none; resize: none; font-size: 17px; line-height: 1.5; padding: 4px 4px 10px; background: transparent; }
.types { display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 10px; }
.opts { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; }
.opt { font-size: 13px; }
.tog { cursor: pointer; user-select: none; }
.go { height: 42px; padding: 0 20px; font-size: 15px; border-radius: 10px; }
.cost { font-size: 12px; }
.lnk { color: var(--accent); font-weight: 600; }
.warn { margin-top: 10px; background: #fff7e6; border: 1px solid #ffd591; color: #8a5a00; padding: 8px 12px; border-radius: 8px; font-size: 13px; }
.ex { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 12px; align-items: center; font-size: 13px; }
.exb { border: 1px dashed var(--line); background: var(--panel-2); border-radius: 999px; padding: 4px 10px; cursor: pointer; font-size: 12px; color: var(--ink-2); max-width: 100%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.exb:hover { border-color: var(--accent); color: var(--accent); }
.progress { margin-top: 14px; display: grid; gap: 6px; }
.st { display: flex; align-items: center; gap: 8px; color: var(--muted); font-size: 13px; }
.st .dot { width: 8px; height: 8px; border-radius: 50%; background: var(--line); }
.st.done { color: var(--ok); } .st.done .dot { background: var(--ok); }
.st.cur { color: var(--accent); font-weight: 600; } .st.cur .dot { background: var(--accent); animation: pulse 1s infinite; }
@keyframes pulse { 50% { transform: scale(1.6); opacity: .6; } }
</style>
