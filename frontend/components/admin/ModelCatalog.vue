<script setup lang="ts">
// Katalog model (seperti admin → Model AI di tiara/katilidev): daftar model yang tersedia di
// sistem, terpisah dari Konfigurasi AI. Model env masuk otomatis sebagai model bawaan; model lain
// ditambah dari daftar GET /models OpenRouter dan kenari. Konfigurasi AI hanya menawarkan model
// katalog yang aktif.
interface Provider {
  name: string
  label: string
  active: boolean
  text: boolean
  image: boolean
  edit: boolean
  catalog: boolean
}
interface Row {
  id: number
  provider: string
  modelId: string
  choice: string
  kind: 'text' | 'image'
  name: string
  maker: string
  status: 'active' | 'disabled'
  isBuiltin: boolean
  isFree: boolean
  contextLength: number | null
  promptPrice: number | null
  completionPrice: number | null
  imagePrice: number | null
  notes: string
  inUse: string[]
}
interface RemoteModel {
  id: string
  kind: 'text' | 'image'
  name: string
  maker?: string
  contextLength: number | null
  promptPrice: number | null
  completionPrice: number | null
  imagePrice: number | null
  inputModalities: string[] | null
  isFree: boolean
  inCatalog: boolean
}

const api = useApi()
const toast = useToast()

const providers = ref<Provider[]>([])
const items = ref<Row[]>([])
const loaded = ref(false)
const busy = ref<Record<number, boolean>>({})

// filter tabel katalog
const fProvider = ref('')
const fKind = ref<'' | 'text' | 'image'>('')
const fStatus = ref<'' | 'active' | 'disabled'>('')
const q = ref('')

function apply(res: any) {
  providers.value = res.providers
  items.value = res.items
}

async function load() {
  try {
    apply(await api.get('/api/admin/models'))
  } catch (e: any) {
    toast.error(e.message)
  } finally {
    loaded.value = true
  }
}
onMounted(load)

const provLabel = (name: string) => providers.value.find((p) => p.name === name)?.label || name
const provActive = (name: string) => !!providers.value.find((p) => p.name === name)?.active
// provider di tabel: yang terdaftar + yang hanya muncul di katalog (API key-nya sudah dikosongkan)
const provNames = computed(() => {
  const names = providers.value.map((p) => p.name)
  for (const r of items.value) if (!names.includes(r.provider)) names.push(r.provider)
  return names
})
const count = (name: string) => items.value.filter((r) => r.provider === name).length

const filtered = computed(() => {
  const s = q.value.trim().toLowerCase()
  return items.value.filter(
    (r) =>
      (!fProvider.value || r.provider === fProvider.value) &&
      (!fKind.value || r.kind === fKind.value) &&
      (!fStatus.value || r.status === fStatus.value) &&
      (!s || r.modelId.toLowerCase().includes(s) || r.name.toLowerCase().includes(s) || r.maker.toLowerCase().includes(s)),
  )
})
const offered = computed(() => items.value.filter((r) => r.status === 'active' && provActive(r.provider)).length)

const USE: Record<string, string> = { text: 'teks', image: 'gambar', edit: 'edit gambar' }

async function setStatus(r: Row, status: Row['status']) {
  busy.value = { ...busy.value, [r.id]: true }
  try {
    apply(await api.request(`/api/admin/models/${r.id}`, { method: 'PATCH', body: { status } }))
    toast.success(status === 'active' ? `${r.name} diaktifkan` : `${r.name} dinonaktifkan`)
  } catch (e: any) {
    toast.error(e.message)
  } finally {
    busy.value = { ...busy.value, [r.id]: false }
  }
}

// ubah nama tampilan & catatan
const editing = ref<{ row: Row; name: string; notes: string } | null>(null)
const savingEdit = ref(false)
function openEdit(r: Row) {
  editing.value = { row: r, name: r.name, notes: r.notes }
}
async function saveEdit() {
  const e = editing.value!
  savingEdit.value = true
  try {
    apply(await api.request(`/api/admin/models/${e.row.id}`, { method: 'PATCH', body: { name: e.name, notes: e.notes } }))
    toast.success('Model tersimpan')
    editing.value = null
  } catch (err: any) {
    toast.error(err.message)
  } finally {
    savingEdit.value = false
  }
}

async function remove(r: Row) {
  if (!confirm(`Hapus ${r.choice} dari katalog?`)) return
  busy.value = { ...busy.value, [r.id]: true }
  try {
    apply(await api.del(`/api/admin/models/${r.id}`))
    toast.success('Model dihapus dari katalog')
  } catch (e: any) {
    toast.error(e.message)
  } finally {
    busy.value = { ...busy.value, [r.id]: false }
  }
}

