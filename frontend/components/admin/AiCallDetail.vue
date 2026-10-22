<script setup lang="ts">
// Detail satu baris AI API Call Log: informasi panggilan + body request & response
// (gambar base64 sudah diringkas backend, tidak disimpan utuh).
import { OPERATIONS, featureLabel, fmtCost, fmtDate, fmtNum, fmtRp, type AiCall } from '~/lib/aiCallLog'

const api = useApi()
const toast = useToast()
const route = useRoute()

const c = ref<AiCall | null>(null)
const error = ref('')
onMounted(async () => {
  try {
    c.value = await api.get<AiCall>(`/api/admin/ai-calls/${route.params.id}`)
  } catch (e: any) {
    error.value = e.message
  }
})

const info = computed(() => {
  const x = c.value
  if (!x) return []
  return [
    { k: 'Waktu', v: fmtDate(x.createdAt) },
    { k: 'Status', v: (x.isSuccess ? 'Sukses' : 'Error') + (x.httpStatus ? ` (HTTP ${x.httpStatus})` : ' (tanpa respons HTTP)') },
    { k: 'Fitur', v: featureLabel(x.feature) + (x.feature ? ` — ${x.feature}` : '') },
    { k: 'Jenis', v: OPERATIONS[x.operation]?.text || x.operation },
    { k: 'Provider', v: x.provider },
    { k: 'Model', v: x.model, mono: true },
    { k: 'Endpoint', v: x.endpoint, mono: true },
    { k: 'Latensi', v: x.latencyMs === null ? '—' : fmtNum(x.latencyMs) + ' ms' },
    { k: 'Token (input / output / total)', v: `${fmtNum(x.promptTokens)} / ${fmtNum(x.completionTokens)} / ${fmtNum(x.totalTokens)}` },
    { k: 'Biaya dari vendor', v: fmtCost(x.cost) },
    { k: 'Harga vendor (Konfigurasi harga)', v: fmtRp(x.vendorPrice) },
    { k: 'User', v: x.email || x.userId || '—' },
    { k: 'Request id', v: x.requestId || '—', mono: true },
  ]
})

const pretty = (v: any) => (typeof v === 'string' ? v : JSON.stringify(v, null, 2))

async function copy(v: any) {
  try {
    await navigator.clipboard.writeText(pretty(v))
    toast.success('Disalin')
  } catch {}
}
</script>

<template>
  <div class="adm">
    <div class="bar">
      <NuxtLink to="/admin/ai-calls" class="btn sm"><Icon name="left" :size="14" /> AI API Call Log</NuxtLink>
      <h2 class="ttl">AI API Call #{{ route.params.id }}</h2>
    </div>
    <p v-if="error" class="err">{{ error }}</p>
    <template v-else-if="c">
      <div class="card sec">
        <div class="sec-h"><h2>Informasi panggilan</h2></div>
        <dl class="grid">
          <div v-for="it in info" :key="it.k">
            <dt>{{ it.k }}</dt>
            <dd :class="{ mono: it.mono }">{{ it.v }}</dd>
          </div>
        </dl>
        <p v-if="c.requestId" class="muted small" style="margin: 12px 0 0">
          Request id yang sama dipakai di <NuxtLink to="/admin/logs" class="lnk">System log</NuxtLink> dan
          <NuxtLink to="/admin/server-log" class="lnk">Log server</NuxtLink> —
          <NuxtLink :to="{ path: '/admin/ai-calls', query: { requestId: c.requestId } }" class="lnk">lihat semua panggilan AI di request ini</NuxtLink>.
        </p>
      </div>

      <div v-if="c.errorMessage" class="card sec">
        <div class="sec-h"><h2>Error</h2></div>
        <pre class="json bad">{{ c.errorMessage }}</pre>
      </div>

      <div class="card sec">
        <div class="sec-h">
          <h2>Request</h2>
          <div class="grow" />
          <button v-if="c.requestBody" class="btn sm" @click="copy(c.requestBody)"><Icon name="copy" :size="14" /> Salin</button>
        </div>
        <pre class="json">{{ c.requestBody ? pretty(c.requestBody) : 'Tidak disimpan (AI_CALL_LOG_BODIES=false)' }}</pre>
      </div>

      <div class="card sec">
        <div class="sec-h">
          <h2>Response</h2>
          <div class="grow" />
          <button v-if="c.responseBody" class="btn sm" @click="copy(c.responseBody)"><Icon name="copy" :size="14" /> Salin</button>
        </div>
        <pre class="json">{{ c.responseBody ? pretty(c.responseBody) : '—' }}</pre>
      </div>
    </template>
    <div v-else class="muted row"><span class="spin" /> Memuat…</div>
  </div>
</template>

<style scoped>
.ttl { margin: 0; font-size: 16px; }
.grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: 12px 20px; margin: 0; }
.grid dt { font-size: 12px; color: var(--muted); font-weight: 600; margin-bottom: 2px; }
.grid dd { margin: 0; font-size: 13px; word-break: break-word; }
.mono { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: 12px; }
.json {
  background: var(--panel-2); border-radius: 8px; padding: 12px; font-size: 12px; overflow: auto; max-height: 600px;
  white-space: pre-wrap; word-break: break-word; margin: 0;
}
.json.bad, .err { color: var(--danger); }
.lnk { color: var(--accent); font-weight: 600; }
</style>
