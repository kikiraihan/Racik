// Ukuran desain siap pakai (mirip Canva).
export interface SizePreset {
  key: string
  label: string
  width: number
  height: number
  group: string
}

export const SIZE_PRESETS: SizePreset[] = [
  { key: 'presentation', label: 'Presentasi 16:9', width: 1920, height: 1080, group: 'Presentasi' },
  { key: 'presentation-43', label: 'Presentasi 4:3', width: 1440, height: 1080, group: 'Presentasi' },
  { key: 'instagram-post', label: 'Instagram Post', width: 1080, height: 1080, group: 'Media sosial' },
  { key: 'instagram-portrait', label: 'Instagram Portrait 4:5', width: 1080, height: 1350, group: 'Media sosial' },
  { key: 'story', label: 'Story / Reels / TikTok', width: 1080, height: 1920, group: 'Media sosial' },
  { key: 'facebook-post', label: 'Facebook Post', width: 1200, height: 630, group: 'Media sosial' },
  { key: 'linkedin-banner', label: 'LinkedIn Banner', width: 1584, height: 396, group: 'Media sosial' },
  { key: 'youtube-thumbnail', label: 'Thumbnail YouTube', width: 1280, height: 720, group: 'Video' },
  { key: 'poster', label: 'Poster A4', width: 1240, height: 1754, group: 'Cetak' },
  { key: 'flyer', label: 'Flyer A5', width: 874, height: 1240, group: 'Cetak' },
  { key: 'business-card', label: 'Kartu Nama', width: 1050, height: 600, group: 'Cetak' },
  { key: 'certificate', label: 'Sertifikat', width: 2000, height: 1414, group: 'Cetak' },
  { key: 'logo', label: 'Logo', width: 500, height: 500, group: 'Lainnya' },
]

export const FONTS = [
  'Inter', 'Poppins', 'Montserrat', 'Playfair Display', 'Roboto', 'Lato', 'Oswald', 'Raleway',
  'Merriweather', 'Bebas Neue', 'Pacifico', 'Dancing Script', 'Space Grotesk', 'DM Serif Display',
  'Nunito', 'Open Sans', 'Archivo Black', 'Lora', 'Caveat', 'Anton',
]

export const DEFAULT_COLORS = [
  '#000000', '#545454', '#737373', '#A6A6A6', '#D9D9D9', '#FFFFFF',
  '#FF3131', '#FF5757', '#FF66C4', '#CB6CE6', '#8C52FF', '#5E17EB',
  '#03989E', '#00C2CB', '#5CE1E6', '#38B6FF', '#5271FF', '#004AAD',
  '#00BF63', '#7ED957', '#C1FF72', '#FFDE59', '#FFBD59', '#FF914D',
]

export const STYLE_PRESETS = [
  'Modern minimalis', 'Bold & ceria', 'Elegan mewah', 'Korporat profesional',
  'Playful kartun', 'Retro 80-an', 'Gelap futuristik', 'Pastel lembut',
]

export function findPreset(key: string) {
  return SIZE_PRESETS.find((p) => p.key === key)
}