// ---------- tambah dari daftar API provider (OpenRouter / kenari) ----------
const browsing = ref(false)
const rProvider = ref('')
const rKind = ref<'text' | 'image'>('text')
const remote = ref<RemoteModel[]>([])
const rq = ref('')
const freeOnly = ref(false)
const rLoading = ref(false)
const rError = ref('')
const picked = ref<Set<string>>(new Set())
const adding = ref(false)
const LIMIT = 150

const catalogProviders = computed(() => providers.value.filter((p) => p.catalog))

async function loadRemote(refresh = false) {
  if (!rProvider.value) return
  rLoading.value = true
  rError.value = ''
  try {
    const res: any = await api.get('/api/admin/models/remote', { provider: rProvider.value, kind: rKind.value, refresh: refresh ? 1 : undefined })
    remote.value = res.items
  } catch (e: any) {
    rError.value = e.message
    remote.value = []
  } finally {
    rLoading.value = false
  }
}
function openBrowse() {
  rProvider.value = catalogProviders.value.find((p) => p.name === fProvider.value)?.name || catalogProviders.value[0]?.name || ''
  rKind.value = fKind.value || 'text'
  rq.value = ''
  picked.value = new Set()
  browsing.value = true
  loadRemote()
}
// pilihan tidak dibawa saat pindah provider / jenis (daftarnya berbeda)
watch([rProvider, rKind], () => {
  if (!browsing.value) return
  picked.value = new Set()
  loadRemote()
})

const rFiltered = computed(() => {
  const s = rq.value.trim().toLowerCase()
  return remote.value.filter(
    (m) =>
      (!freeOnly.value || m.isFree) &&
      (!s || m.id.toLowerCase().includes(s) || m.name.toLowerCase().includes(s) || (m.maker || '').toLowerCase().includes(s)),
  )
})
function togglePick(id: string) {
  const s = new Set(picked.value)
  s.has(id) ? s.delete(id) : s.add(id)
  picked.value = s
}

