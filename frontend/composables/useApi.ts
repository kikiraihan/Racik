// Wrapper $fetch ke backend Go + manajemen sesi (token JWT di localStorage).
export interface User {
  id: string
  email: string
  name: string
  isGuest: boolean
  role: 'user' | 'admin'
  balance: number // saldo dalam Rupiah, dipotong setiap memakai fitur AI
}

const TOKEN_KEY = 'racik_token'

function readToken(): string {
  try {
    return localStorage.getItem(TOKEN_KEY) || ''
  } catch {
    return ''
  }
}

export function useSession() {
  const token = useState<string>('token', () => readToken())
  const user = useState<User | null>('user', () => null)

  function setSession(t: string, u: User) {
    token.value = t
    user.value = u
    try {
      localStorage.setItem(TOKEN_KEY, t)
    } catch {}
  }

  function logout() {
    token.value = ''
    user.value = null
    try {
      localStorage.removeItem(TOKEN_KEY)
    } catch {}
  }

  return { token, user, setSession, logout }
}

export function useApi() {
  const config = useRuntimeConfig()
  const session = useSession()
  const base = config.public.apiBase as string

  async function request<T>(path: string, opts: any = {}): Promise<T> {
    const headers: Record<string, string> = { ...(opts.headers || {}) }
    if (session.token.value) headers.Authorization = `Bearer ${session.token.value}`
    try {
      return await $fetch<T>(base + path, { ...opts, headers })
    } catch (e: any) {
      const msg = e?.data?.error || e?.message || 'Permintaan gagal'
      if (e?.status === 401 && session.token.value) session.logout()
      // code "insufficient_balance" (402) dipakai UI untuk mengarahkan ke halaman saldo.
      throw Object.assign(new Error(msg), { status: e?.status, code: e?.data?.code })
    }
  }

  /** Pastikan user punya sesi; kalau belum, buat akun tamu otomatis (tanpa daftar). */
  async function ensureSession(): Promise<User> {
    if (session.token.value) {
      if (session.user.value) return session.user.value
      try {
        const u = await request<User>('/api/auth/me')
        session.user.value = u
        return u
      } catch {
        session.logout()
      }
    }
    const res = await request<{ token: string; user: User }>('/api/auth/guest', { method: 'POST' })
    session.setSession(res.token, res.user)
    return res.user
  }

  /** Muat ulang data user (saldo) setelah memakai fitur AI. */
  async function refreshUser() {
    if (!session.token.value) return
    try {
      session.user.value = await request<User>('/api/auth/me')
    } catch {}
  }

  return {
    request,
    ensureSession,
    refreshUser,
    get: <T>(p: string, query?: any) => request<T>(p, { query }),
    post: <T>(p: string, body?: any) => request<T>(p, { method: 'POST', body }),
    put: <T>(p: string, body?: any) => request<T>(p, { method: 'PUT', body }),
    del: <T>(p: string) => request<T>(p, { method: 'DELETE' }),
    upload: async (file: File | Blob, name = 'image.png') => {
      const fd = new FormData()
      fd.append('file', file, (file as File).name || name)
      return request<Asset>('/api/assets', { method: 'POST', body: fd })
    },
  }
}

export interface Asset {
  id: string
  filename: string
  mime: string
  size: number
  url: string
  source: string
  createdAt: string
}

export interface DesignSummary {
  id: string
  title: string
  width: number
  height: number
  thumbnail: string
  starred: boolean
  createdAt: string
  updatedAt: string
}

export interface Template {
  id: string
  slug: string
  title: string
  category: string
  width: number
  height: number
  spec: any
}

// Tarif fitur AI per hit (Rupiah). Model/provider tidak pernah dikirim ke user — diatur admin.
export interface AIPrices {
  text: number // Magic Design / Perbaiki desain / Copywriting (1x susun layout/teks)
  image: number // per gambar AI yang berhasil dibuat
  edit: number // per gambar yang diedit AI
}

export interface AIModels {
  configured: boolean
  prices: AIPrices
  fonts: string[]
  maxImagesPerDesign: number
}

export interface WalletTx {
  id: string
  amount: number
  balanceAfter: number
  kind: 'topup' | 'charge' | 'refund' | 'adjust'
  feature: string
  description: string
  createdAt: string
}

/** Format Rupiah: 12500 → "Rp 12.500" */
export function rupiah(n: number) {
  return (n < 0 ? '-' : '') + 'Rp ' + Math.abs(Math.round(n)).toLocaleString('id-ID')
}
