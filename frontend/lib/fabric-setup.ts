// Konfigurasi global fabric.js: properti kustom yang ikut tersimpan & gaya kontrol mirip Canva.
import { FabricObject, Textbox, ActiveSelection } from 'fabric'

let done = false

export const ACCENT = '#8B3DFF'

export function setupFabric() {
  if (done) return
  done = true
  // Properti tambahan yang ikut diserialisasi ke JSON desain.
  FabricObject.customProperties = ['id', 'name', 'locked', 'imagePrompt', 'fx', 'frame', 'role']

  const controls = {
    transparentCorners: false,
    cornerColor: '#FFFFFF',
    cornerStrokeColor: ACCENT,
    borderColor: ACCENT,
    cornerStyle: 'circle' as const,
    cornerSize: 11,
    borderScaleFactor: 1.6,
    padding: 2,
  }
  Object.assign(FabricObject.ownDefaults, controls)
  Object.assign(ActiveSelection.ownDefaults, controls)
  Object.assign(Textbox.ownDefaults, { editingBorderColor: ACCENT, cursorColor: ACCENT, selectionColor: 'rgba(139,61,255,0.25)' })
}

export function uid(prefix = 'o') {
  return prefix + '_' + Math.random().toString(36).slice(2, 10)
}
