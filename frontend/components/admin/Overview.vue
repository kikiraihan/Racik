<script setup lang="ts">
// Ringkasan: angka utama (user, generate AI, pemakaian saldo), grafik 14 hari, pemakaian per
// fitur, status sistem, dan transaksi saldo terbaru.
import { rupiah } from '~/composables/useApi'

interface Day { date: string; aiHits: number; revenue: number; signups: number }
interface Overview {
  users: { total: number; guests: number; admins: number; new7d: number; balance: number }
  content: { designs: number; assets: number }
  ai: { hitsToday: number; hits7d: number; liked: number; disliked: number; configured: boolean; textModel: string; imageModel: string; editModel: string }
  wallet: { revenueToday: number; revenue30d: number; topup30d: number }
  daily: Day[]
  byFeature: { kind: string; count: number }[]
  recentTx: { id: string; email: string; amount: number; kind: string; feature: string; createdAt: string }[]
  system: { elasticsearch: boolean; googleLogin: boolean }
  serverLog?: { capacity: number; stored: number; total: number; levels: Record<string, number>; startedAt: string }
}

const api = useApi()
const toast = useToast()
const data = ref<Overview | null>(null)
const loading = ref(false)

async function load() {
  loading.value = true
  try {
    data.value = await api.get('/api/admin/overview')
  } catch (e: any) {
    toast.error(e.message)
  } finally {
    loading.value = false
  }
}
onMounted(load)

const num = (n: number) => (n ?? 0).toLocaleString('id-ID')

// --- Grafik 14 hari (satu seri, metrik bisa dipilih) ---
const METRICS = [
  { key: 'aiHits', label: 'Generate AI', fmt: num },
  { key: 'revenue', label: 'Pemakaian saldo', fmt: rupiah },
  { key: 'signups', label: 'Pendaftar baru', fmt: num },
] as const
const metric = ref<(typeof METRICS)[number]['key']>('aiHits')
const metricDef = computed(() => METRICS.find((m) => m.key === metric.value)!)
const days = computed(() => data.value?.daily ?? [])
const maxVal = computed(() => Math.max(1, ...days.value.map((d) => d[metric.value])))
const totalVal = computed(() => days.value.reduce((s, d) => s + d[metric.value], 0))
const hover = ref<number | null>(null)
const dayLabel = (s: string) => new Date(s + 'T00:00:00').toLocaleDateString('id-ID', { day: 'numeric', month: 'short' })
const weekday = (s: string) => new Date(s + 'T00:00:00').toLocaleDateString('id-ID', { weekday: 'long', day: 'numeric', month: 'long' })

// --- Pemakaian per fitur ---
const FEATURE: Record<string, string> = { design: 'Magic Design', refine: 'Perbaiki desain', image: 'Gambar AI', edit: 'Edit gambar', copy: 'Copywriting' }
const maxFeature = computed(() => Math.max(1, ...(data.value?.byFeature ?? []).map((f) => f.count)))
const likeRate = computed(() => {
  const a = data.value?.ai
  if (!a || a.liked + a.disliked === 0) return null
  return Math.round((a.liked / (a.liked + a.disliked)) * 100)
})

const KIND: Record<string, string> = { topup: 'top up', charge: 'pakai AI', refund: 'refund', adjust: 'koreksi' }
const fmtTime = (s: string) => new Date(s).toLocaleString('id-ID', { dateStyle: 'short', timeStyle: 'short' })
</script>

