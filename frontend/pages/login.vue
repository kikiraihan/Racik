<script setup lang="ts">
import type { User } from '~/composables/useApi'

const route = useRoute()
const api = useApi()
const toast = useToast()
const session = useSession()
const mode = ref<'login' | 'register'>(route.query.mode === 'register' ? 'register' : 'login')
const form = reactive({ email: '', password: '', name: '' })
const loading = ref(false)

const isGuest = computed(() => !!session.user.value?.isGuest)

onMounted(() => {
  if (session.token.value) api.ensureSession().then(() => isGuest.value && (mode.value = 'register')).catch(() => {})
})

async function submit() {
  loading.value = true
  try {
    let path = mode.value === 'login' ? '/api/auth/login' : '/api/auth/register'
    // Akun tamu di-upgrade agar desain yang sudah dibuat tidak hilang.
    if (mode.value === 'register' && isGuest.value) path = '/api/auth/upgrade'
    const res = await api.post<{ token: string; user: User }>(path, form)
    session.setSession(res.token, res.user)
    toast.success(mode.value === 'login' ? 'Selamat datang kembali!' : 'Akun berhasil dibuat')
    navigateTo((route.query.next as string) || '/')
  } catch (e: any) {
    toast.error(e.message)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div>
    <AppHeader />
    <div class="center">
      <form class="card box" @submit.prevent="submit">
        <h2>{{ mode === 'login' ? 'Masuk ke Racik' : isGuest ? 'Simpan akun tamu kamu' : 'Buat akun Racik' }}</h2>
        <p v-if="isGuest && mode === 'register'" class="muted">Semua desain yang kamu buat sebagai tamu akan tetap tersimpan.</p>
        <label v-if="mode === 'register'"><span class="label">Nama</span><input v-model="form.name" class="input" autocomplete="name" /></label>
        <label><span class="label">Email</span><input v-model="form.email" type="email" class="input" required autocomplete="email" /></label>
        <label><span class="label">Password</span><input v-model="form.password" type="password" class="input" required minlength="8" :autocomplete="mode === 'login' ? 'current-password' : 'new-password'" /></label>
        <button class="btn primary" style="height: 42px" :disabled="loading"><span v-if="loading" class="spin" />{{ mode === 'login' ? 'Masuk' : 'Daftar' }}</button>
        <p class="muted" style="text-align: center; margin: 4px 0 0">
          <template v-if="mode === 'login'">Belum punya akun? <a href="#" @click.prevent="mode = 'register'" class="lnk">Daftar</a></template>
          <template v-else>Sudah punya akun? <a href="#" @click.prevent="mode = 'login'" class="lnk">Masuk</a></template>
        </p>
      </form>
    </div>
  </div>
</template>

<style scoped>
.center { display: grid; place-items: center; padding: 60px 16px; }
.box { width: min(400px, 100%); padding: 28px; display: flex; flex-direction: column; gap: 14px; }
h2 { margin: 0; }
.lnk { color: var(--accent); font-weight: 600; }
</style>
