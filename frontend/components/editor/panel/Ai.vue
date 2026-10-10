<script setup lang="ts">
// Panel AI: Magic Design, Perbaiki desain, Generate gambar, Edit gambar, Copywriting.
import { Textbox } from 'fabric'
import { STYLE_PRESETS } from '~/lib/presets'
import { rupiah, type AIModels, type Asset } from '~/composables/useApi'

const editor = useEditor()
const doc = useDoc()
const api = useApi()
const toast = useToast()
const session = useSession()
const st = editor.state

const models = ref<AIModels | null>(null)
const section = ref<'design' | 'refine' | 'image' | 'copy'>('design')
const busy = ref('')

const design = reactive({ prompt: '', pages: 1, mode: 'append' as 'append' | 'replace-page' | 'replace-all', style: '', images: true })
const refine = reactive({ instruction: '', images: false })
const img = reactive({ prompt: '', style: '', aspect: 'page' as 'page' | 'square' | 'landscape' | 'portrait', edit: '' })
const copy = reactive({ prompt: '', kind: 'headline', options: [] as string[] })
const lowBalance = ref(false)
const aiImages = ref<Asset[]>([])

const REFINE_CHIPS = ['Buat lebih minimalis & lega', 'Warna lebih cerah dan berani', 'Perbesar judul, perkuat hierarki', 'Perbaiki kontras & keterbacaan', 'Ubah ke gaya korporat profesional', 'Tambahkan aksen dekoratif modern']
const IMG_STYLES = ['Foto realistis', 'Ilustrasi flat', '3D render', 'Cat air', 'Anime', 'Minimalis line art', 'Isometrik']
const EDIT_CHIPS = ['Hapus latar belakang, jadikan putih polos', 'Ubah jadi gaya ilustrasi', 'Tambahkan cahaya senja hangat', 'Perjelas & tingkatkan kualitas']

onMounted(async () => {
  try {
    // Model AI diatur admin; yang diterima hanya status layanan & tarif per fitur.
    models.value = await api.get<AIModels>('/api/ai/models')
  } catch {}
  loadAiImages()
  design.pages = st.width > st.height ? 5 : 1
})

async function loadAiImages() {
  try {
    aiImages.value = await api.get<Asset[]>('/api/assets', { source: 'ai' })
  } catch {}
}

async function run<T>(key: string, fn: () => Promise<T>) {
  busy.value = key
  lowBalance.value = false
  try {
    return await fn()
  } catch (e: any) {
    lowBalance.value = e.code === 'insufficient_balance'
    toast.error(e.message)
  } finally {
    busy.value = ''
    api.refreshUser() // saldo terpotong / dikembalikan
  }
}

// Label tarif di tombol, mis. "Rp 105 + Rp 683/gambar".
const price = computed(() => models.value?.prices)
function withImagesCost(images: boolean) {
  if (!price.value) return ''
  return images ? `${rupiah(price.value.text)} + ${rupiah(price.value.image)}/gambar` : rupiah(price.value.text)
}

function designType() {
  const r = st.width / st.height
  if (r > 1.6) return 'presentation'
  if (r < 0.7) return 'story'
  return 'custom'
}

function handleResult(res: { generationId: string; spec: any; warnings: string[] }) {
  res.warnings?.forEach((w) => toast.error(w))
  doc.generationId = res.generationId
  doc.rated = false
}

const genDesign = () => run('design', async () => {
  if (!design.prompt.trim()) throw new Error('Tulis brief desain dulu')
  const res = await api.post<any>('/api/ai/design', {
    prompt: design.prompt, designType: designType(), width: st.width, height: st.height, pages: design.pages,
    style: design.style, generateImages: design.images,
  })
  await editor.applySpec(res.spec, design.mode)
  handleResult(res)
  toast.success('Desain AI ditambahkan ✨')
})

const doRefine = (instruction = refine.instruction) => run('refine', async () => {
  if (!instruction.trim()) throw new Error('Tulis instruksi perbaikan')
  const res = await api.post<any>('/api/ai/refine', {
    spec: { pages: [editor.currentPageSpec()] }, instruction, width: st.width, height: st.height,
    generateImages: refine.images,
  })
  await editor.applySpec({ pages: res.spec.pages.slice(0, 1) }, 'replace-page')
  handleResult(res)
  toast.success('Halaman diperbaiki AI — tekan Undo kalau kurang cocok')
})

function aspectSize() {
  switch (img.aspect) {
    case 'square': return [1024, 1024]
    case 'landscape': return [1536, 1024]
    case 'portrait': return [1024, 1536]
    default: {
      const sel = st.sel
      if (sel.kind === 'placeholder' || sel.kind === 'image') return [sel.width, sel.height]
      return [st.width, st.height]
    }
  }
}

