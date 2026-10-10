// Properti kustom Racik pada objek fabric (lihat FabricObject.customProperties di fabric-setup.ts).
import 'fabric'

declare module 'fabric' {
  interface FabricObject {
    id?: string
    name?: string
    locked?: boolean
    imagePrompt?: string
    fx?: any
    frame?: { type: 'rect' | 'circle'; radius: number }
    role?: string
  }
  interface SerializedObjectProps {
    id?: string
    name?: string
    locked?: boolean
    imagePrompt?: string
    fx?: any
    frame?: { type: 'rect' | 'circle'; radius: number }
    role?: string
  }
}
