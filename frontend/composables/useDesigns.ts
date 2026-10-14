// Aksi tingkat tinggi: membuat desain baru dari AI, template, atau kanvas kosong.
import type { DesignSummary, Template } from './useApi'

export interface AIDesignInput {
  prompt: string
  designType: string
  width: number
  height: number
  pages: number
  style?: string
  generateImages: boolean
}

export function useDesigns() {
  const api = useApi()
  const router = useRouter()

  async function create(title: string, width: number, height: number, data: any = { pages: [] }) {
    await api.ensureSession()
    const d = await api.post<DesignSummary>('/api/designs', { title, width, height, data })
    await router.push(`/editor/${d.id}`)
    return d
  }

  /** Auto-bikin desain dengan AI lalu langsung buka di editor. */
  async function generate(input: AIDesignInput) {
    await api.ensureSession()
    const res = await api.post<{ generationId: string; spec: any; warnings: string[] }>('/api/ai/design', input)
      .finally(() => api.refreshUser()) // saldo terpotong (atau dikembalikan kalau gagal)
    const title = res.spec?.title || input.prompt.slice(0, 60)
    const d = await api.post<DesignSummary>('/api/designs', {
      title, width: input.width, height: input.height,
      data: { pages: [], spec: res.spec, generationId: res.generationId },
    })
    if (res.warnings?.length) sessionStorage.setItem('racik_warn_' + d.id, JSON.stringify(res.warnings))
    await router.push(`/editor/${d.id}`)
    return d
  }

  function fromTemplate(t: Template) {
    return create(t.title, t.width, t.height, { pages: [], spec: t.spec })
  }

  // Aksi kartu desain (dipakai Beranda & halaman Desain saya).
  async function remove(d: DesignSummary) {
    if (!confirm(`Hapus "${d.title}"?`)) return false
    await api.del(`/api/designs/${d.id}`)
    return true
  }
  function duplicate(d: DesignSummary) {
    return api.post<DesignSummary>(`/api/designs/${d.id}/duplicate`)
  }
  async function toggleStar(d: DesignSummary) {
    d.starred = !d.starred
    await api.put(`/api/designs/${d.id}`, { starred: d.starred })
  }

  return { create, generate, fromTemplate, remove, duplicate, toggleStar }
}

/** Dialog "Buat desain" (ala Canva) — dibuka dari sidebar, tombol + mobile, dan Beranda. */
export function useCreateDialog() {
  const state = useState('createDialog', () => ({ open: false, tab: 'for-you' }))
  return {
    state,
    open: (tab = 'for-you') => Object.assign(state.value, { open: true, tab }),
    close: () => (state.value.open = false),
  }
}

export function timeAgo(s: string) {
  const m = Math.round((Date.now() - new Date(s).getTime()) / 60000)
  if (m < 1) return 'baru saja'
  if (m < 60) return `${m} menit lalu`
  if (m < 1440) return `${Math.round(m / 60)} jam lalu`
  return new Date(s).toLocaleDateString('id-ID')
}