const genImage = () => run('image', async () => {
  const prompt = img.prompt.trim() || st.sel.imagePrompt
  if (!prompt) throw new Error('Tulis deskripsi gambar')
  const [w, h] = aspectSize()
  const res = await api.post<{ asset: Asset }>('/api/ai/image', { prompt, style: img.style, width: w, height: h })
  if (st.sel.kind === 'placeholder') await editor.replaceActiveImage(res.asset.url, prompt)
  else await editor.addImage(res.asset.url, { name: 'Gambar AI', imagePrompt: prompt })
  aiImages.value.unshift(res.asset)
})

const editImage = (prompt = img.edit) => run('edit', async () => {
  if (!prompt.trim()) throw new Error('Tulis instruksi edit gambar')
  const src = st.sel.src
  const body: any = { prompt }
  if (src.includes('/uploads/')) body.url = src
  else body.dataUrl = editor.activeDataURL()
  const res = await api.post<{ asset: Asset }>('/api/ai/image-edit', body)
  await editor.replaceActiveImage(res.asset.url)
  aiImages.value.unshift(res.asset)
  toast.success('Gambar diedit AI')
})

const genCopy = () => run('copy', async () => {
  const prompt = copy.prompt.trim() || (st.sel.kind === 'text' ? st.sel.text : '')
  if (!prompt) throw new Error('Tulis topik copy')
  const res = await api.post<{ options: string[] }>('/api/ai/copy', { prompt, kind: copy.kind })
  copy.options = res.options
})

function useCopy(t: string) {
  const o = editor.active()
  if (o instanceof Textbox) editor.set({ text: t })
  else editor.addText({ text: t, fontSize: 48, fontWeight: 'bold' })
}
</script>

