<script setup lang="ts">
// Log server: baris log backend terakhir yang disimpan di memori (access log HTTP,
// peringatan, error, log yang gagal dikirim ke Elasticsearch). Tetap tersedia walaupun
// Elasticsearch belum diatur; isinya hilang saat backend restart.
interface Line { seq: number; time: string; level: 'info' | 'warn' | 'error'; source: 'http' | 'app' | 'log'; msg: string }
interface Stats { capacity: number; stored: number; total: number; levels: Record<string, number>; startedAt: string }

const api = useApi()
const toast = useToast()

const q = ref('')
const level = ref<'' | 'info' | 'warn' | 'error'>('')
const source = ref<'' | 'http' | 'app' | 'log'>('')
const lines = ref<Line[]>([])
const stats = ref<Stats | null>(null)
const enabled = ref(true)
const lastSeq = ref(0)
const loading = ref(false)
const live = ref(true)
const wrap = ref(true)
const MAX = 1000
let timer: ReturnType<typeof setInterval> | undefined

const params = () => ({ q: q.value, level: level.value, source: source.value })

async function load() {
  loading.value = true
  try {
    const res = await api.get<{ enabled: boolean; items: Line[]; lastSeq: number; stats?: Stats }>('/api/admin/server-logs', { ...params(), limit: 500 })
    enabled.value = res.enabled
    lines.value = res.items
    lastSeq.value = res.lastSeq
    stats.value = res.stats ?? null
  } catch (e: any) {
    toast.error(e.message)
  } finally {
    loading.value = false
  }
}

// Auto refresh: ambil baris baru saja (seq > lastSeq) lalu taruh di atas.
async function poll() {
  if (loading.value || document.hidden) return
  try {
    const res = await api.get<{ items: Line[]; lastSeq: number; stats?: Stats }>('/api/admin/server-logs', { ...params(), after: lastSeq.value, limit: 500 })
    if (res.items.length) lines.value = [...res.items, ...lines.value].slice(0, MAX)
    lastSeq.value = res.lastSeq
    stats.value = res.stats ?? stats.value
  } catch {}
}

function setLive(v: boolean) {
  live.value = v
  clearInterval(timer)
  if (v) timer = setInterval(poll, 3000)
}

onMounted(() => {
  load()
  setLive(true)
})
onBeforeUnmount(() => clearInterval(timer))
watch([level, source], load)
let t: ReturnType<typeof setTimeout> | undefined
watch(q, () => {
  clearTimeout(t)
  t = setTimeout(load, 300)
})

function download() {
  const text = [...lines.value].reverse().map((l) => `${l.time} [${l.level.toUpperCase()}] [${l.source}] ${l.msg}`).join('\n')
  const a = document.createElement('a')
  a.href = URL.createObjectURL(new Blob([text], { type: 'text/plain' }))
  a.download = `racik-server-log-${new Date().toISOString().slice(0, 19).replace(/[:T]/g, '-')}.txt`
  a.click()
  URL.revokeObjectURL(a.href)
}

async function copy(l: Line) {
  try {
    await navigator.clipboard.writeText(`${l.time} [${l.level.toUpperCase()}] ${l.msg}`)
    toast.success('Baris log disalin')
  } catch {}
}

const fmtTime = (s: string) => new Date(s).toLocaleTimeString('id-ID', { hour12: false })
const fmtDate = (s: string) => new Date(s).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' })
</script>

