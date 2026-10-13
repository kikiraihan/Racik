<script setup lang="ts">
// Layout panel admin: sidebar (bisa diciutkan / jadi drawer di mobile), topbar dengan judul
// halaman & pengganti tema (terang/gelap/sistem). Tampilannya sengaja berbeda dari halaman
// public: sidebar gelap, tanpa AppHeader, dan punya mode gelap sendiri.
import { rupiah } from '~/composables/useApi'
import { ADMIN_NAV } from '~/composables/useMenu'

const api = useApi()
const session = useSession()
const route = useRoute()
const { theme, resolved, setTheme, watchSystem } = useAdminTheme()

const ready = ref(false)
const COLLAPSE_KEY = 'racik_admin_sidebar_collapsed'
const collapsed = ref(false)
const mobileOpen = ref(false)
const themeMenu = ref(false)

// Menu sidebar: menu khusus admin, didefinisikan di composables/useMenu.ts (dipakai juga oleh AppHeader).
const NAV = ADMIN_NAV

const THEMES = [
  { key: 'light', text: 'Terang', icon: 'sun' },
  { key: 'dark', text: 'Gelap', icon: 'moon' },
  { key: 'system', text: 'Ikuti sistem', icon: 'monitor' },
] as const

const pageTitle = computed(() => (route.meta.title as string) || 'Panel admin')
const user = computed(() => session.user.value)
const initial = computed(() => (user.value?.name || user.value?.email || '?')[0]!.toUpperCase())

function isActive(item: { to: string; exact?: boolean }) {
  return item.exact ? route.path === item.to || route.path === item.to + '/' : route.path.startsWith(item.to)
}

function toggleSidebar() {
  if (window.matchMedia('(max-width: 959px)').matches) {
    mobileOpen.value = !mobileOpen.value
  } else {
    collapsed.value = !collapsed.value
    try {
      localStorage.setItem(COLLAPSE_KEY, collapsed.value ? '1' : '0')
    } catch {}
  }
}

// Ctrl+\ menutup/membuka sidebar (sama dengan pintasan di editor).
function onKey(e: KeyboardEvent) {
  if ((e.ctrlKey || e.metaKey) && e.key === '\\') {
    e.preventDefault()
    toggleSidebar()
  }
  if (e.key === 'Escape') {
    mobileOpen.value = false
    themeMenu.value = false
  }
}

watch(
  () => route.fullPath,
  () => (mobileOpen.value = false),
)

// Latar <html> ikut tema admin supaya area overscroll tidak berkedip putih.
watch(resolved, (v) => document.documentElement.classList.toggle('adm-dark', v === 'dark'))

let stopSystem: (() => void) | undefined
onMounted(async () => {
  try {
    collapsed.value = localStorage.getItem(COLLAPSE_KEY) === '1'
  } catch {}
  stopSystem = watchSystem()
  document.documentElement.classList.toggle('adm-dark', resolved.value === 'dark')
  window.addEventListener('keydown', onKey)
  try {
    await api.ensureSession()
  } catch {}
  if (session.user.value?.role !== 'admin') {
    navigateTo('/login?next=' + encodeURIComponent(route.fullPath))
    return
  }
  ready.value = true
})

onBeforeUnmount(() => {
  stopSystem?.()
  window.removeEventListener('keydown', onKey)
  document.documentElement.classList.remove('adm-dark')
})

function logout() {
  session.logout()
  navigateTo('/')
}

useHead({ title: () => `${pageTitle.value} · Admin Racik` })
</script>

