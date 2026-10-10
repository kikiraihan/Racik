<script setup lang="ts">
// Daftar semua user + tambah saldo (tahap testing, sebelum ada payment gateway).
import { rupiah, type WalletTx } from '~/composables/useApi'

interface AdminUser {
  id: string
  email: string
  name: string
  role: 'user' | 'admin'
  isGuest: boolean
  balance: number
  spent: number
  topUp: number
  aiHits: number
  createdAt: string
}

const api = useApi()
const toast = useToast()
const session = useSession()

const q = ref('')
const guests = ref(false)
const page = ref(1)
const data = ref<{ items: AdminUser[]; total: number; size: number } | null>(null)
const loading = ref(false)

// Dialog tambah saldo
const target = ref<AdminUser | null>(null)
const amount = ref<number>(10000)
const note = ref('')
const saving = ref(false)
const QUICK = [5000, 10000, 25000, 50000, 100000]

// Dialog riwayat saldo
const historyOf = ref<AdminUser | null>(null)
const history = ref<WalletTx[]>([])

async function load() {
  loading.value = true
  try {
    data.value = await api.get('/api/admin/users', { q: q.value, guests: guests.value ? 1 : 0, page: page.value })
  } catch (e: any) {
    toast.error(e.message)
  } finally {
    loading.value = false
  }
}
onMounted(load)
watch(guests, () => ((page.value = 1), load()))
let t: any
watch(q, () => {
  clearTimeout(t)
  t = setTimeout(() => ((page.value = 1), load()), 300)
})
const pages = computed(() => Math.max(1, Math.ceil((data.value?.total || 0) / (data.value?.size || 50))))

function openTopup(u: AdminUser) {
  target.value = u
  amount.value = 10000
  note.value = ''
}

async function submitTopup() {
  if (!target.value || !amount.value) return
  saving.value = true
  try {
    const res = await api.post<{ balance: number }>(`/api/admin/users/${target.value.id}/balance`, { amount: Math.round(amount.value), note: note.value })
    toast.success(`Saldo ${target.value.email} sekarang ${rupiah(res.balance)}`)
    if (target.value.id === session.user.value?.id) api.refreshUser()
    target.value = null
    load()
  } catch (e: any) {
    toast.error(e.message)
  } finally {
    saving.value = false
  }
}

async function setRole(u: AdminUser, role: 'user' | 'admin') {
  if (!confirm(role === 'admin' ? `Jadikan ${u.email} admin?` : `Cabut akses admin ${u.email}?`)) return
  try {
    await api.request(`/api/admin/users/${u.id}`, { method: 'PATCH', body: { role } })
    toast.success('Role diperbarui')
    load()
  } catch (e: any) {
    toast.error(e.message)
  }
}

async function openHistory(u: AdminUser) {
  historyOf.value = u
  history.value = []
  try {
    history.value = await api.get(`/api/admin/users/${u.id}/transactions`)
  } catch (e: any) {
    toast.error(e.message)
  }
}

const fmtDate = (s: string) => new Date(s).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' })
</script>

