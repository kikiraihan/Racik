// Menu navigasi per role. Satu sumber untuk sidebar situs (AppSidebar) dan sidebar panel admin,
// supaya admin, user biasa, tamu, dan pengunjung yang belum masuk punya menu masing-masing.
import type { User } from '~/composables/useApi'

export type MenuRole = 'anon' | 'guest' | 'user' | 'admin'

export interface MenuItem {
  to: string
  text: string
  icon: string
  exact?: boolean
}

export interface MenuGroup {
  label: string
  items: MenuItem[]
}

/** Menu khusus admin: dipakai sidebar layout admin & grup "Admin" di sidebar situs. */
export const ADMIN_NAV: MenuGroup[] = [
  {
    label: 'Menu',
    items: [
      { to: '/admin', text: 'Ringkasan', icon: 'home', exact: true },
      { to: '/admin/users', text: 'Pengguna & saldo', icon: 'users' },
    ],
  },
  {
    label: 'Konfigurasi',
    items: [
      { to: '/admin/models', text: 'Katalog model', icon: 'layers' },
      { to: '/admin/ai', text: 'Konfigurasi AI', icon: 'sparkles' },
      { to: '/admin/pricing', text: 'Konfigurasi harga', icon: 'wallet' },
    ],
  },
  {
    label: 'Monitoring',
    items: [
      { to: '/admin/logs', text: 'System log', icon: 'activity' },
      { to: '/admin/server-log', text: 'Log server', icon: 'terminal' },
    ],
  },
]

const HOME: MenuItem = { to: '/', text: 'Beranda', icon: 'home', exact: true }
const DESIGNS: MenuItem = { to: '/designs', text: 'Desain saya', icon: 'grid' }
const TEMPLATES: MenuItem = { to: '/templates', text: 'Template', icon: 'template' }
const BRAND: MenuItem = { to: '/brand', text: 'Brand Kit', icon: 'brand' }
const WALLET: MenuItem = { to: '/wallet', text: 'Saldo', icon: 'wallet' }

/** Menu utama di sidebar situs, per role. */
const MAIN_NAV: Record<MenuRole, MenuItem[]> = {
  anon: [HOME, TEMPLATES],
  guest: [HOME, DESIGNS, TEMPLATES, BRAND],
  user: [HOME, DESIGNS, TEMPLATES, BRAND, WALLET],
  admin: [HOME, DESIGNS, TEMPLATES, BRAND, WALLET],
}

/** Isi dropdown akun di sidebar situs, per role (tamu & pengunjung tidak punya dropdown akun). */
const ACCOUNT_NAV: Record<MenuRole, MenuItem[]> = {
  anon: [],
  guest: [],
  user: [WALLET, BRAND],
  admin: [{ to: '/admin', text: 'Panel admin', icon: 'shield' }, WALLET, BRAND],
}

export function menuRole(u: User | null | undefined): MenuRole {
  if (!u) return 'anon'
  if (u.isGuest) return 'guest'
  return u.role === 'admin' ? 'admin' : 'user'
}

export function useMenu() {
  const { user } = useSession()
  const role = computed(() => menuRole(user.value))
  return {
    role,
    isAdmin: computed(() => role.value === 'admin'),
    mainNav: computed(() => MAIN_NAV[role.value]),
    accountNav: computed(() => ACCOUNT_NAV[role.value]),
    adminNav: computed(() => (role.value === 'admin' ? ADMIN_NAV : [])),
  }
}
