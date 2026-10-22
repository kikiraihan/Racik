<script setup lang="ts">
// Sidebar situs (pengganti header lama). Menu dibedakan per role (lihat composables/useMenu.ts):
// pengunjung belum masuk, tamu, user biasa, dan admin (punya grup menu admin sendiri).
import { rupiah, type DesignSummary } from '~/composables/useApi'
import type { MenuItem } from '~/composables/useMenu'

const emit = defineEmits<{ navigate: [] }>()
const { user, token, logout } = useSession()
const api = useApi()
const route = useRoute()
const { role, isAdmin, mainNav, accountNav, adminNav } = useMenu()
const createDialog = useCreateDialog()
const menu = ref(false)
const adminOpen = ref(route.path.startsWith('/admin'))
const root = ref<HTMLElement | null>(null)
const recent = ref<DesignSummary[]>([])

const initial = computed(() => (user.value?.name || user.value?.email || '?')[0]!.toUpperCase())

onMounted(() => {
  if (token.value && !user.value) api.ensureSession().catch(() => {})
  loadRecent()
  document.addEventListener('click', onDocClick)
})
onBeforeUnmount(() => document.removeEventListener('click', onDocClick))

// Desain terbaru di sidebar (seperti "Recent designs" di Canva).
async function loadRecent() {
  if (!token.value) return (recent.value = [])
  try {
    recent.value = (await api.get<DesignSummary[]>('/api/designs')).slice(0, 5)
  } catch {}
}
watch(token, loadRecent)

// Tutup dropdown saat klik di luar sidebar atau saat pindah halaman.
function onDocClick(e: MouseEvent) {
  if (root.value && !root.value.contains(e.target as Node)) menu.value = false
}
watch(() => route.fullPath, () => {
  menu.value = false
  emit('navigate')
})

function isActive(item: MenuItem) {
  // Cocokkan per segmen path supaya /admin/ai tidak ikut aktif di /admin/ai-calls.
  return item.exact ? route.path === item.to : route.path === item.to || route.path.startsWith(item.to + '/')
}

function openCreate() {
  createDialog.open()
  emit('navigate')
}

function doLogout() {
  logout()
  menu.value = false
  navigateTo('/')
}
</script>

<template>
  <aside ref="root" class="sb" :data-role="role">
    <div class="sb-top">
      <NuxtLink to="/" class="logo"><span class="mark">R</span> Racik</NuxtLink>
    </div>

    <button class="btn new" @click="openCreate"><Icon name="plus" :size="16" /> Buat desain</button>

    <nav class="nav scroll">
      <NuxtLink v-for="item in mainNav" :key="item.to" :to="item.to" class="item" :class="{ on: isActive(item) }">
        <Icon :name="item.icon" :size="17" :stroke="1.8" /> {{ item.text }}
      </NuxtLink>

      <!-- Menu khusus admin: pintasan ke semua halaman panel admin -->
      <template v-if="isAdmin">
        <button class="group-label" @click="adminOpen = !adminOpen">
          <span>Admin</span><Icon :name="adminOpen ? 'up' : 'down'" :size="14" />
        </button>
        <template v-if="adminOpen">
          <template v-for="group in adminNav" :key="group.label">
            <NuxtLink v-for="item in group.items" :key="item.to" :to="item.to" class="item">
              <Icon :name="item.icon" :size="17" :stroke="1.8" /> {{ item.text }}
            </NuxtLink>
          </template>
        </template>
      </template>

      <template v-if="recent.length">
        <div class="group-label static">Desain terbaru</div>
        <NuxtLink v-for="d in recent" :key="d.id" :to="`/editor/${d.id}`" class="item recent">
          <span class="rthumb"><img v-if="d.thumbnail" :src="d.thumbnail" alt="" /></span>
          <span class="ellipsis">{{ d.title }}</span>
        </NuxtLink>
      </template>
    </nav>

    <div class="sb-foot">
      <!-- Saldo: dipotong setiap memakai fitur AI -->
      <NuxtLink v-if="user" to="/wallet" class="credit" title="Saldo & riwayat pemakaian AI">
        <span class="muted">Saldo</span>
        <b>{{ rupiah(user.balance ?? 0) }}</b>
      </NuxtLink>

      <template v-if="role === 'user' || role === 'admin'">
        <div class="acct">
          <button class="user-row" @click="menu = !menu">
            <span class="avatar">{{ initial }}</span>
            <span class="grow ellipsis">{{ user!.name || user!.email }}</span>
            <span v-if="isAdmin" class="badge">Admin</span>
            <Icon name="more" :size="16" />
          </button>
          <div v-if="menu" class="popover menu acct-menu">
            <div class="menu-head">
              <b>{{ user!.name || 'Akun' }}</b>
              <small class="muted">{{ user!.email }}</small>
            </div>
            <NuxtLink v-for="item in accountNav" :key="item.to" :to="item.to" class="menu-item" @click="menu = false"><Icon :name="item.icon" :size="16" /> {{ item.text }}</NuxtLink>
            <button class="menu-item" @click="doLogout"><Icon name="logout" :size="16" /> Keluar</button>
          </div>
        </div>
      </template>
      <template v-else>
        <p v-if="role === 'guest'" class="guest muted" title="Desain tersimpan di perangkat ini. Simpan akun supaya tidak hilang.">
          <span class="badge">Mode tamu</span> Desain tersimpan di perangkat ini.
        </p>
        <div class="auth">
          <NuxtLink to="/login" class="btn sm grow">{{ role === 'guest' ? 'Simpan akun' : 'Masuk' }}</NuxtLink>
          <NuxtLink v-if="role === 'anon'" to="/login?mode=register" class="btn primary sm grow">Daftar</NuxtLink>
        </div>
      </template>
    </div>
  </aside>
