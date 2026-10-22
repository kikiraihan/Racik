<script setup lang="ts">
// AI API Call Log: setiap request HTTP ke vendor AI (termasuk percobaan ulang & model
// cadangan) dari tabel ai_api_calls — provider, model, status, latensi, token, biaya.
// Pola sama dengan AI API Call Log di tiara. Selalu tersedia (tidak butuh Elasticsearch).
import { FEATURES, OPERATIONS, featureLabel, fmtCost, fmtDate, fmtNum, fmtRp, type AiCall } from '~/lib/aiCallLog'

interface SummaryItem {
  feature: string | null
  provider: string
  model: string
  operation: string
  calls: number
  errors: number
  avgMs: number | null
  p95Ms: number | null
  tokens: number | null
  cost: number | null
  vendorPrice: number | null
}
interface Summary {
  days: number
  items: SummaryItem[]
  total: { calls: number; errors: number; tokens: number | null; cost: number | null; vendorPrice: number | null }
}

const api = useApi()
const toast = useToast()
const route = useRoute()

// ---- ringkasan per fitur & model ----
const days = ref(7)
const summary = ref<Summary | null>(null)
const summaryLoading = ref(false)
async function loadSummary() {
  summaryLoading.value = true
  try {
    summary.value = await api.get<Summary>('/api/admin/ai-calls/summary', { days: days.value })
  } catch (e: any) {
    toast.error(e.message)
  } finally {
    summaryLoading.value = false
  }
}
watch(days, loadSummary)

// ---- daftar ----
const q = ref('')
const operation = ref('')
const feature = ref('')
const success = ref<'' | 'true' | 'false'>('')
const model = ref('')
const from = ref('')
const to = ref('')
// Dari halaman detail: semua panggilan AI dalam satu request user (?requestId=...).
const requestId = ref(String(route.query.requestId || ''))
const sort = ref('')
const dir = ref<'asc' | 'desc'>('desc')
const page = ref(1)
const SIZE = 50
const data = ref<{ items: AiCall[]; total: number; bodies: boolean; retentionDays: number } | null>(null)
const loading = ref(false)
const selected = ref<number[]>([])

async function load() {
  loading.value = true
  try {
    data.value = await api.get('/api/admin/ai-calls', {
      q: q.value, operation: operation.value, feature: feature.value, success: success.value, model: model.value.trim(),
      from: from.value, to: to.value, requestId: requestId.value, sort: sort.value, dir: dir.value, page: page.value, size: SIZE,
    })
    selected.value = []
  } catch (e: any) {
    toast.error(e.message)
  } finally {
    loading.value = false
  }
}
const search = () => ((page.value = 1), load())
watch([operation, feature, success, from, to, requestId], search)
const pages = computed(() => Math.max(1, Math.ceil((data.value?.total || 0) / SIZE)))

// Klik judul kolom: urutkan turun → naik → bawaan (terbaru dulu).
function sortBy(key: string) {
  if (sort.value !== key) {
    sort.value = key
    dir.value = 'desc'
  } else if (dir.value === 'desc') {
    dir.value = 'asc'
  } else {
    sort.value = ''
    dir.value = 'desc'
  }
  load()
}
const sortIcon = (key: string) => (sort.value !== key ? '' : dir.value === 'desc' ? '↓' : '↑')

const allChecked = computed(() => !!data.value?.items.length && selected.value.length === data.value.items.length)
function toggleAll() {
  selected.value = allChecked.value ? [] : (data.value?.items || []).map((i) => i.id)
}

onMounted(() => {
  loadSummary()
  load()
})

// ---- hapus ----
const toDelete = ref<{ ids?: number[]; all?: boolean; text: string } | null>(null)
const deleting = ref(false)
async function confirmDelete() {
  if (!toDelete.value) return
  deleting.value = true
  try {
    const d = toDelete.value
    const res = await api.post<{ message: string }>('/api/admin/ai-calls/delete', d.all ? { all: true } : { ids: d.ids })
    toast.success(res.message)
    toDelete.value = null
    load()
    loadSummary()
  } catch (e: any) {
    toast.error(e.message)
  } finally {
    deleting.value = false
  }
}

const open = (c: AiCall) => navigateTo(`/admin/ai-calls/${c.id}`)
</script>

