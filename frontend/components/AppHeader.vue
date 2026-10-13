<script setup lang="ts">
// Header situs. Menu dibedakan per role (lihat composables/useMenu.ts):
// pengunjung belum masuk, tamu, user biasa, dan admin (punya dropdown menu admin sendiri).
import { rupiah } from '~/composables/useApi'
import type { MenuItem } from '~/composables/useMenu'

const { user, token, logout } = useSession()
const api = useApi()
const route = useRoute()
const { role, isAdmin, mainNav, accountNav, adminNav } = useMenu()
const menu = ref(false)
const adminMenu = ref(false)
const root = ref<HTMLElement | null>(null)

onMounted(() => {
  if (token.value && !user.value) api.ensureSession().catch(() => {})
  document.addEventListener('click', onDocClick)
})
onBeforeUnmount(() => document.removeEventListener('click', onDocClick))

// Tutup dropdown saat klik di luar header atau saat pindah halaman.
function onDocClick(e: MouseEvent) {
  if (root.value && !root.value.contains(e.target as Node)) closeMenus()
}
watch(() => route.fullPath, closeMenus)

function closeMenus() {
  menu.value = false
  adminMenu.value = false
}

function isActive(item: MenuItem) {
  return item.exact ? route.path === item.to : route.path.startsWith(item.to)
}

function doLogout() {
  logout()
  closeMenus()
  navigateTo('/')
}
</script>

<template>
  <header ref="root" class="hdr" :data-role="role">
    <NuxtLink to="/" class="logo"><span class="mark">R</span> Racik</NuxtLink>
    <nav class="row">
      <NuxtLink v-for="item in mainNav" :key="item.to" :to="item.to" class="btn ghost sm" :class="{ on: isActive(item) }">{{ item.text }}</NuxtLink>
      <!-- Menu khusus admin: pintasan ke semua halaman panel admin -->
      <div v-if="isAdmin" class="acct">
        <button class="btn ghost sm admin-btn" :class="{ on: adminMenu }" @click="adminMenu = !adminMenu; menu = false">
          <Icon name="shield" :size="15" /> Admin <Icon name="down" :size="14" />
        </button>
        <div v-if="adminMenu" class="popover menu admin-menu" style="left: 0; top: 40px">
          <template v-for="group in adminNav" :key="group.label">
            <div class="menu-label">{{ group.label }}</div>
            <NuxtLink v-for="item in group.items" :key="item.to" :to="item.to" class="menu-item" @click="adminMenu = false">
              <Icon :name="item.icon" :size="16" /> {{ item.text }}
            </NuxtLink>
          </template>
        </div>
      </div>
    </nav>
    <div class="grow" />
    <!-- Saldo: dipotong setiap memakai fitur AI -->
    <NuxtLink v-if="user" to="/wallet" class="bal" title="Saldo & riwayat pemakaian AI"><Icon name="wallet" :size="15" /> {{ rupiah(user.balance ?? 0) }}</NuxtLink>
    <template v-if="role === 'user' || role === 'admin'">
      <span v-if="isAdmin" class="badge admin-badge">Admin</span>
      <div class="acct">
        <button class="btn ghost sm" @click="menu = !menu; adminMenu = false"><Icon name="user" :size="16" /> {{ user!.name || user!.email }}</button>
        <div v-if="menu" class="popover menu" style="right: 0; top: 40px">
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
      <span v-if="role === 'guest'" class="badge" title="Desain tersimpan di perangkat ini. Simpan akun supaya tidak hilang.">Mode tamu</span>
      <NuxtLink to="/login" class="btn sm">{{ role === 'guest' ? 'Simpan akun' : 'Masuk' }}</NuxtLink>
      <NuxtLink v-if="role === 'anon'" to="/login?mode=register" class="btn primary sm">Daftar</NuxtLink>
    </template>
  </header>
</template>

<style scoped>
.hdr { height: 60px; display: flex; align-items: center; gap: 12px; padding: 0 24px; background: #fff; border-bottom: 1px solid var(--line); position: sticky; top: 0; z-index: 50; }
.logo { font-weight: 800; font-size: 20px; display: flex; align-items: center; gap: 8px; letter-spacing: -.02em; }
.mark { width: 30px; height: 30px; border-radius: 9px; background: var(--grad); color: #fff; display: grid; place-items: center; font-size: 16px; }
.acct { position: relative; }
.bal { display: inline-flex; align-items: center; gap: 6px; height: 30px; padding: 0 12px; border-radius: 999px; background: var(--accent-soft); color: var(--accent); font-weight: 700; font-size: 13px; }
.bal:hover { filter: brightness(.97); }
nav .btn.on { background: var(--panel-2); }
.admin-btn { color: var(--accent); }
.admin-menu { min-width: 230px; }
.admin-badge { background: #1f2333; color: #fff; }
.menu-label { padding: 8px 10px 4px; font-size: 11px; font-weight: 700; text-transform: uppercase; letter-spacing: .04em; color: var(--muted); }
.menu-head { display: flex; flex-direction: column; gap: 2px; padding: 8px 10px 10px; margin-bottom: 4px; border-bottom: 1px solid var(--line); }
.menu-head small { font-size: 12px; }
@media (max-width: 640px) { nav { display: none; } .hdr { padding: 0 14px; } .admin-badge { display: none; } }
</style>
