// Tema panel admin: terang / gelap / ikut sistem. Hanya berlaku di layout admin — halaman
// public (beranda, editor) tetap memakai tema terang Racik.
export type AdminTheme = 'light' | 'dark' | 'system'

const STORAGE_KEY = 'racik_admin_theme'

function readTheme(): AdminTheme {
  try {
    const v = localStorage.getItem(STORAGE_KEY)
    if (v === 'light' || v === 'dark' || v === 'system') return v
  } catch {}
  return 'system'
}

export function useAdminTheme() {
  const theme = useState<AdminTheme>('admin-theme', () => readTheme())
  const systemDark = useState<boolean>('admin-system-dark', () => false)

  // Tema yang benar-benar dipakai: 'light' | 'dark'
  const resolved = computed<'light' | 'dark'>(() => (theme.value === 'system' ? (systemDark.value ? 'dark' : 'light') : theme.value))

  function setTheme(v: AdminTheme) {
    theme.value = v
    try {
      localStorage.setItem(STORAGE_KEY, v)
    } catch {}
  }

  function toggle() {
    setTheme(resolved.value === 'dark' ? 'light' : 'dark')
  }

  /** Pasang listener prefers-color-scheme; panggil sekali dari layout admin. */
  function watchSystem() {
    const media = window.matchMedia('(prefers-color-scheme: dark)')
    systemDark.value = media.matches
    const onChange = (e: MediaQueryListEvent) => (systemDark.value = e.matches)
    media.addEventListener('change', onChange)
    return () => media.removeEventListener('change', onChange)
  }

  return { theme, resolved, setTheme, toggle, watchSystem }
}