<template>
  <div class="adm-shell" :data-theme="resolved" :class="{ collapsed, 'mobile-open': mobileOpen }">
    <div class="drawer-backdrop" @click="mobileOpen = false" />

    <aside class="sidebar">
      <NuxtLink to="/admin" class="brand">
        <span class="mark">R</span>
        <span class="brand-text">
          <strong>Racik</strong>
          <small>Panel admin</small>
        </span>
      </NuxtLink>

      <nav class="side-nav">
        <template v-for="group in NAV" :key="group.label">
          <div class="nav-label">{{ group.label }}</div>
          <NuxtLink
            v-for="item in group.items"
            :key="item.to"
            :to="item.to"
            class="nav-item"
            :class="{ 'is-active': isActive(item) }"
            :title="collapsed ? item.text : undefined"
          >
            <Icon :name="item.icon" :size="18" :stroke="1.8" class="nav-icon" />
            <span class="nav-text">{{ item.text }}</span>
          </NuxtLink>
        </template>
      </nav>

      <div class="side-footer">
        <NuxtLink to="/" class="nav-item" :title="collapsed ? 'Kembali ke Racik' : undefined">
          <Icon name="external" :size="18" :stroke="1.8" class="nav-icon" />
          <span class="nav-text">Kembali ke Racik</span>
        </NuxtLink>
        <div v-if="user" class="user-row">
          <span class="avatar">{{ initial }}</span>
          <span class="nav-text user-meta">
            <b>{{ user.name || 'Admin' }}</b>
            <small>{{ user.email }}</small>
          </span>
          <button class="ghost-btn nav-text" type="button" title="Keluar" @click="logout"><Icon name="logout" :size="16" /></button>
        </div>
      </div>
    </aside>

    <div class="main">
      <header class="topbar">
        <button class="icon-btn" type="button" title="Buka/tutup menu (Ctrl+\)" @click="toggleSidebar"><Icon name="menu" :size="18" /></button>
        <div class="crumbs">
          <span class="muted small crumb-root">Admin</span>
          <span class="muted small crumb-root">/</span>
          <h1>{{ pageTitle }}</h1>
        </div>
        <div class="grow" />
        <NuxtLink v-if="user" to="/wallet" class="bal" title="Saldo akun admin"><Icon name="wallet" :size="14" /> {{ rupiah(user.balance ?? 0) }}</NuxtLink>
        <div class="theme-wrap">
          <button class="icon-btn" type="button" :title="resolved === 'dark' ? 'Tema: gelap' : 'Tema: terang'" @click="themeMenu = !themeMenu">
            <Icon :name="resolved === 'dark' ? 'moon' : 'sun'" :size="18" />
          </button>
          <div v-if="themeMenu" class="popover menu theme-menu" @click="themeMenu = false">
            <button v-for="t in THEMES" :key="t.key" class="menu-item" :class="{ on: theme === t.key }" @click="setTheme(t.key)">
              <Icon :name="t.icon" :size="16" /> {{ t.text }}
              <Icon v-if="theme === t.key" name="check" :size="14" class="kbd" />
            </button>
          </div>
        </div>
        <NuxtLink to="/" class="btn sm view-btn"><Icon name="external" :size="14" /> Lihat situs</NuxtLink>
      </header>

      <main class="content">
        <slot v-if="ready" />
        <div v-else class="muted checking"><span class="spin" /> Memeriksa akses…</div>
      </main>
    </div>
  </div>
</template>

<style scoped>
.adm-shell { --sidebar-width: 248px; display: flex; min-height: 100vh; background: var(--bg); color: var(--ink); }
.adm-shell.collapsed { --sidebar-width: 68px; }

