import type { InjectionKey } from 'vue'
import type { Editor } from '~/lib/editor'

export interface DocMeta {
  id: string
  title: string
  saving: boolean
  savedAt: number
  dirty: boolean
  generationId: string
  rated: boolean
}

export const EditorKey: InjectionKey<Editor> = Symbol('editor')
export const DocKey: InjectionKey<DocMeta> = Symbol('doc')

export function useEditor() {
  const ed = inject(EditorKey)
  if (!ed) throw new Error('Editor belum tersedia')
  return ed
}

export function useDoc() {
  const d = inject(DocKey)
  if (!d) throw new Error('Doc belum tersedia')
  return d
}