<template>
  <div class="adm">
    <div class="sec-h">
      <p class="muted small" style="margin: 0">Gambaran cepat pemakaian Racik.</p>
      <div class="grow" />
      <button class="btn sm" :disabled="loading" @click="load"><span v-if="loading" class="spin" /><Icon v-else name="refresh" :size="14" /> Muat ulang</button>
    </div>

    <template v-if="data">
      <!-- Angka utama -->
      <div class="stats six">
        <div class="card stat">
          <span class="k"><span class="ic"><Icon name="users" :size="15" /></span> Pengguna terdaftar</span>
          <span class="v">{{ num(data.users.total) }}</span>
          <span class="s">+{{ num(data.users.new7d) }} minggu ini · {{ num(data.users.guests) }} tamu · {{ num(data.users.admins) }} admin</span>
        </div>
        <div class="card stat">
          <span class="k"><span class="ic"><Icon name="sparkles" :size="15" /></span> Generate AI hari ini</span>
          <span class="v">{{ num(data.ai.hitsToday) }}</span>
          <span class="s">{{ num(data.ai.hits7d) }} dalam 7 hari terakhir</span>
        </div>
        <div class="card stat">
          <span class="k"><span class="ic"><Icon name="chart" :size="15" /></span> Pemakaian saldo (30 hari)</span>
          <span class="v">{{ rupiah(data.wallet.revenue30d) }}</span>
          <span class="s">Hari ini {{ rupiah(data.wallet.revenueToday) }}</span>
        </div>
        <div class="card stat">
          <span class="k"><span class="ic"><Icon name="wallet" :size="15" /></span> Top up (30 hari)</span>
          <span class="v">{{ rupiah(data.wallet.topup30d) }}</span>
          <span class="s">Saldo beredar {{ rupiah(data.users.balance) }}</span>
        </div>
        <div class="card stat">
          <span class="k"><span class="ic"><Icon name="file" :size="15" /></span> Desain tersimpan</span>
          <span class="v">{{ num(data.content.designs) }}</span>
          <span class="s">{{ num(data.content.assets) }} aset (upload & gambar AI)</span>
        </div>
        <div class="card stat">
          <span class="k"><span class="ic"><Icon name="heart" :size="15" /></span> Kepuasan hasil AI</span>
          <span class="v">{{ likeRate === null ? '—' : likeRate + '%' }}</span>
          <span class="s">👍 {{ num(data.ai.liked) }} · 👎 {{ num(data.ai.disliked) }}</span>
        </div>
      </div>

      <div class="grid-main">
        <!-- Grafik 14 hari -->
        <div class="card sec chart-card">
          <div class="sec-h">
            <h2>{{ metricDef.label }} · 14 hari</h2>
            <div class="grow" />
            <button v-for="m in METRICS" :key="m.key" class="chip" :class="{ on: metric === m.key }" @click="metric = m.key">{{ m.label }}</button>
          </div>
          <p class="muted small" style="margin: -4px 0 12px">Total {{ metricDef.fmt(totalVal) }}</p>
          <div class="bars" role="img" :aria-label="`${metricDef.label} per hari, 14 hari terakhir`" @mouseleave="hover = null">
            <div class="gridline" style="bottom: 100%"><span>{{ metricDef.fmt(maxVal) }}</span></div>
            <div class="gridline" style="bottom: 50%"><span>{{ metricDef.fmt(Math.round(maxVal / 2)) }}</span></div>
            <div
              v-for="(d, i) in days"
              :key="d.date"
              class="bar-col"
              :class="{ dim: hover !== null && hover !== i }"
              @mouseenter="hover = i"
              @focus="hover = i"
              tabindex="0"
            >
              <div class="bar" :style="{ height: `${(d[metric] / maxVal) * 100}%` }" />
              <div v-if="hover === i" class="tip" :class="{ left: i > days.length - 4 }">
                <b>{{ metricDef.fmt(d[metric]) }}</b>
                <span>{{ weekday(d.date) }}</span>
              </div>
            </div>
          </div>
          <div class="xlabels">
            <span v-for="(d, i) in days" :key="d.date">{{ i % 2 === (days.length - 1) % 2 ? dayLabel(d.date) : '' }}</span>
          </div>
        </div>

        <!-- Status sistem -->
        <div class="card sec">
          <div class="sec-h"><h2>Status sistem</h2></div>
          <ul class="status">
            <li>
              <span class="dot" :class="data.ai.configured ? 'ok' : 'bad'" />
              <div class="grow"><b>Provider AI</b><div class="muted small">{{ data.ai.configured ? 'Aktif' : 'Belum ada API key provider' }}</div></div>
              <NuxtLink to="/admin/ai" class="btn sm ghost">Atur</NuxtLink>
            </li>
            <li class="models">
              <div class="small"><span class="muted">Teks</span> <span class="tag acc">{{ data.ai.textModel || '—' }}</span></div>
              <div class="small"><span class="muted">Gambar</span> <span class="tag acc">{{ data.ai.imageModel || '—' }}</span></div>
              <div class="small"><span class="muted">Edit</span> <span class="tag acc">{{ data.ai.editModel || '—' }}</span></div>
            </li>
            <li>
              <span class="dot" :class="data.system.elasticsearch ? 'ok' : 'warn'" />
              <div class="grow"><b>Elasticsearch</b><div class="muted small">{{ data.system.elasticsearch ? 'System log tersimpan & bisa dicari' : 'Belum diatur — log hanya di stdout' }}</div></div>
              <NuxtLink to="/admin/logs" class="btn sm ghost">Lihat</NuxtLink>
            </li>
            <li>
              <span class="dot" :class="data.system.googleLogin ? 'ok' : 'off'" />
              <div class="grow"><b>Login Google</b><div class="muted small">{{ data.system.googleLogin ? 'Aktif' : 'Nonaktif (GOOGLE_CLIENT_ID kosong)' }}</div></div>
            </li>
            <li v-if="data.serverLog">
              <span class="dot" :class="data.serverLog.levels.error ? 'bad' : data.serverLog.levels.warn ? 'warn' : 'ok'" />
              <div class="grow">
                <b>Log server</b>
                <div class="muted small">{{ num(data.serverLog.levels.error) }} error · {{ num(data.serverLog.levels.warn) }} peringatan di {{ num(data.serverLog.stored) }} baris terakhir</div>
              </div>
              <NuxtLink to="/admin/server-log" class="btn sm ghost">Lihat</NuxtLink>
            </li>
          </ul>
        </div>

        <!-- Pemakaian per fitur -->
        <div class="card sec">
          <div class="sec-h"><h2>Generate AI per fitur · 30 hari</h2></div>
          <p v-if="!data.byFeature.length" class="muted small">Belum ada generate AI.</p>
          <div v-for="f in data.byFeature" :key="f.kind" class="hbar">
            <span class="small hlabel">{{ FEATURE[f.kind] || f.kind }}</span>
            <div class="htrack"><div class="hfill" :style="{ width: `${(f.count / maxFeature) * 100}%` }" /></div>
            <b class="small hval">{{ num(f.count) }}</b>
          </div>
        </div>

        <!-- Transaksi terbaru -->
        <div class="card sec">
          <div class="sec-h">
            <h2>Transaksi saldo terbaru</h2>
            <div class="grow" />
            <NuxtLink to="/admin/users" class="btn sm ghost">Pengguna <Icon name="right" :size="14" /></NuxtLink>
          </div>
          <p v-if="!data.recentTx.length" class="muted small">Belum ada transaksi.</p>
          <ul class="txs">
            <li v-for="t in data.recentTx" :key="t.id">
              <div class="grow" style="min-width: 0">
                <div class="small trunc"><b>{{ t.email }}</b></div>
                <div class="muted small">{{ fmtTime(t.createdAt) }} · {{ KIND[t.kind] || t.kind }}<template v-if="t.feature"> · {{ FEATURE[t.feature] || t.feature }}</template></div>
              </div>
              <b class="small nowrap" :class="t.amount > 0 ? 'pos' : ''">{{ t.amount > 0 ? '+' : '' }}{{ rupiah(t.amount) }}</b>
            </li>
          </ul>
        </div>
      </div>
    </template>
    <div v-else-if="loading" class="stats">
      <div v-for="i in 6" :key="i" class="card stat shimmer" style="height: 104px" />
    </div>
  </div>
