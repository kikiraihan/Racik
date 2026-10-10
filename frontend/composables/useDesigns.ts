// Aksi tingkat tinggi: membuat desain baru dari AI, template, atau kanvas kosong.
import type { DesignSummary, Template } from './useApi'

export interface AIDesignInput {
  prompt: string
  designType: string
  width: number
  height: number
  pages: number
  style?: string
  textModel?: string
  imageModel?: string
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

  return { create, generate, fromTemplate }
}
