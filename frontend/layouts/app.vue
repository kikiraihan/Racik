<script setup lang="ts">
// Layout halaman public & user (Beranda, Desain saya, Template, Brand Kit, Saldo).
// Desktop: sidebar kiri + konten (ala dashboard Canva). Mobile: top bar, tab bar bawah,
// tombol + mengambang, dan sidebar jadi drawer lewat tab "Menu".
import { rupiah } from '~/composables/useApi'

const route = useRoute()
const { user, token } = useSession()
const createDialog = useCreateDialog()
const drawer = ref(false)

// Tab bar mobile: maksimal 3 menu utama + "Menu" (membuka drawer sidebar).
const TABS = computed(() => [
  { to: '/', text: 'Beranda', icon: 'home', exact: true },
  ...(token.value ? [{ to: '/designs', text: 'Desain', icon: 'grid', exact: false }] : []),
  { to: '/templates', text: 'Template', icon: 'template', exact: false },
])

function isActive(t: { to: string; exact: boolean }) {
  return t.exact ? route.path === t.to : route.path.startsWith(t.to)
}

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') drawer.value = false
}
onMounted(() => document.addEventListener('keydown', onKey))
onBeforeUnmount(() => document.removeEventListener('keydown', onKey))
</script>

<template>
  <div class="pub shell" :class="{ 'drawer-open': drawer }">
    <div class="backdrop" @click="drawer = false" />
    <AppSidebar class="side" @navigate="drawer = false" />

    <div class="main">
      <!-- Top bar khusus mobile -->
      <header class="mtop">
        <NuxtLink to="/" class="logo"><span class="mark">R</span> Racik</NuxtLink>
        <div class="grow" />
        <NuxtLink v-if="user" to="/wallet" class="pill"><Icon name="wallet" :size="14" /> {{ rupiah(user.balance ?? 0) }}</NuxtLink>
        <NuxtLink v-else to="/login" class="btn sm">Masuk</NuxtLink>
      </header>

      <slot />
    </div>

    <!-- Tab bar bawah + tombol buat (mobile) -->
    <nav class="tabbar">
      <NuxtLink v-for="t in TABS" :key="t.to" :to="t.to" class="tab" :class="{ on: isActive(t) }">
        <Icon :name="t.icon" :size="20" :stroke="1.8" /><span>{{ t.text }}</span>
      </NuxtLink>
      <button class="tab" :class="{ on: drawer }" @click="drawer = true">
        <Icon name="menu" :size="20" :stroke="1.8" /><span>Menu</span>
      </button>
    </nav>
    <button class="fab" title="Buat desain" @click="createDialog.open()"><Icon name="plus" :size="24" /></button>

    <CreateDialog />
  </div>
</template>

<style scoped>
.shell { display: grid; grid-template-columns: 248px 1fr; height: 100vh; overflow: hidden; }
.side { position: relative; z-index: 2; }
.main { overflow-y: auto; min-width: 0; height: 100vh; }
.mtop, .tabbar, .fab, .backdrop { display: none; }

@media (max-width: 900px) {
  .shell { grid-template-columns: 1fr; height: auto; min-height: 100vh; overflow: visible; }
  .main { height: auto; overflow: visible; }
  .side { position: fixed; inset: 0 auto 0 0; width: 280px; z-index: 90; transform: translateX(-100%); transition: transform .2s ease; }
  .drawer-open .side { transform: none; box-shadow: var(--shadow-lg); }
  .backdrop { display: block; position: fixed; inset: 0; z-index: 80; background: rgba(24, 24, 27, .35); opacity: 0; pointer-events: none; transition: opacity .2s; }
  .drawer-open .backdrop { opacity: 1; pointer-events: auto; }

  .mtop { display: flex; align-items: center; gap: 10px; height: 56px; padding: 0 16px; background: var(--panel); border-bottom: 1px solid var(--line); position: sticky; top: 0; z-index: 40; }
  .logo { font-weight: 700; font-size: 18px; display: inline-flex; align-items: center; gap: 8px; letter-spacing: -.02em; }
  .mark { width: 26px; height: 26px; border-radius: 7px; background: var(--ink); color: #fff; display: grid; place-items: center; font-size: 14px; }
  .pill { display: inline-flex; align-items: center; gap: 6px; height: 30px; padding: 0 12px; border-radius: 999px; border: 1px solid var(--line); font-weight: 600; font-size: 13px; }

  .tabbar { display: flex; position: fixed; left: 0; right: 0; bottom: 0; z-index: 40; height: 64px; background: var(--panel); border-top: 1px solid var(--line); padding-bottom: env(safe-area-inset-bottom); }
  .tab { flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 3px; border: 0; background: none; color: var(--muted); font-size: 11px; font-weight: 500; cursor: pointer; }
  .tab.on { color: var(--ink); }
  .fab { display: grid; place-items: center; position: fixed; right: 18px; bottom: 80px; z-index: 41; width: 54px; height: 54px; border-radius: 50%; border: 0; background: var(--ink); color: #fff; box-shadow: var(--shadow-lg); cursor: pointer; }
}
</style>