<template>
  <div class="adm">
    <!-- ringkasan -->
    <div class="card sec">
      <div class="bar">
        <div class="sec-h" style="margin: 0">
          <h2>Ringkasan pemakaian AI</h2>
          <span v-if="summaryLoading" class="spin" />
        </div>
        <div class="grow" />
        <select v-model.number="days" class="select sm" style="width: 120px">
          <option :value="1">24 jam</option>
          <option :value="7">7 hari</option>
          <option :value="30">30 hari</option>
        </select>
      </div>
      <p class="muted small" style="margin: -4px 0 12px">
        Per fitur & model. Token = input + output (kalau dilaporkan vendor). Biaya USD dari laporan vendor (mis. OpenRouter); harga vendor Rupiah dari menu
        <NuxtLink to="/admin/pricing" class="lnk">Konfigurasi harga</NuxtLink> untuk panggilan yang sukses.
      </p>
      <template v-if="summary">
        <div class="stats mini">
          <div class="card stat"><span class="k">Panggilan</span><span class="v">{{ fmtNum(summary.total.calls) }}</span></div>
          <div class="card stat">
            <span class="k">Error</span><span class="v" :class="{ bad: summary.total.errors }">{{ fmtNum(summary.total.errors) }}</span>
          </div>
          <div class="card stat"><span class="k">Token</span><span class="v">{{ fmtNum(summary.total.tokens) }}</span></div>
          <div class="card stat"><span class="k">Biaya vendor (USD)</span><span class="v">{{ fmtCost(summary.total.cost) }}</span></div>
          <div class="card stat"><span class="k">Harga vendor (Rp)</span><span class="v">{{ fmtRp(summary.total.vendorPrice) }}</span></div>
        </div>
        <div v-if="summary.items.length" class="tbl-wrap">
          <table class="tbl">
            <thead>
              <tr>
                <th>Fitur</th><th>Provider · model</th><th class="r">Panggilan</th><th class="r">Error</th>
                <th class="r">Rata-rata / p95</th><th class="r">Token</th><th class="r">Biaya</th><th class="r">Harga vendor</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(it, i) in summary.items" :key="i">
                <td>{{ featureLabel(it.feature) }}</td>
                <td class="small">
                  <span class="tag" :class="OPERATIONS[it.operation]?.cls">{{ OPERATIONS[it.operation]?.text || it.operation }}</span>
                  <span class="mono nowrap">{{ it.provider }} · {{ it.model }}</span>
                </td>
                <td class="r">{{ fmtNum(it.calls) }}</td>
                <td class="r" :class="{ bad: it.errors }">{{ fmtNum(it.errors) }}</td>
                <td class="r nowrap small">{{ fmtNum(Math.round(it.avgMs || 0)) }} / {{ fmtNum(Math.round(it.p95Ms || 0)) }} ms</td>
                <td class="r">{{ fmtNum(it.tokens) }}</td>
                <td class="r nowrap">{{ fmtCost(it.cost) }}</td>
                <td class="r nowrap">{{ fmtRp(it.vendorPrice) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <p v-else class="muted">Belum ada panggilan API AI pada rentang ini.</p>
      </template>
    </div>

    <!-- daftar -->
    <div class="card sec">
      <div class="bar">
        <input v-model="q" class="input sm" placeholder="Cari model / fitur / email / request id / error" style="width: 280px" @keydown.enter="search" />
        <select v-model="success" class="select sm" style="width: 120px">
          <option value="">Semua status</option><option value="true">Sukses</option><option value="false">Error</option>
        </select>
        <select v-model="operation" class="select sm" style="width: 140px">
          <option value="">Semua jenis</option>
          <option v-for="(o, k) in OPERATIONS" :key="k" :value="k">{{ o.text }}</option>
        </select>
        <select v-model="feature" class="select sm" style="width: 160px">
          <option value="">Semua fitur</option>
          <option v-for="(label, k) in FEATURES" :key="k" :value="k">{{ label }}</option>
        </select>
        <input v-model="model" class="input sm" placeholder="Model (ID persis)" style="width: 170px" @keydown.enter="search" />
        <input v-model="from" type="date" class="input sm" title="Dari tanggal" style="width: 140px" />
        <input v-model="to" type="date" class="input sm" title="Sampai tanggal" style="width: 140px" />
        <button class="btn sm" :disabled="loading" @click="search"><span v-if="loading" class="spin" />Cari</button>
        <div class="grow" />
        <button v-if="selected.length" class="btn sm danger" @click="toDelete = { ids: [...selected], text: `Hapus ${selected.length} baris log?` }">
          <Icon name="trash" :size="14" /> Hapus ({{ selected.length }})
        </button>
        <button class="btn sm danger" @click="toDelete = { all: true, text: 'Hapus SEMUA AI API Call Log?' }"><Icon name="trash" :size="14" /> Hapus semua</button>
      </div>
      <div v-if="requestId" class="bar">
        <span class="chip on small">Request id: <span class="mono">{{ requestId }}</span></span>
        <button class="btn sm" @click="(requestId = ''), navigateTo('/admin/ai-calls', { replace: true })"><Icon name="x" :size="14" /> Hapus filter</button>
      </div>
      <div v-if="data && !data.bodies" class="note">Body request & response tidak disimpan (<code>AI_CALL_LOG_BODIES=false</code>).</div>

      <div class="tbl-wrap">
        <table class="tbl">
          <thead>
            <tr>
              <th style="width: 28px"><input type="checkbox" :checked="allChecked" @change="toggleAll" /></th>
              <th class="sortable" @click="sortBy('createdAt')">Waktu {{ sortIcon('createdAt') }}</th>
              <th class="sortable" @click="sortBy('feature')">Fitur {{ sortIcon('feature') }}</th>
              <th class="sortable" @click="sortBy('model')">Provider · model {{ sortIcon('model') }}</th>
              <th>Status</th>
              <th class="r sortable" @click="sortBy('latencyMs')">Latensi {{ sortIcon('latencyMs') }}</th>
              <th class="r sortable" @click="sortBy('totalTokens')">Token {{ sortIcon('totalTokens') }}</th>
              <th class="r sortable" @click="sortBy('cost')">Biaya {{ sortIcon('cost') }}</th>
              <th class="r sortable" @click="sortBy('vendorPrice')">Harga vendor {{ sortIcon('vendorPrice') }}</th>
              <th>User</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="c in data?.items" :key="c.id" class="click" @click="open(c)">
              <td @click.stop><input v-model="selected" type="checkbox" :value="c.id" /></td>
              <td class="nowrap small">{{ fmtDate(c.createdAt) }}</td>
              <td>
                <div>{{ featureLabel(c.feature) }}</div>
                <span class="tag" :class="OPERATIONS[c.operation]?.cls">{{ OPERATIONS[c.operation]?.text || c.operation }}</span>
              </td>
              <td class="small">
                <div class="mono">{{ c.model }}</div>
                <div class="muted">{{ c.provider }} · <span class="mono">{{ c.endpoint }}</span></div>
              </td>
              <td>
                <span class="tag" :class="c.isSuccess ? 'ok' : 'bad'">{{ c.isSuccess ? 'Sukses' : 'Error' }}<template v-if="c.httpStatus"> · {{ c.httpStatus }}</template></span>
                <div v-if="c.errorMessage" class="small err trunc" :title="c.errorMessage">{{ c.errorMessage }}</div>
              </td>
              <td class="r nowrap small">{{ c.latencyMs === null ? '—' : fmtNum(c.latencyMs) + ' ms' }}</td>
              <td class="r small" :title="`input ${c.promptTokens ?? '—'} · output ${c.completionTokens ?? '—'}`">{{ fmtNum(c.totalTokens) }}</td>
              <td class="r nowrap small">{{ fmtCost(c.cost) }}</td>
              <td class="r nowrap small">{{ fmtRp(c.vendorPrice) }}</td>
              <td class="small trunc">{{ c.email || '—' }}</td>
            </tr>
          </tbody>
        </table>
        <p v-if="data && !data.items.length" class="muted">Tidak ada log.</p>
      </div>
      <div class="pager">
        <span class="muted">{{ fmtNum(data?.total || 0) }} log<template v-if="data?.retentionDays"> · disimpan {{ data.retentionDays }} hari</template></span>
        <template v-if="pages > 1">
          <button class="btn sm" :disabled="page <= 1" @click="page--, load()">‹</button>
          <span>{{ page }} / {{ pages }}</span>
          <button class="btn sm" :disabled="page >= pages" @click="page++, load()">›</button>
        </template>
      </div>
    </div>

    <div v-if="toDelete" class="modal-back" @click.self="toDelete = null">
      <div class="modal">
        <h3>Hapus log</h3>
        <p>{{ toDelete.text }}</p>
        <div class="row" style="justify-content: flex-end">
          <button class="btn" @click="toDelete = null">Batal</button>
          <button class="btn primary" :disabled="deleting" @click="confirmDelete"><span v-if="deleting" class="spin" />Hapus</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.stats.mini { grid-template-columns: repeat(auto-fill, minmax(140px, 1fr)); }
.stats.mini .v { font-size: 20px; }
.click { cursor: pointer; }
.click:hover { background: var(--panel-2); }
.sortable { cursor: pointer; user-select: none; }
.sortable:hover { color: var(--ink); }
.trunc { max-width: 260px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.mono { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: 12px; word-break: break-all; }
.mono.nowrap { white-space: nowrap; word-break: normal; }
.bad { color: var(--danger); }
.err { color: var(--danger); margin-top: 4px; }
.lnk { color: var(--accent); font-weight: 600; }
</style>