<template>
  <div class="adm">
    <div v-if="stats" class="stats mini">
      <button class="card stat" :class="{ sel: level === '' }" @click="level = ''">
        <span class="k">Tersimpan</span><span class="v">{{ stats.stored.toLocaleString('id-ID') }}</span>
        <span class="s">dari kapasitas {{ stats.capacity.toLocaleString('id-ID') }} baris</span>
      </button>
      <button class="card stat" :class="{ sel: level === 'error' }" @click="level = 'error'">
        <span class="k"><span class="lv error" /> Error</span><span class="v">{{ stats.levels.error.toLocaleString('id-ID') }}</span>
        <span class="s">HTTP 5xx, panic, “gagal …”</span>
      </button>
      <button class="card stat" :class="{ sel: level === 'warn' }" @click="level = 'warn'">
        <span class="k"><span class="lv warn" /> Peringatan</span><span class="v">{{ stats.levels.warn.toLocaleString('id-ID') }}</span>
        <span class="s">HTTP 4xx & peringatan</span>
      </button>
      <div class="card stat">
        <span class="k">Server berjalan sejak</span><span class="v small-v">{{ fmtDate(stats.startedAt) }}</span>
        <span class="s">{{ stats.total.toLocaleString('id-ID') }} baris sejak start</span>
      </div>
    </div>

    <div class="card sec">
      <div class="bar">
        <input v-model="q" class="input sm" placeholder="Cari teks (path, error, request id…)" style="max-width: 300px" />
        <select v-model="level" class="select sm" style="width: 150px">
          <option value="">Semua level</option>
          <option value="error">Error</option>
          <option value="warn">Peringatan + error</option>
          <option value="info">Info</option>
        </select>
        <select v-model="source" class="select sm" style="width: 160px">
          <option value="">Semua sumber</option>
          <option value="http">Access log HTTP</option>
          <option value="app">Aplikasi</option>
          <option value="log">System log (stdout)</option>
        </select>
        <div class="grow" />
        <button class="btn sm" :class="{ active: live }" :title="live ? 'Jeda auto refresh' : 'Nyalakan auto refresh (3 detik)'" @click="setLive(!live)">
          <Icon :name="live ? 'pause' : 'play'" :size="13" /> {{ live ? 'Live' : 'Dijeda' }}
        </button>
        <button class="btn sm icon" title="Bungkus baris panjang" :class="{ active: wrap }" @click="wrap = !wrap"><Icon name="text" :size="14" /></button>
        <button class="btn sm icon" title="Muat ulang" :disabled="loading" @click="load"><span v-if="loading" class="spin" /><Icon v-else name="refresh" :size="14" /></button>
        <button class="btn sm icon" title="Unduh .txt" :disabled="!lines.length" @click="download"><Icon name="download" :size="14" /></button>
      </div>

      <div v-if="!enabled" class="note">Log server tidak tersedia di backend ini.</div>
      <p class="muted small" style="margin: 0 0 10px">
        Disimpan di memori backend (hilang saat restart). Untuk log permanen & pencarian lengkap activity user / request vendor AI, pakai menu
        <NuxtLink to="/admin/logs" class="lnk">System log</NuxtLink>.
      </p>

      <div class="console scroll" :class="{ nowrap: !wrap }">
        <div v-for="l in lines" :key="l.seq" class="ln" :class="l.level" title="Klik untuk menyalin" @click="copy(l)">
          <span class="t">{{ fmtTime(l.time) }}</span>
          <span class="badge-lv" :class="l.level">{{ l.level }}</span>
          <span class="src">{{ l.source }}</span>
          <span class="m">{{ l.msg }}</span>
        </div>
        <div v-if="!lines.length && !loading" class="empty">Tidak ada baris log yang cocok.</div>
      </div>
      <div class="pager"><span class="muted">{{ lines.length }} baris ditampilkan (terbaru di atas)</span></div>
    </div>
  </div>
</template>

<style scoped>
.stats.mini { grid-template-columns: repeat(auto-fill, minmax(190px, 1fr)); }
button.stat { text-align: left; cursor: pointer; font: inherit; color: inherit; }
button.stat:hover { border-color: var(--accent); }
.stat.sel { border-color: var(--accent); box-shadow: 0 0 0 3px var(--accent-soft); }
.small-v { font-size: 16px !important; }
.lv { width: 8px; height: 8px; border-radius: 50%; display: inline-block; }
.lv.error { background: var(--danger); }
.lv.warn { background: #f5a524; }
.lnk { color: var(--accent); font-weight: 600; }

/* Tampilan konsol: selalu gelap, di kedua tema. */
.console {
  background: #0b0c12; color: #d5d8e4; border-radius: 10px; border: 1px solid #1d1f2b;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: 12px; line-height: 1.55;
  max-height: 62vh; min-height: 240px; padding: 8px 0;
}
.ln { display: flex; gap: 10px; padding: 1px 12px; cursor: pointer; align-items: baseline; }
.ln:hover { background: rgba(255, 255, 255, .05); }
.ln.error { background: rgba(255, 90, 95, .08); }
.ln.warn { background: rgba(245, 165, 36, .06); }
.t { color: #6f7590; flex: none; }
.src { color: #7f8ab8; flex: none; width: 34px; }
.m { white-space: pre-wrap; word-break: break-word; min-width: 0; }
.console.nowrap .m { white-space: pre; }
.console.nowrap { overflow-x: auto; }
.badge-lv { flex: none; width: 44px; text-transform: uppercase; font-size: 10px; font-weight: 700; letter-spacing: .04em; color: #8fd19e; }
.badge-lv.warn { color: #ffc46b; }
.badge-lv.error { color: #ff7a7f; }
.empty { color: #6f7590; padding: 24px 12px; text-align: center; }
@media (max-width: 640px) { .src { display: none; } .ln { gap: 6px; } }
</style>