</template>

<style scoped>
.stats.six { grid-template-columns: repeat(6, minmax(0, 1fr)); }
.grid-main { display: grid; grid-template-columns: minmax(0, 1.7fr) minmax(0, 1fr); gap: 16px; }
.grid-main > .sec { margin-bottom: 0; }

/* Grafik batang: 4px ujung membulat di atas, celah antar batang, garis bantu samar. */
.bars { position: relative; height: 200px; display: flex; align-items: flex-end; gap: 2px; padding-right: 64px; margin-top: 18px; }
.gridline { position: absolute; left: 0; right: 0; border-top: 1px dashed var(--line); pointer-events: none; }
.gridline span { position: absolute; right: 0; top: -8px; width: 60px; text-align: right; font-size: 11px; color: var(--muted); }
.bar-col { flex: 1; height: 100%; display: flex; align-items: flex-end; position: relative; cursor: default; outline: none; padding: 0 2px; }
.bar { width: 100%; min-height: 2px; background: var(--accent); border-radius: 4px 4px 0 0; transition: opacity .12s, height .25s ease; }
.bar-col.dim .bar { opacity: .35; }
.tip {
  position: absolute; bottom: calc(100% + 6px); left: 50%; transform: translateX(-50%); z-index: 3; white-space: nowrap;
  background: var(--panel); border: 1px solid var(--line); box-shadow: var(--shadow-lg); border-radius: 8px; padding: 6px 10px;
  display: flex; flex-direction: column; font-size: 12px; pointer-events: none;
}
.tip.left { left: auto; right: 0; transform: none; }
.tip span { color: var(--muted); }
.bar-col:focus-visible .bar { outline: 2px solid var(--accent-2); outline-offset: 2px; }
.xlabels { display: flex; gap: 2px; margin-top: 6px; padding-right: 64px; border-top: 1px solid var(--line); padding-top: 6px; }
.xlabels span { flex: 1; text-align: center; font-size: 11px; color: var(--muted); white-space: nowrap; }

