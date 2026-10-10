<script setup lang="ts">
// System log dari Elasticsearch: activity user (login, hit model AI, aksi admin) dan
// request ke vendor AI.
const api = useApi()
const toast = useToast()

const type = ref<'activity' | 'vendor'>('activity')
const q = ref('')
const action = ref('')
const success = ref<'' | 'true' | 'false'>('')
const page = ref(1)
const data = ref<{ enabled: boolean; error?: string; total: number; items: any[] } | null>(null)
const loading = ref(false)
const open = ref<any>(null)

const ACTIONS = {
  activity: ['auth.login', 'auth.register', 'auth.guest', 'auth.upgrade', 'ai.design', 'ai.refine', 'ai.image', 'ai.image_edit', 'ai.copy', 'admin.balance', 'admin.user_role', 'admin.ai_config', 'admin.pricing'],
  vendor: ['chat', 'image', 'edit'],
}

async function load() {
  loading.value = true
  try {
    data.value = await api.get('/api/admin/logs', { type: type.value, q: q.value, action: action.value, success: success.value, page: page.value })
  } catch (e: any) {
    toast.error(e.message)
  } finally {
    loading.value = false
  }
}
onMounted(load)
watch(type, () => ((action.value = ''), (page.value = 1), load()))
watch([action, success], () => ((page.value = 1), load()))
const pages = computed(() => Math.max(1, Math.ceil(Math.min(data.value?.total || 0, 10000) / 50)))

const fmtDate = (s: string) => new Date(s).toLocaleString('id-ID', { dateStyle: 'short', timeStyle: 'medium' })
</script>

<template>
  <div class="adm">
    <div class="card sec">
      <div class="bar">
        <button class="chip" :class="{ on: type === 'activity' }" @click="type = 'activity'">Activity user</button>
        <button class="chip" :class="{ on: type === 'vendor' }" @click="type = 'vendor'">Request ke vendor AI</button>
        <div class="grow" />
        <select v-model="action" class="select sm" style="width: 170px">
          <option value="">Semua {{ type === 'activity' ? 'aksi' : 'operasi' }}</option>
          <option v-for="a in ACTIONS[type]" :key="a" :value="a">{{ a }}</option>
        </select>
        <select v-model="success" class="select sm" style="width: 120px">
          <option value="">Semua status</option><option value="true">Sukses</option><option value="false">Gagal</option>
        </select>
        <input v-model="q" class="input sm" placeholder="Cari (email, request id, error…)" style="width: 240px" @keydown.enter="(page = 1), load()" />
        <button class="btn sm" :disabled="loading" @click="(page = 1), load()"><span v-if="loading" class="spin" />Cari</button>
      </div>
      <div v-if="data && !data.enabled" class="note">{{ data.error }}. Isi <code>LOG_ES_URL</code> di env backend (lihat README) untuk menyimpan & mencari log.</div>
      <div class="tbl-wrap">
        <table v-if="type === 'activity'" class="tbl">
          <thead><tr><th>Waktu</th><th>Aksi</th><th>User</th><th>Status</th><th class="r">Durasi</th><th>IP</th><th>Error / detail</th></tr></thead>
          <tbody>
            <tr v-for="e in data?.items" :key="e._id" class="click" @click="open = e">
              <td class="nowrap small">{{ fmtDate(e['@timestamp']) }}</td>
              <td><span class="tag acc">{{ e.action }}</span></td>
              <td class="small">{{ e.email || e.details?.email || '—' }}</td>
              <td><span class="tag" :class="e.success ? 'ok' : 'bad'">{{ e.status }}</span></td>
              <td class="r small nowrap">{{ e.duration_ms }} ms</td>
              <td class="small">{{ e.ip }}</td>
              <td class="small trunc">{{ e.error || (e.details?.charged !== undefined ? `potong Rp ${e.details.charged}` : '') }}</td>
            </tr>
          </tbody>
        </table>
        <table v-else class="tbl">
          <thead><tr><th>Waktu</th><th>Fitur</th><th>Operasi</th><th>Provider · model</th><th>Status</th><th class="r">Durasi</th><th class="r">Harga vendor</th><th>Error</th></tr></thead>
          <tbody>
            <tr v-for="e in data?.items" :key="e._id" class="click" @click="open = e">
              <td class="nowrap small">{{ fmtDate(e['@timestamp']) }}</td>
              <td class="small">{{ e.feature }}</td>
              <td><span class="tag">{{ e.operation }}</span></td>
              <td class="small">{{ e.provider }} · {{ e.model }}</td>
              <td><span class="tag" :class="e.success ? 'ok' : 'bad'">{{ e.success ? 'ok' : e.status || 'gagal' }}</span></td>
              <td class="r small nowrap">{{ e.duration_ms }} ms</td>
              <td class="r small nowrap">{{ e.vendor_price ?? '—' }}</td>
              <td class="small trunc">{{ e.error_code || e.error }}</td>
            </tr>
          </tbody>
        </table>
        <p v-if="data?.enabled && !data.items.length" class="muted">Tidak ada log.</p>
      </div>
      <div v-if="pages > 1" class="pager">
        <span class="muted">{{ data?.total }} log</span>
        <button class="btn sm" :disabled="page <= 1" @click="page--, load()">‹</button>
        <span>{{ page }} / {{ pages }}</span>
        <button class="btn sm" :disabled="page >= pages" @click="page++, load()">›</button>
      </div>
    </div>

    <div v-if="open" class="modal-back" @click.self="open = null">
      <div class="modal" style="width: min(760px, 100%)">
        <h3>Detail log</h3>
        <pre class="json">{{ JSON.stringify(open, null, 2) }}</pre>
        <div class="row" style="justify-content: flex-end"><button class="btn" @click="open = null">Tutup</button></div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.click { cursor: pointer; }
.click:hover { background: var(--panel-2); }
.trunc { max-width: 320px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.json { background: var(--panel-2); border-radius: 8px; padding: 12px; font-size: 12px; overflow: auto; max-height: 60vh; white-space: pre-wrap; word-break: break-word; }
</style>
