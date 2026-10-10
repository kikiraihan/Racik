<script setup lang="ts">
// Panel admin: pengguna & saldo, konfigurasi model AI, konfigurasi harga, system log.
const api = useApi()
const session = useSession()
const route = useRoute()
const router = useRouter()

const TABS = [
  { key: 'users', label: 'Pengguna & saldo', icon: 'users' },
  { key: 'ai', label: 'Konfigurasi AI', icon: 'sparkles' },
  { key: 'pricing', label: 'Konfigurasi harga', icon: 'wallet' },
  { key: 'logs', label: 'System log', icon: 'file' },
] as const
type Tab = (typeof TABS)[number]['key']

const tab = computed<Tab>({
  get: () => (TABS.some((t) => t.key === route.query.tab) ? (route.query.tab as Tab) : 'users'),
  set: (v) => router.replace({ query: { ...route.query, tab: v } }),
})
const ready = ref(false)

onMounted(async () => {
  try {
    await api.ensureSession()
  } catch {}
  if (session.user.value?.role !== 'admin') {
    navigateTo('/login?next=/admin')
    return
  }
  ready.value = true
})
</script>

<template>
  <div>
    <AppHeader />
    <main v-if="ready" class="wrap">
      <h1>Panel admin</h1>
      <div class="tabs">
        <button v-for="t in TABS" :key="t.key" class="chip" :class="{ on: tab === t.key }" @click="tab = t.key">
          <Icon :name="t.icon" :size="14" /> {{ t.label }}
        </button>
      </div>
      <AdminUsers v-if="tab === 'users'" />
      <AdminAiConfig v-else-if="tab === 'ai'" />
      <AdminPricing v-else-if="tab === 'pricing'" />
      <AdminLogs v-else />
    </main>
    <div v-else class="wrap muted">Memeriksa akses…</div>
  </div>
</template>

<style scoped>
.wrap { max-width: 1200px; margin: 0 auto; padding: 28px 16px 60px; }
h1 { margin: 0 0 14px; }
.tabs { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 16px; }
</style>