<template>
  <div>
    <div v-if="models && !models.configured" class="warn">Layanan AI belum tersedia. Hubungi admin.</div>
    <div class="saldo">
      <Icon name="wallet" :size="14" /> Saldo <b>{{ rupiah(session.user.value?.balance ?? 0) }}</b>
      <div class="grow" />
      <NuxtLink to="/wallet" target="_blank" class="lnk">Riwayat &amp; top up</NuxtLink>
    </div>
    <div v-if="lowBalance" class="warn">Saldo tidak cukup untuk fitur AI ini. Fitur edit biasa tetap gratis. <NuxtLink to="/wallet" target="_blank" class="lnk">Top up saldo →</NuxtLink></div>
    <div class="tabs">
      <button class="chip" :class="{ on: section === 'design' }" @click="section = 'design'">Magic Design</button>
      <button class="chip" :class="{ on: section === 'refine' }" @click="section = 'refine'">Perbaiki</button>
      <button class="chip" :class="{ on: section === 'image' }" @click="section = 'image'">Gambar AI</button>
      <button class="chip" :class="{ on: section === 'copy' }" @click="section = 'copy'">Copywriting</button>
    </div>


    <!-- Magic Design -->
    <div v-if="section === 'design'" class="col">
      <textarea v-model="design.prompt" class="textarea" rows="4" placeholder="mis. 5 slide tentang strategi marketing UMKM di TikTok" />
      <div class="grid2">
        <label><span class="label">Jumlah halaman</span><input v-model.number="design.pages" type="number" min="1" max="15" class="input sm" /></label>
        <label><span class="label">Gaya</span>
          <select v-model="design.style" class="select sm"><option value="">Otomatis</option><option v-for="s in STYLE_PRESETS" :key="s">{{ s }}</option></select></label>
      </div>
      <label><span class="label">Hasil</span>
        <select v-model="design.mode" class="select sm">
          <option value="append">Tambahkan sebagai halaman baru</option>
          <option value="replace-page">Ganti halaman ini</option>
          <option value="replace-all">Ganti semua halaman</option>
        </select></label>
      <label class="row"><input v-model="design.images" type="checkbox" /> Buat gambar AI</label>
      <button class="btn grad" :disabled="!!busy" @click="genDesign"><span v-if="busy === 'design'" class="spin" /><Icon v-else name="sparkles" /> Generate desain</button>
      <p v-if="price" class="muted cost">Biaya {{ withImagesCost(design.images) }} · gambar yang gagal tidak ditagih</p>
      <p class="muted tip">Tip: atur Brand Kit agar warna & font hasil AI sesuai brand kamu. Beri 👍/👎 pada hasil supaya AI makin paham seleramu.</p>
    </div>

    <!-- Refine -->
    <div v-else-if="section === 'refine'" class="col">
      <p class="muted tip">AI membaca halaman aktif lalu menyusun ulang sesuai instruksimu.</p>
      <div class="chips">
        <button v-for="c in REFINE_CHIPS" :key="c" class="chip" :disabled="!!busy" @click="doRefine(c)">{{ c }}</button>
      </div>
      <textarea v-model="refine.instruction" class="textarea" rows="3" placeholder="Instruksi lain, mis. ganti tema jadi ramadhan, warna hijau emas" />
      <label class="row"><input v-model="refine.images" type="checkbox" /> Boleh tambah gambar AI baru</label>
      <button class="btn grad" :disabled="!!busy" @click="doRefine()"><span v-if="busy === 'refine'" class="spin" /><Icon v-else name="wand" /> Perbaiki halaman ini</button>
      <p v-if="price" class="muted cost">Biaya {{ withImagesCost(refine.images) }} per perbaikan</p>
    </div>

    <!-- Image -->
    <div v-else-if="section === 'image'" class="col">
      <template v-if="st.sel.kind === 'image'">
        <h4>Edit gambar terpilih dengan AI</h4>
        <div class="chips"><button v-for="c in EDIT_CHIPS" :key="c" class="chip" :disabled="!!busy" @click="editImage(c)">{{ c }}</button></div>
        <textarea v-model="img.edit" class="textarea" rows="2" placeholder="mis. ganti langit jadi malam berbintang" />
        <button class="btn primary" :disabled="!!busy" @click="editImage()"><span v-if="busy === 'edit'" class="spin" /><Icon v-else name="wand" /> Edit gambar</button>
        <p v-if="price" class="muted cost">Biaya {{ rupiah(price.edit) }} per edit</p>
        <div class="sep" />
      </template>
      <h4>{{ st.sel.kind === 'placeholder' ? 'Isi placeholder dengan gambar AI' : 'Generate gambar baru' }}</h4>
      <textarea v-model="img.prompt" class="textarea" rows="3" :placeholder="st.sel.imagePrompt || 'mis. secangkir kopi susu di meja kayu, cahaya pagi'" />
      <div class="chips"><button v-for="s in IMG_STYLES" :key="s" class="chip" :class="{ on: img.style === s }" @click="img.style = img.style === s ? '' : s">{{ s }}</button></div>
      <label><span class="label">Rasio</span>
        <select v-model="img.aspect" class="select sm">
          <option value="page">Sesuai halaman / objek terpilih</option><option value="square">Persegi 1:1</option>
          <option value="landscape">Landscape 3:2</option><option value="portrait">Portrait 2:3</option>
        </select></label>
      <button class="btn grad" :disabled="!!busy" @click="genImage"><span v-if="busy === 'image'" class="spin" /><Icon v-else name="image" /> Generate gambar</button>
      <p v-if="price" class="muted cost">Biaya {{ rupiah(price.image) }} per gambar</p>
      <h4 v-if="aiImages.length">Riwayat gambar AI</h4>
      <div class="imgs">
        <img v-for="a in aiImages" :key="a.id" :src="a.url" draggable="true" alt="" title="Klik untuk menambahkan"
          @dragstart="(e) => e.dataTransfer?.setData('text/racik-image', a.url)" @click="editor.addImage(a.url, { name: 'Gambar AI' })" />
      </div>
    </div>

    <!-- Copy -->
    <div v-else class="col">
      <select v-model="copy.kind" class="select sm">
        <option value="headline">Judul / headline</option><option value="tagline">Tagline / slogan</option>
        <option value="caption">Caption Instagram</option><option value="cta">Call to action</option><option value="bullet points">Poin-poin</option>
      </select>
      <textarea v-model="copy.prompt" class="textarea" rows="3" :placeholder="st.sel.kind === 'text' ? 'Kosongkan untuk menulis ulang teks terpilih' : 'mis. promo es teh jumbo 5 ribu'" />
      <button class="btn grad" :disabled="!!busy" @click="genCopy"><span v-if="busy === 'copy'" class="spin" /><Icon v-else name="text" /> Tulis dengan AI</button>
      <p v-if="price" class="muted cost">Biaya {{ rupiah(price.text) }} per permintaan (5 alternatif)</p>
      <button v-for="o in copy.options" :key="o" class="opt" @click="useCopy(o)">{{ o }}</button>
    </div>
  </div>
</template>

<style scoped>
.warn { background: #fff7e6; border: 1px solid #ffd591; color: #8a5a00; padding: 8px 10px; border-radius: 8px; font-size: 12px; margin-bottom: 10px; }
.tabs { display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 12px; }
.saldo { display: flex; align-items: center; gap: 6px; font-size: 12px; background: var(--accent-soft); color: var(--accent); padding: 8px 10px; border-radius: 8px; margin-bottom: 10px; }
.lnk { color: var(--accent); font-weight: 600; }
.cost { font-size: 11px; margin: -2px 0 0; text-align: center; }
.chips { display: flex; flex-wrap: wrap; gap: 6px; }
.chips .chip { height: auto; padding: 6px 10px; font-size: 12px; text-align: left; }
.tip { font-size: 12px; line-height: 1.5; margin: 0; }
.imgs { display: grid; grid-template-columns: 1fr 1fr; gap: 6px; }
.imgs img { width: 100%; aspect-ratio: 1; object-fit: cover; border-radius: 8px; cursor: pointer; border: 1px solid var(--line); }
.imgs img:hover { outline: 2px solid var(--accent); }
.opt { text-align: left; background: var(--panel-2); border: 1px solid var(--line); border-radius: 8px; padding: 10px; cursor: pointer; line-height: 1.4; }
.opt:hover { border-color: var(--accent); }
.row { font-size: 13px; }
</style>
