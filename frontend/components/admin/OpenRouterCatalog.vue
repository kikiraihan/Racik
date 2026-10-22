<script setup lang="ts">
// Katalog model OpenRouter (seperti admin → Model AI di tiara): admin mencari model dari
// GET /models OpenRouter lalu menambahkannya ke pilihan model teks/gambar tanpa ubah env.
interface CatalogModel {
  id: string
  kind: 'text' | 'image'
  name: string
  description?: string
  contextLength: number | null
  promptPrice: number | null
  completionPrice: number | null
  imagePrice: number | null
  inputModalities: string[] | null
  isFree: boolean
}
interface Lists {
  text: string[]
  image: string[]
}

const emit = defineEmits<{ changed: [] }>()
const api = useApi()
const toast = useToast()

const kind = ref<'text' | 'image'>('text')
const enabled = ref(true)
const items = ref<CatalogModel[]>([])
const env = ref<Lists>({ text: [], image: [] })
const saved = ref<Lists>({ text: [], image: [] })
const form = reactive<Lists>({ text: [], image: [] })
const q = ref('')
const freeOnly = ref(false)
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const LIMIT = 100

function apply(res: any) {
  enabled.value = res.enabled
  env.value = res.env
  saved.value = { text: [...(res.added?.text || [])], image: [...(res.added?.image || [])] }
}

async function load(refresh = false) {
  loading.value = true
  error.value = ''
  try {
    const res: any = await api.get('/api/admin/openrouter/models', { kind: kind.value, refresh: refresh ? 1 : undefined })
    apply(res)
    items.value = res.items
  } catch (e: any) {
    error.value = e.message
    items.value = []
  } finally {
    loading.value = false
  }
}

async function init() {
  await load()
  Object.assign(form, { text: [...saved.value.text], image: [...saved.value.image] })
}
onMounted(init)
watch(kind, () => load())

const byId = computed(() => new Map(items.value.map((m) => [m.id, m])))
const filtered = computed(() => {
  const s = q.value.trim().toLowerCase()
  return items.value.filter(
    (m) => (!freeOnly.value || m.isFree) && (!s || m.id.toLowerCase().includes(s) || m.name.toLowerCase().includes(s)),
  )
})
const dirty = computed(() => JSON.stringify(form) !== JSON.stringify(saved.value))

const inEnv = (id: string) => env.value[kind.value].includes(id)
const added = (id: string) => form[kind.value].includes(id)
function toggle(id: string) {
  const list = form[kind.value]
  const i = list.indexOf(id)
  if (i >= 0) list.splice(i, 1)
  else list.push(id)
}

async function save() {
  saving.value = true
  try {
    apply(await api.put('/api/admin/openrouter/models', form))
    Object.assign(form, { text: [...saved.value.text], image: [...saved.value.image] })
    toast.success('Model OpenRouter tersimpan — langsung muncul di pilihan model')
    emit('changed')
  } catch (e: any) {
    toast.error(e.message)
  } finally {
    saving.value = false
  }
}

function reset() {
  Object.assign(form, { text: [...saved.value.text], image: [...saved.value.image] })
}

// Harga OpenRouter dalam USD (per 1 juta token / per gambar), hanya sebagai acuan admin.
function usd(n: number | null, digits = 2) {
  if (n === null || n === undefined) return '—'
  if (n === 0) return '0'
  return '$' + (n < 0.01 ? n.toPrecision(2) : n.toFixed(digits))
}
function ctx(n: number | null) {
  if (!n) return '—'
  return n >= 1000 ? Math.round(n / 1000) + 'K' : String(n)
}
</script>