/* --- Sidebar --------------------------------------------------------------- */
.sidebar {
  width: var(--sidebar-width); flex: none; position: sticky; top: 0; height: 100vh; z-index: 30;
  display: flex; flex-direction: column; overflow-x: hidden; overflow-y: auto;
  background: var(--sb-bg); border-right: 1px solid var(--sb-line); color: var(--sb-text);
  transition: width .18s ease;
}
.brand { display: flex; align-items: center; gap: 10px; padding: 0 16px; min-height: 64px; border-bottom: 1px solid var(--sb-line); }
.mark { width: 32px; height: 32px; flex: none; border-radius: 9px; background: var(--grad); color: #fff; display: grid; place-items: center; font-weight: 800; font-size: 16px; }
.brand-text { display: flex; flex-direction: column; line-height: 1.2; white-space: nowrap; }
.brand-text strong { color: #fff; font-size: 15px; letter-spacing: -.01em; }
.brand-text small { color: var(--sb-muted); font-size: 11px; }

.side-nav { flex: 1; padding: 8px 10px 16px; }
.nav-label { font-size: 10px; font-weight: 700; letter-spacing: .12em; text-transform: uppercase; color: var(--sb-muted); padding: 16px 10px 6px; white-space: nowrap; }
.nav-item {
  display: flex; align-items: center; gap: 10px; padding: 9px 10px; margin-bottom: 2px; border-radius: 8px;
  color: var(--sb-text); font-size: 13px; font-weight: 600; white-space: nowrap; position: relative;
}
.nav-item:hover { background: rgba(255, 255, 255, .06); color: #fff; }
.nav-item.is-active { background: var(--sb-active); color: #fff; }
.nav-item.is-active::before { content: ''; position: absolute; left: -10px; top: 8px; bottom: 8px; width: 3px; border-radius: 0 3px 3px 0; background: var(--accent-2); }
.nav-icon { flex: none; }

.side-footer { border-top: 1px solid var(--sb-line); padding: 10px; }
.user-row { display: flex; align-items: center; gap: 8px; padding: 8px 4px 2px; }
.avatar { width: 32px; height: 32px; flex: none; border-radius: 50%; background: var(--grad); color: #fff; display: grid; place-items: center; font-weight: 700; font-size: 13px; }
.user-meta { flex: 1; min-width: 0; display: flex; flex-direction: column; line-height: 1.25; }
.user-meta b { color: #fff; font-size: 12px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.user-meta small { color: var(--sb-muted); font-size: 11px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.ghost-btn { width: 30px; height: 30px; flex: none; display: grid; place-items: center; border: 0; border-radius: 8px; background: transparent; color: var(--sb-muted); cursor: pointer; }
.ghost-btn:hover { color: #fff; background: rgba(255, 255, 255, .08); }

/* Diciutkan: hanya ikon */
.adm-shell.collapsed .brand-text,
.adm-shell.collapsed .nav-text,
.adm-shell.collapsed .nav-label { display: none; }
.adm-shell.collapsed .brand { justify-content: center; padding: 0; }
.adm-shell.collapsed .nav-item { justify-content: center; padding-left: 0; padding-right: 0; }
.adm-shell.collapsed .user-row { justify-content: center; padding-left: 0; padding-right: 0; }

/* --- Area utama ------------------------------------------------------------ */
.main { flex: 1; min-width: 0; display: flex; flex-direction: column; }
.topbar {
  position: sticky; top: 0; z-index: 20; display: flex; align-items: center; gap: 10px; min-height: 64px; padding: 10px 24px;
  background: color-mix(in srgb, var(--panel) 88%, transparent); backdrop-filter: blur(8px); border-bottom: 1px solid var(--line);
}
.crumbs { display: flex; align-items: baseline; gap: 6px; min-width: 0; }
.crumbs h1 { margin: 0; font-size: 17px; font-weight: 700; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.icon-btn {
  width: 36px; height: 36px; flex: none; display: grid; place-items: center; cursor: pointer;
  border: 1px solid var(--line); border-radius: 8px; background: var(--panel); color: var(--ink);
}
.icon-btn:hover { background: var(--panel-2); }
.bal { display: inline-flex; align-items: center; gap: 6px; height: 30px; padding: 0 12px; border-radius: 999px; background: var(--accent-soft); color: var(--accent); font-weight: 700; font-size: 12px; white-space: nowrap; }
.theme-wrap { position: relative; }
.theme-menu { right: 0; top: 42px; min-width: 180px; }
.theme-menu .menu-item.on { color: var(--accent); font-weight: 600; }
.content { padding: 24px; width: 100%; max-width: 1440px; }
.checking { display: flex; align-items: center; gap: 8px; padding: 40px 0; }
.drawer-backdrop { display: none; }

/* --- Mobile: sidebar jadi drawer ------------------------------------------- */
@media (max-width: 959px) {
  .adm-shell, .adm-shell.collapsed { --sidebar-width: 260px; }
  .sidebar { position: fixed; left: 0; top: 0; bottom: 0; transform: translateX(-100%); transition: transform .2s ease; }
  .adm-shell.mobile-open .sidebar { transform: translateX(0); box-shadow: var(--shadow-lg); }
  .adm-shell.collapsed .brand-text,
  .adm-shell.collapsed .nav-text,
  .adm-shell.collapsed .nav-label { display: initial; }
  .adm-shell.collapsed .user-meta { display: flex; }
  .adm-shell.collapsed .ghost-btn { display: grid; }
  .adm-shell.collapsed .nav-item { justify-content: flex-start; padding: 9px 10px; }
  .adm-shell.collapsed .brand { justify-content: flex-start; padding: 0 16px; }
  .adm-shell.mobile-open .drawer-backdrop { display: block; position: fixed; inset: 0; background: rgba(8, 10, 20, .5); z-index: 25; }
  .topbar { padding: 10px 14px; }
  .content { padding: 16px; }
  .view-btn, .crumb-root { display: none; }
}
@media (max-width: 520px) { .bal { display: none; } }
</style>
