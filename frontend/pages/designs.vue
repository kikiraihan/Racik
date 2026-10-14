<script setup lang="ts">
// Semua desain milik user: filter (semua / berbintang), pencarian, tampilan grid atau daftar.
import type { DesignSummary } from '~/composables/useApi'

definePageMeta({ layout: 'app' })

const api = useApi()
const { token } = useSession()
const { remove, duplicate, toggleStar } = useDesigns()
const createDialog = useCreateDialog()

const VIEW_KEY = 'racik_designs_view'
const designs = ref<DesignSummary[]>([])
const loaded = ref(false)
const q = ref('')
const filter = ref<'all' | 'starred'>('all')
const view = ref<'grid' | 'list'>('grid')

const shown = computed(() => (filter.value === 'starred' ? designs.value.filter((d) => d.starred) : designs.value))

let qTimer: any
async function load() {
  if (!token.value) return (loaded.value = true)
  try {
    designs.value = await api.get<DesignSummary[]>('/api/designs', { q: q.value })
  } catch {}
  loaded.value = true
}
function onSearch() {
  clearTimeout(qTimer)
  qTimer = setTimeout(load, 250)
}

onMounted(() => {
  try {
    if (localStorage.getItem(VIEW_KEY) === 'list') view.value = 'list'
  } catch {}
  load()
})
watch(view, (v) => {
  try {
    localStorage.setItem(VIEW_KEY, v)
  } catch {}
})

async function removeDesign(d: DesignSummary) {
  if (await remove(d)) designs.value = designs.value.filter((x) => x.id !== d.id)
}
async function duplicateDesign(d: DesignSummary) {
  designs.value.unshift(await duplicate(d))
}
const fmtDate = (s: string) => new Date(s).toLocaleDateString('id-ID', { dateStyle: 'medium' })
</script>

<template>
  <div class="page">
    <div class="page-head">
      <div class="grow">
        <h1>Desain saya</h1>
        <p>Semua desain yang pernah kamu buat.</p>
      </div>
      <button class="btn primary" @click="createDialog.open()"><Icon name="plus" :size="16" /> Buat desain</button>
    </div>

    <div class="toolbar">
      <div class="seg">
        <button :class="{ on: filter === 'all' }" @click="filter = 'all'">Semua</button>
        <button :class="{ on: filter === 'starred' }" @click="filter = 'starred'">Berbintang</button>
      </div>
      <div class="search">
        <Icon name="search" :size="16" />
        <input v-model="q" class="input" placeholder="Cari desain…" @input="onSearch" />
      </div>
      <div class="seg icons">
        <button :class="{ on: view === 'grid' }" title="Tampilan grid" @click="view = 'grid'"><Icon name="grid" :size="16" /></button>
        <button :class="{ on: view === 'list' }" title="Tampilan daftar" @click="view = 'list'"><Icon name="list" :size="16" /></button>
      </div>
    </div>

    <div v-if="!token" class="empty"><b>Belum ada desain</b>Buat desain pertamamu — tanpa perlu daftar dulu.</div>
    <div v-else-if="!loaded" class="muted">Memuat…</div>
    <div v-else-if="!shown.length" class="empty">
      <b>{{ q ? 'Tidak ditemukan' : filter === 'starred' ? 'Belum ada desain berbintang' : 'Belum ada desain' }}</b>
      {{ q ? 'Coba kata kunci lain.' : filter === 'starred' ? 'Klik ikon bintang pada desain supaya mudah ditemukan.' : 'Mulai dari AI di Beranda atau dari kanvas kosong.' }}
    </div>

    <div v-else-if="view === 'grid'" class="dgrid">
      <DesignCard v-for="d in shown" :key="d.id" :d="d" @star="toggleStar(d)" @duplicate="duplicateDesign(d)" @remove="removeDesign(d)" />
    </div>

    <div v-else class="panel tbl-wrap">
      <table class="tbl">
        <thead>
          <tr><th>Nama</th><th class="hide-sm">Ukuran</th><th class="hide-sm">Dibuat</th><th>Diubah</th><th /></tr>
        </thead>
        <tbody>
          <tr v-for="d in shown" :key="d.id">
            <td>
              <NuxtLink :to="`/editor/${d.id}`" class="name">
                <span class="th"><img v-if="d.thumbnail" :src="d.thumbnail" alt="" /></span>
                <span class="ellipsis">{{ d.title }}</span>
                <Icon v-if="d.starred" name="star" :size="13" class="star-on" />
              </NuxtLink>
            </td>
            <td class="muted hide-sm nowrap">{{ d.width }}×{{ d.height }}</td>
            <td class="muted hide-sm nowrap">{{ fmtDate(d.createdAt) }}</td>
            <td class="muted nowrap">{{ timeAgo(d.updatedAt) }}</td>
            <td class="r nowrap">
              <button class="btn ghost sm icon" :title="d.starred ? 'Hapus bintang' : 'Beri bintang'" @click="toggleStar(d)"><Icon name="star" :size="15" /></button>
              <button class="btn ghost sm icon" title="Duplikat" @click="duplicateDesign(d)"><Icon name="copy" :size="15" /></button>
              <button class="btn ghost sm icon danger" title="Hapus" @click="removeDesign(d)"><Icon name="trash" :size="15" /></button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<style scoped>
.tbl-wrap { overflow-x: auto; }
.tbl { width: 100%; border-collapse: collapse; font-size: 14px; }
.tbl th { text-align: left; font-weight: 500; color: var(--muted); font-size: 13px; padding: 12px 16px; border-bottom: 1px solid var(--line); }
.tbl td { padding: 10px 16px; border-bottom: 1px solid var(--line); vertical-align: middle; }
.tbl tr:last-child td { border-bottom: 0; }
.tbl tbody tr:hover { background: #fcfcfc; }
.name { display: flex; align-items: center; gap: 12px; font-weight: 500; min-width: 0; max-width: 420px; }
.th { width: 48px; height: 36px; border-radius: 6px; overflow: hidden; background: var(--panel-2); border: 1px solid var(--line); flex: 0 0 48px; }
.th img { width: 100%; height: 100%; object-fit: cover; display: block; }
.ellipsis { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.star-on { color: #d97706; flex: 0 0 auto; }
.r { text-align: right; }
.nowrap { white-space: nowrap; }
.toolbar .search { max-width: 520px; }
@media (max-width: 720px) {
  .hide-sm { display: none; }
  .toolbar .search { order: 3; flex-basis: 100%; max-width: none; }
}
</style>
