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

// ---- Sign in / sign up dengan Google (Google Identity Services) ----
// Client ID dibaca dari backend (GOOGLE_CLIENT_ID); kosong = tombol Google tidak ditampilkan.
const googleClientId = ref('')
const googleBtn = ref<HTMLElement | null>(null)
let gisReady = false

function loadGis(): Promise<void> {
  const w = window as any
  if (w.google?.accounts?.id) return Promise.resolve()
  return new Promise((resolve, reject) => {
    const s = document.createElement('script')
    s.src = 'https://accounts.google.com/gsi/client'
    s.async = true
    s.onload = () => resolve()
    s.onerror = () => reject(new Error('Gagal memuat Google Sign-In'))
    document.head.appendChild(s)
  })
}

function renderGoogleButton() {
  const g = (window as any).google?.accounts?.id
  if (!gisReady || !g || !googleBtn.value) return
  googleBtn.value.innerHTML = ''
  g.renderButton(googleBtn.value, {
    type: 'standard',
    theme: 'outline',
    size: 'large',
    shape: 'pill',
    text: mode.value === 'login' ? 'signin_with' : 'signup_with',
    logo_alignment: 'center',
    width: Math.min(googleBtn.value.clientWidth || 344, 400),
    locale: 'id',
  })
}

async function initGoogle() {
  try {
    const cfg = await api.get<{ googleClientId: string }>('/api/auth/config')
    if (!cfg.googleClientId) return
    googleClientId.value = cfg.googleClientId
    await loadGis()
    ;(window as any).google.accounts.id.initialize({
      client_id: cfg.googleClientId,
      callback: onGoogleCredential,
      ux_mode: 'popup',
      context: mode.value === 'login' ? 'signin' : 'signup',
    })
    gisReady = true
    await nextTick()
    renderGoogleButton()
  } catch (e: any) {
    console.warn('Google Sign-In tidak tersedia:', e?.message)
  }
}

async function onGoogleCredential(resp: { credential: string }) {
  loading.value = true
  try {
    // Token akun tamu (kalau ada) ikut terkirim, sehingga akun tamu di-upgrade dan desainnya tidak hilang.
    const res = await api.post<{ token: string; user: User }>('/api/auth/google', { credential: resp.credential })
    session.setSession(res.token, res.user)
    toast.success(`Selamat datang, ${res.user.name || res.user.email}!`)
    navigateTo((route.query.next as string) || '/')
  } catch (e: any) {
    toast.error(e.message)
  } finally {
    loading.value = false
  }
}

watch(mode, () => nextTick(renderGoogleButton))

onMounted(() => {
  if (session.token.value) api.ensureSession().then(() => isGuest.value && (mode.value = 'register')).catch(() => {})
  initGoogle()
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
        <template v-if="googleClientId">
          <div ref="googleBtn" class="google-btn" />
          <div class="divider"><span>atau dengan email</span></div>
        </template>
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
.google-btn { display: flex; justify-content: center; min-height: 44px; }
.divider { display: flex; align-items: center; gap: 10px; color: var(--muted); font-size: 13px; }
.divider::before, .divider::after { content: ''; flex: 1; height: 1px; background: var(--line); }
</style>
