// Label bersama halaman AI API Call Log (daftar & detail), pola sama dengan tiara.

// Fitur yang memanggil AI = action activity log (track(...) di backend handlers/server.go).
export const FEATURES: Record<string, string> = {
  'ai.design': 'Magic Design',
  'ai.refine': 'Perbaiki desain',
  'ai.image': 'Gambar AI',
  'ai.image_edit': 'Edit gambar AI',
  'ai.copy': 'Copywriting',
}

export const OPERATIONS: Record<string, { text: string; cls: string }> = {
  chat: { text: 'Teks', cls: 'acc' },
  image: { text: 'Gambar', cls: 'ok' },
  edit: { text: 'Edit gambar', cls: 'warn' },
}

export interface AiCall {
  id: number
  provider: string
  endpoint: string
  model: string
  operation: string
  feature: string | null
  userId: string | null
  email: string | null
  requestId: string | null
  httpStatus: number | null
  isSuccess: boolean
  latencyMs: number | null
  promptTokens: number | null
  completionTokens: number | null
  totalTokens: number | null
  cost: number | null // USD dari vendor
  vendorPrice: number | null // Rupiah per hit (Konfigurasi harga)
  errorMessage: string | null
  requestBody?: any
  responseBody?: any
  createdAt: string
}

export const featureLabel = (f: string | null) => (f ? FEATURES[f] || f : '—')

export const fmtNum = (v: number | null | undefined) => (v === null || v === undefined ? '—' : Number(v).toLocaleString('id-ID'))

export const fmtCost = (v: number | null | undefined) => {
  if (v === null || v === undefined) return '—'
  if (v === 0) return '$0'
  if (v < 0.000001) return '<$0.000001'
  return '$' + (v < 0.01 ? v.toFixed(6) : v.toFixed(4))
}

// Harga vendor bisa pecahan rupiah (mis. 0,5), jadi tidak memakai rupiah() yang membulatkan.
export const fmtRp = (v: number | null | undefined) =>
  v === null || v === undefined ? '—' : 'Rp ' + Number(v).toLocaleString('id-ID', { maximumFractionDigits: 2 })

export const fmtDate = (s: string) => new Date(s).toLocaleString('id-ID', { dateStyle: 'short', timeStyle: 'medium' })
