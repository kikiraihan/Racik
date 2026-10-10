<script setup lang="ts">
import { rupiah } from '~/composables/useApi'

const { user, token, logout } = useSession()
const api = useApi()
const menu = ref(false)

onMounted(() => {
  if (token.value && !user.value) api.ensureSession().catch(() => {})
})

function doLogout() {
  logout()
  menu.value = false
  navigateTo('/')
}
</script>

<template>
  <header class="hdr">
    <NuxtLink to="/" class="logo"><span class="mark">R</span> Racik</NuxtLink>
    <nav class="row">
      <NuxtLink to="/" class="btn ghost sm">Beranda</NuxtLink>
      <NuxtLink to="/brand" class="btn ghost sm">Brand Kit</NuxtLink>
      <NuxtLink v-if="user?.role === 'admin'" to="/admin" class="btn ghost sm"><Icon name="shield" :size="15" /> Admin</NuxtLink>
    </nav>
    <div class="grow" />
    <!-- Saldo: dipotong setiap memakai fitur AI -->
    <NuxtLink v-if="user" to="/wallet" class="bal" title="Saldo & riwayat pemakaian AI"><Icon name="wallet" :size="15" /> {{ rupiah(user.balance ?? 0) }}</NuxtLink>
    <template v-if="user && !user.isGuest">
      <div class="acct">
        <button class="btn ghost sm" @click="menu = !menu"><Icon name="user" :size="16" /> {{ user.name || user.email }}</button>
        <div v-if="menu" class="popover menu" style="right: 0; top: 40px">
          <NuxtLink to="/wallet" class="menu-item" @click="menu = false"><Icon name="wallet" :size="16" /> Saldo</NuxtLink>
          <NuxtLink v-if="user.role === 'admin'" to="/admin" class="menu-item" @click="menu = false"><Icon name="shield" :size="16" /> Panel admin</NuxtLink>
          <button class="menu-item" @click="doLogout"><Icon name="logout" :size="16" /> Keluar</button>
        </div>
      </div>
    </template>
    <template v-else>
      <span v-if="user?.isGuest" class="badge">Mode tamu</span>
      <NuxtLink to="/login" class="btn sm">{{ user?.isGuest ? 'Simpan akun' : 'Masuk' }}</NuxtLink>
      <NuxtLink v-if="!user" to="/login?mode=register" class="btn primary sm">Daftar</NuxtLink>
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
@media (max-width: 640px) { nav { display: none; } .hdr { padding: 0 14px; } }
</style>