async function addPicked() {
  adding.value = true
  try {
    const res: any = await api.post('/api/admin/models', { provider: rProvider.value, kind: rKind.value, ids: [...picked.value] })
    apply(res)
    toast.success(`${res.added} model ditambahkan ke katalog` + (res.skipped ? `, ${res.skipped} dilewati` : '') + ' — langsung muncul di Konfigurasi AI')
    browsing.value = false
  } catch (e: any) {
    toast.error(e.message)
  } finally {
    adding.value = false
  }
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
function price(m: { kind: string; promptPrice: number | null; completionPrice: number | null; imagePrice: number | null }) {
  if (m.promptPrice === null && m.completionPrice === null && !m.imagePrice) return '—'
  let s = `${usd(m.promptPrice)} / ${usd(m.completionPrice)}`
  if (m.kind === 'image' && m.imagePrice) s += ` · ${usd(m.imagePrice, 3)}`
  return s
}
</script>

<template>
  <div class="adm">
    <div class="card sec col" style="gap: 12px">
      <div class="row head">
        <div>
          <b>Katalog model</b>
          <div class="muted small">
            Model yang tersedia di sistem. Konfigurasi AI hanya menawarkan model yang <b>aktif</b> dari provider yang API key-nya diisi
            ({{ offered }} model sekarang).
          </div>
        </div>
        <button class="btn primary sm" :disabled="!catalogProviders.length" @click="openBrowse">
          <Icon name="plus" :size="14" />Tambah dari daftar provider
        </button>
      </div>

      <div v-if="loaded && !catalogProviders.length" class="note">
        Daftar model tersedia untuk OpenRouter dan kenari. Isi <code>OPENROUTER_API_KEY</code> atau <code>KENARI_API_KEY</code> di env backend lalu
        restart untuk menambah model dari daftarnya.
      </div>

      <div class="row filters">
        <button class="chip" :class="{ on: !fProvider }" @click="fProvider = ''">Semua ({{ items.length }})</button>
        <button v-for="n in provNames" :key="n" class="chip" :class="{ on: fProvider === n }" @click="fProvider = n">
          {{ provLabel(n) }} ({{ count(n) }})<span v-if="!provActive(n)" class="muted"> · belum aktif</span>
        </button>
      </div>
      <div class="row" style="gap: 8px; flex-wrap: wrap">
        <input v-model="q" class="input sm" style="flex: 1; min-width: 180px" placeholder="Cari id / nama / pembuat model" />
        <select v-model="fKind" class="select sm" style="width: auto">
          <option value="">Semua jenis</option>
          <option value="text">Teks</option>
          <option value="image">Gambar</option>
        </select>
        <select v-model="fStatus" class="select sm" style="width: auto">
          <option value="">Semua status</option>
          <option value="active">Aktif</option>
          <option value="disabled">Nonaktif</option>
        </select>
      </div>

      <div class="tbl-wrap">
        <table class="tbl">
          <thead>
            <tr>
              <th>Model</th>
              <th>Provider</th>
              <th>Jenis</th>
              <th>Status</th>
              <th class="r">Konteks</th>
              <th class="r">Harga OpenRouter (USD)</th>
              <th />
            </tr>
          </thead>
          <tbody>
            <tr v-for="r in filtered" :key="r.id" :class="{ off: r.status !== 'active' || !provActive(r.provider) }">
              <td>
                <b :class="{ mono: r.name === r.modelId }">{{ r.name }}</b>
                <div v-if="r.name !== r.modelId" class="muted small mono">{{ r.modelId }}</div>
                <div class="badges">
                  <span v-if="r.isBuiltin" class="tag" title="Dari env backend; tidak bisa dihapus, boleh dinonaktifkan">bawaan env</span>
                  <span v-if="r.isFree" class="tag ok">gratis</span>
                  <span v-for="u in r.inUse" :key="u" class="tag acc" title="Dipakai di Konfigurasi AI">dipakai: {{ USE[u] || u }}</span>
                </div>
                <div v-if="r.notes" class="muted small">{{ r.notes }}</div>
              </td>
              <td class="nowrap">
                {{ provLabel(r.provider) }}
                <div v-if="!provActive(r.provider)" class="muted small">API key kosong</div>
              </td>
              <td>{{ r.kind === 'image' ? 'Gambar' : 'Teks' }}</td>
              <td>
                <span class="tag" :class="r.status === 'active' ? 'ok' : ''">{{ r.status === 'active' ? 'Aktif' : 'Nonaktif' }}</span>
              </td>
              <td class="r nowrap">{{ ctx(r.contextLength) }}</td>
              <td class="r nowrap small">{{ price(r) }}</td>
              <td class="r nowrap">
                <button v-if="r.status === 'active'" class="btn sm ghost" :disabled="busy[r.id]" @click="setStatus(r, 'disabled')">Nonaktifkan</button>
                <button v-else class="btn sm" :disabled="busy[r.id]" @click="setStatus(r, 'active')">Aktifkan</button>
                <button class="btn sm ghost icon" title="Ubah nama & catatan" @click="openEdit(r)"><Icon name="edit" :size="14" /></button>
                <button v-if="!r.isBuiltin" class="btn sm ghost icon danger" title="Hapus dari katalog" :disabled="busy[r.id]" @click="remove(r)">
                  <Icon name="trash" :size="14" />
                </button>
              </td>
            </tr>
            <tr v-if="loaded && !filtered.length"><td colspan="7" class="muted">Tidak ada model yang cocok.</td></tr>
            <tr v-if="!loaded"><td colspan="7" class="muted">Memuat…</td></tr>
          </tbody>
        </table>
      </div>
      <p class="muted small" style="margin: 0">
        Model bawaan berasal dari env (<code>KENARI_*_MODELS</code>, <code>OPENROUTER_*_MODELS</code>, dst.) dan tidak bisa dihapus — nonaktifkan saja kalau tidak ingin
        ditawarkan. Model yang sedang dipilih di Konfigurasi AI tidak bisa dinonaktifkan atau dihapus. Model gambar kenari & OpenRouter juga dipakai untuk edit gambar.
        Harga ke user tetap diatur di Konfigurasi harga (Rupiah per hit).
      </p>
    </div>

    <!-- Tambah dari daftar API provider -->
    <div v-if="browsing" class="modal-back" @click.self="browsing = false">
      <div class="modal col" style="width: min(860px, 100%); gap: 10px">
        <h3>Tambah model ke katalog</h3>
        <div class="row" style="gap: 6px; flex-wrap: wrap">
          <button v-for="p in catalogProviders" :key="p.name" class="chip" :class="{ on: rProvider === p.name }" @click="rProvider = p.name">{{ p.label }}</button>
          <span class="sep" />
          <button class="chip" :class="{ on: rKind === 'text' }" @click="rKind = 'text'">Teks</button>
          <button class="chip" :class="{ on: rKind === 'image' }" @click="rKind = 'image'">Gambar</button>
        </div>
        <div class="row" style="gap: 8px; flex-wrap: wrap">
          <input v-model="rq" class="input sm" style="flex: 1; min-width: 180px" placeholder="Cari id / nama model, mis. claude, gemini, flux" />
          <label class="row small"><input v-model="freeOnly" type="checkbox" /> gratis saja</label>
          <button class="btn sm" :disabled="rLoading" title="Ambil ulang daftar dari provider" @click="loadRemote(true)">
            <span v-if="rLoading" class="spin" /><Icon v-else name="refresh" :size="14" />Muat ulang
          </button>
        </div>
        <div v-if="rError" class="note">{{ rError }}</div>
        <div v-else class="tbl-wrap list">
          <table class="tbl">
            <thead>
              <tr>
                <th />
                <th>Model</th>
                <th class="r">Konteks</th>
                <th class="r">{{ rProvider === 'openrouter' ? (rKind === 'image' ? 'Input / output (USD/1M) · gambar' : 'Input / output (USD/1M token)') : 'Harga' }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="m in rFiltered.slice(0, LIMIT)" :key="m.id" :class="{ off: m.inCatalog }" @click="!m.inCatalog && togglePick(m.id)">
                <td><input type="checkbox" :disabled="m.inCatalog" :checked="m.inCatalog || picked.has(m.id)" @click.stop="togglePick(m.id)" /></td>
                <td>
                  <b>{{ m.name }}</b> <span v-if="m.isFree" class="tag ok">gratis</span>
                  <span v-if="m.inputModalities?.includes('image')" class="tag" title="Bisa membaca gambar">vision</span>
                  <span v-if="m.inCatalog" class="tag">sudah di katalog</span>
                  <div class="muted small mono">{{ m.id }}<template v-if="m.maker"> · {{ m.maker }}</template></div>
                </td>
                <td class="r nowrap">{{ ctx(m.contextLength) }}</td>
                <td class="r nowrap small">{{ rProvider === 'openrouter' ? price(m) : m.isFree ? 'gratis' : 'lihat kenari.id' }}</td>
              </tr>
              <tr v-if="!rLoading && !rFiltered.length"><td colspan="4" class="muted">Tidak ada model yang cocok.</td></tr>
              <tr v-if="rLoading && !remote.length"><td colspan="4" class="muted">Memuat daftar model…</td></tr>
            </tbody>
          </table>
        </div>
        <div v-if="rFiltered.length > LIMIT" class="muted small">Menampilkan {{ LIMIT }} dari {{ rFiltered.length }} model — persempit pencarian.</div>
        <div class="row" style="justify-content: flex-end">
          <span class="muted small" style="margin-right: auto">{{ picked.size }} dipilih</span>
          <button class="btn" @click="browsing = false">Batal</button>
          <button class="btn primary" :disabled="adding || !picked.size" @click="addPicked"><span v-if="adding" class="spin" />Tambahkan ke katalog</button>
        </div>
      </div>
    </div>

    <!-- Ubah nama & catatan -->
    <div v-if="editing" class="modal-back" @click.self="editing = null">
      <form class="modal col" @submit.prevent="saveEdit">
        <h3>Ubah model</h3>
        <p class="muted small mono" style="margin: 0">{{ editing.row.choice }}</p>
        <label><span class="label">Nama tampilan</span><input v-model="editing.name" class="input" maxlength="200" :placeholder="editing.row.modelId" /></label>
        <label><span class="label">Catatan (opsional)</span><textarea v-model="editing.notes" class="input" rows="3" maxlength="2000" /></label>
        <div class="row" style="justify-content: flex-end">
          <button type="button" class="btn" @click="editing = null">Batal</button>
          <button class="btn primary" :disabled="savingEdit"><span v-if="savingEdit" class="spin" />Simpan</button>
        </div>
      </form>
    </div>
  </div>
</template>

<style scoped>
.head { justify-content: space-between; align-items: flex-start; gap: 12px; flex-wrap: wrap; }
.filters { gap: 6px; flex-wrap: wrap; }
.badges { display: flex; flex-wrap: wrap; gap: 4px; margin-top: 4px; }
.badges:empty { display: none; }
.mono { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; word-break: break-all; }
.off { opacity: 0.6; }
.list { max-height: 50vh; overflow-y: auto; }
.list tbody tr { cursor: pointer; }
.sep { width: 1px; align-self: stretch; background: var(--line); margin: 0 4px; }
.btn.sm { gap: 4px; }
textarea.input { height: auto; padding-top: 8px; }
</style>