<template>
  <div class="adm">
    <div class="card sec">
      <div class="bar">
        <input v-model="q" class="input sm" placeholder="Cari email / nama…" style="max-width: 280px" />
        <label class="row small"><input v-model="guests" type="checkbox" /> Tampilkan akun tamu</label>
        <div class="grow" />
        <span class="muted small">{{ data?.total ?? 0 }} user</span>
        <button class="btn sm" :disabled="loading" @click="load"><span v-if="loading" class="spin" />Muat ulang</button>
      </div>
      <div class="note">Tahap testing: saldo ditambah manual oleh admin. Setiap penambahan/koreksi tercatat di riwayat saldo user dan di activity log.</div>
      <div class="tbl-wrap">
        <table class="tbl">
          <thead>
            <tr><th>User</th><th>Role</th><th class="r">Saldo</th><th class="r">Total top up</th><th class="r">Pemakaian AI</th><th class="r">Generate</th><th>Daftar</th><th /></tr>
          </thead>
          <tbody>
            <tr v-for="u in data?.items" :key="u.id">
              <td>
                <div><b>{{ u.name || '—' }}</b> <span v-if="u.isGuest" class="tag">tamu</span></div>
                <div class="muted small">{{ u.email }}</div>
              </td>
              <td><span class="tag" :class="u.role === 'admin' ? 'acc' : ''">{{ u.role }}</span></td>
              <td class="r nowrap"><b>{{ rupiah(u.balance) }}</b></td>
              <td class="r nowrap">{{ rupiah(u.topUp) }}</td>
              <td class="r nowrap">{{ rupiah(u.spent) }}</td>
              <td class="r">{{ u.aiHits }}</td>
              <td class="nowrap small">{{ fmtDate(u.createdAt) }}</td>
              <td class="nowrap r">
                <button class="btn sm primary" @click="openTopup(u)"><Icon name="plus" :size="14" /> Saldo</button>
                <button class="btn sm ghost" title="Riwayat saldo" @click="openHistory(u)"><Icon name="clock" :size="14" /></button>
                <button v-if="!u.isGuest && u.role !== 'admin'" class="btn sm ghost" title="Jadikan admin" @click="setRole(u, 'admin')"><Icon name="shield" :size="14" /></button>
                <button v-if="u.role === 'admin' && u.id !== session.user.value?.id" class="btn sm ghost danger" title="Cabut admin" @click="setRole(u, 'user')"><Icon name="shield" :size="14" /></button>
              </td>
            </tr>
            <tr v-if="data && !data.items.length"><td colspan="8" class="muted">Tidak ada user.</td></tr>
          </tbody>
        </table>
      </div>
      <div v-if="pages > 1" class="pager">
        <button class="btn sm" :disabled="page <= 1" @click="page--, load()">‹</button>
        <span>{{ page }} / {{ pages }}</span>
        <button class="btn sm" :disabled="page >= pages" @click="page++, load()">›</button>
      </div>
    </div>

    <!-- Tambah saldo -->
    <div v-if="target" class="modal-back" @click.self="target = null">
      <form class="modal col" @submit.prevent="submitTopup">
        <h3>Tambah saldo</h3>
        <p class="muted small" style="margin: 0">{{ target.email }} · saldo sekarang <b>{{ rupiah(target.balance) }}</b></p>
        <div class="row" style="flex-wrap: wrap">
          <button v-for="v in QUICK" :key="v" type="button" class="chip" :class="{ on: amount === v }" @click="amount = v">{{ rupiah(v) }}</button>
        </div>
        <label><span class="label">Jumlah (Rp) — negatif untuk koreksi</span><input v-model.number="amount" type="number" step="100" class="input" required /></label>
        <label><span class="label">Catatan (opsional)</span><input v-model="note" class="input" placeholder="mis. saldo uji coba" /></label>
        <p class="muted small" style="margin: 0">Saldo setelahnya: <b>{{ rupiah(target.balance + (amount || 0)) }}</b></p>
        <div class="row" style="justify-content: flex-end">
          <button type="button" class="btn" @click="target = null">Batal</button>
          <button class="btn primary" :disabled="saving || !amount"><span v-if="saving" class="spin" />Simpan</button>
        </div>
      </form>
    </div>

    <!-- Riwayat saldo -->
    <div v-if="historyOf" class="modal-back" @click.self="historyOf = null">
      <div class="modal" style="width: min(760px, 100%)">
        <h3>Riwayat saldo · {{ historyOf.email }}</h3>
        <p v-if="!history.length" class="muted">Belum ada transaksi.</p>
        <div v-else class="tbl-wrap">
          <table class="tbl">
            <thead><tr><th>Waktu</th><th>Jenis</th><th>Keterangan</th><th class="r">Jumlah</th><th class="r">Saldo</th></tr></thead>
            <tbody>
              <tr v-for="t in history" :key="t.id">
                <td class="nowrap small">{{ fmtDate(t.createdAt) }}</td>
                <td><span class="tag" :class="t.amount > 0 ? 'ok' : 'acc'">{{ t.kind }}</span></td>
                <td class="small">{{ t.description }}</td>
                <td class="r nowrap">{{ rupiah(t.amount) }}</td>
                <td class="r nowrap">{{ rupiah(t.balanceAfter) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="row" style="justify-content: flex-end; margin-top: 12px"><button class="btn" @click="historyOf = null">Tutup</button></div>
      </div>
    </div>
  </div>
</template>
