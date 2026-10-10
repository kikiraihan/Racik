<script setup lang="ts">
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
    </nav>
    <div class="grow" />
    <template v-if="user && !user.isGuest">
      <div class="acct">
        <button class="btn ghost sm" @click="menu = !menu"><Icon name="user" :size="16" /> {{ user.name || user.email }}</button>
        <div v-if="menu" class="popover menu" style="right: 0; top: 40px">
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
@media (max-width: 640px) { nav { display: none; } .hdr { padding: 0 14px; } }
</style>