<template>
  <div class="adm">
    <div class="card sec col" style="gap: 12px">
      <div class="row head">
        <div>
          <b>Katalog model OpenRouter</b>
          <div class="muted small">Cari model dari OpenRouter lalu tambahkan ke pilihan model di atas — tanpa mengubah env atau restart.</div>
        </div>
        <div class="row" style="gap: 6px">
          <button class="btn sm" :class="{ active: kind === 'text' }" @click="kind = 'text'">Teks ({{ form.text.length }})</button>
          <button class="btn sm" :class="{ active: kind === 'image' }" @click="kind = 'image'">Gambar ({{ form.image.length }})</button>
        </div>
      </div>

      <div v-if="!enabled" class="note">OpenRouter belum aktif. Isi <code>OPENROUTER_API_KEY</code> di env backend lalu restart.</div>
      <template v-else>
        <div class="chips">
          <span v-for="id in env[kind]" :key="'env-' + id" class="tag" title="Dari env, ubah lewat OPENROUTER_*_MODELS">{{ id }} · env</span>
          <span v-for="id in form[kind]" :key="id" class="tag acc chip">
            {{ byId.get(id)?.name || id }}
            <button class="x" title="Hapus" @click="toggle(id)"><Icon name="x" :size="12" /></button>
          </span>
          <span v-if="!env[kind].length && !form[kind].length" class="muted small">Belum ada model {{ kind === 'text' ? 'teks' : 'gambar' }} OpenRouter.</span>
        </div>

        <div class="row" style="gap: 8px; flex-wrap: wrap">
          <input v-model="q" class="input sm" style="flex: 1; min-width: 180px" placeholder="Cari id / nama model, mis. claude, gemini, flux" />
          <label class="row small"><input v-model="freeOnly" type="checkbox" /> gratis saja</label>
          <button class="btn sm" :disabled="loading" title="Ambil ulang katalog dari OpenRouter" @click="load(true)">
            <span v-if="loading" class="spin" /><Icon v-else name="refresh" :size="14" />Muat ulang
          </button>
        </div>

        <div v-if="error" class="note">{{ error }}</div>
        <div v-else class="tbl-wrap list">
          <table class="tbl">
            <thead>
              <tr>
                <th>Model</th>
                <th class="r">Konteks</th>
                <th class="r">{{ kind === 'image' ? 'Input / output (USD/1M) · gambar' : 'Input / output (USD/1M token)' }}</th>
                <th />
              </tr>
            </thead>
            <tbody>
              <tr v-for="m in filtered.slice(0, LIMIT)" :key="m.id">
                <td>
                  <b>{{ m.name }}</b> <span v-if="m.isFree" class="tag ok">gratis</span>
                  <span v-if="m.inputModalities?.includes('image')" class="tag" title="Bisa membaca gambar">vision</span>
                  <div class="muted small">{{ m.id }}</div>
                </td>
                <td class="r nowrap">{{ ctx(m.contextLength) }}</td>
                <td class="r nowrap small">
                  {{ usd(m.promptPrice) }} / {{ usd(m.completionPrice) }}<template v-if="kind === 'image' && m.imagePrice"> · {{ usd(m.imagePrice, 3) }}</template>
                </td>
                <td class="r">
                  <span v-if="inEnv(m.id)" class="tag">env</span>
                  <button v-else class="btn sm" :class="added(m.id) ? 'ghost danger' : ''" @click="toggle(m.id)">
                    <Icon :name="added(m.id) ? 'trash' : 'plus'" :size="14" />{{ added(m.id) ? 'Hapus' : 'Tambah' }}
                  </button>
                </td>
              </tr>
              <tr v-if="!loading && !filtered.length"><td colspan="4" class="muted">Tidak ada model yang cocok.</td></tr>
            </tbody>
          </table>
        </div>
        <div v-if="filtered.length > LIMIT" class="muted small">Menampilkan {{ LIMIT }} dari {{ filtered.length }} model — persempit pencarian.</div>

        <p class="muted small" style="margin: 0">
          Harga di atas adalah harga OpenRouter (USD) sebagai acuan; harga ke user tetap diatur di menu Konfigurasi harga (Rupiah per hit).
          Model gambar dipanggil lewat chat completions, jadi model gambar OpenRouter juga bisa dipakai untuk edit gambar.
        </p>
        <div class="row" style="justify-content: flex-end">
          <button class="btn" :disabled="!dirty" @click="reset">Batalkan</button>
          <button class="btn primary" :disabled="saving || !dirty" @click="save"><span v-if="saving" class="spin" />Simpan model</button>
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
.head { justify-content: space-between; align-items: flex-start; gap: 12px; flex-wrap: wrap; }
.chips { display: flex; flex-wrap: wrap; gap: 6px; }
.chip { display: inline-flex; align-items: center; gap: 4px; }
.chip .x { border: 0; background: transparent; color: inherit; cursor: pointer; padding: 0; display: inline-flex; }
.list { max-height: 420px; overflow-y: auto; }
.btn.sm { gap: 4px; }
</style>