</template>

<style scoped>
.sb { height: 100%; display: flex; flex-direction: column; gap: 12px; padding: 14px 10px 12px; background: #f7f7f8; border-right: 1px solid var(--line); }
.sb-top { padding: 2px 6px; }
.logo { font-weight: 700; font-size: 18px; display: inline-flex; align-items: center; gap: 8px; letter-spacing: -.02em; }
.mark { width: 26px; height: 26px; border-radius: 7px; background: var(--ink); color: #fff; display: grid; place-items: center; font-size: 14px; }
.new { width: 100%; height: 36px; background: var(--panel); }
.nav { flex: 1; display: flex; flex-direction: column; gap: 1px; min-height: 0; margin: 0 -4px; padding: 0 4px; }
.item { display: flex; align-items: center; gap: 10px; height: 34px; padding: 0 10px; border-radius: 7px; color: var(--ink-2); font-weight: 500; flex: 0 0 auto; }
.item:hover { background: var(--panel-3); color: var(--ink); }
.item.on { background: var(--panel-3); color: var(--ink); }
.item svg { flex: 0 0 auto; color: var(--muted); }
.item.on svg { color: var(--ink); }
.group-label { display: flex; align-items: center; justify-content: space-between; margin: 14px 0 4px; padding: 0 10px; height: 22px; border: 0; background: none; font-size: 12px; font-weight: 500; color: var(--muted); cursor: pointer; text-align: left; }
.group-label.static { cursor: default; }
.recent { font-weight: 400; }
.rthumb { width: 22px; height: 22px; border-radius: 5px; overflow: hidden; background: var(--panel-3); border: 1px solid var(--line); flex: 0 0 22px; }
.rthumb img { width: 100%; height: 100%; object-fit: cover; display: block; }
.ellipsis { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; min-width: 0; }
.sb-foot { display: flex; flex-direction: column; gap: 8px; border-top: 1px solid var(--line); padding-top: 12px; }
.credit { display: flex; align-items: center; justify-content: space-between; padding: 8px 10px; border-radius: 8px; background: var(--panel); border: 1px solid var(--line); font-size: 13px; }
.credit:hover { border-color: var(--line-2); }
.acct { position: relative; }
.user-row { display: flex; align-items: center; gap: 8px; width: 100%; padding: 6px 8px; border: 0; border-radius: 8px; background: none; cursor: pointer; font-weight: 500; text-align: left; }
.user-row:hover { background: var(--panel-3); }
.avatar { width: 26px; height: 26px; border-radius: 50%; background: var(--ink); color: #fff; display: grid; place-items: center; font-size: 12px; font-weight: 600; flex: 0 0 26px; }
.acct-menu { left: 0; right: 0; bottom: 44px; }
.menu-head { display: flex; flex-direction: column; gap: 2px; padding: 8px 10px 10px; margin-bottom: 4px; border-bottom: 1px solid var(--line); }
.menu-head small { font-size: 12px; word-break: break-all; }
.guest { margin: 0; font-size: 12px; line-height: 1.5; padding: 0 4px; }
.auth { display: flex; gap: 6px; }
</style>