.status { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; }
.status li { display: flex; align-items: center; gap: 10px; padding: 10px 0; border-top: 1px solid var(--line); }
.status li:first-child { border-top: 0; padding-top: 0; }
.status li.models { flex-wrap: wrap; gap: 6px 12px; padding-left: 20px; border-top: 0; padding-top: 0; }
.dot { width: 10px; height: 10px; border-radius: 50%; flex: none; background: var(--muted); }
.dot.ok { background: var(--ok); box-shadow: 0 0 0 3px var(--ok-soft); }
.dot.bad { background: var(--danger); box-shadow: 0 0 0 3px var(--bad-soft); }
.dot.warn { background: #f5a524; box-shadow: 0 0 0 3px var(--warn-soft); }
.dot.off { background: var(--line); }

.hbar { display: grid; grid-template-columns: 120px 1fr 50px; align-items: center; gap: 10px; padding: 6px 0; }
.htrack { height: 10px; background: var(--panel-2); border-radius: 999px; overflow: hidden; }
.hfill { height: 100%; background: var(--accent); border-radius: 999px; transition: width .25s ease; }
.hval { text-align: right; }

.txs { list-style: none; margin: 0; padding: 0; }
.txs li { display: flex; align-items: center; gap: 10px; padding: 8px 0; border-top: 1px solid var(--line); }
.txs li:first-child { border-top: 0; }
.pos { color: var(--ok); }
.trunc { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

@media (max-width: 1600px) { .stats.six { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
@media (max-width: 1100px) { .grid-main { grid-template-columns: 1fr; } }
@media (max-width: 640px) { .stats.six { grid-template-columns: repeat(2, minmax(0, 1fr)); } .stat .v { font-size: 20px; } }
@media (max-width: 520px) { .chart-card .sec-h { flex-wrap: wrap; } .hbar { grid-template-columns: 96px 1fr 40px; } }
</style>
