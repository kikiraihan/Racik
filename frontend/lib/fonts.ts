// Memastikan Google Fonts sudah termuat sebelum fabric mengukur & merender teks.
import { cache } from 'fabric'

const loaded = new Set<string>()

export async function ensureFonts(families: Iterable<string>) {
  if (typeof document === 'undefined' || !document.fonts) return
  const todo = [...new Set(families)].filter((f) => f && !loaded.has(f))
  if (!todo.length) return
  await Promise.all(todo.flatMap((f) => ['400', '700'].map((w) => document.fonts.load(`${w} 32px "${f}"`).catch(() => []))))
  todo.forEach((f) => {
    loaded.add(f)
    cache.clearFontCache(f)
  })
}

/** Kumpulkan fontFamily dari JSON fabric (rekursif, termasuk grup). */
export function fontsInJSON(json: any, out = new Set<string>()) {
  for (const o of json?.objects || []) {
    if (o.fontFamily) out.add(o.fontFamily)
    if (o.objects) fontsInJSON(o, out)
  }
  return out
}

export function fontsInSpec(spec: any, out = new Set<string>()) {
  for (const p of spec?.pages || []) for (const el of p.elements || []) if (el.fontFamily) out.add(el.fontFamily)
  if (spec?.fonts?.heading) out.add(spec.fonts.heading)
  if (spec?.fonts?.body) out.add(spec.fonts.body)
  return out
}
