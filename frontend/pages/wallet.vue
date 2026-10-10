<script setup lang="ts">
// Saldo user: fitur AI dibayar per hit dari saldo; fitur edit biasa gratis.
import { rupiah, type AIPrices, type WalletTx } from '~/composables/useApi'

const api = useApi()
const toast = useToast()
const session = useSession()
const data = ref<{ balance: number; prices: AIPrices; maxImagesPerDesign: number; transactions: WalletTx[] } | null>(null)

const KIND_LABEL: Record<string, string> = { topup: 'Top up', charge: 'Pemakaian AI', refund: 'Pengembalian', adjust: 'Koreksi' }

onMounted(load)

async function load() {
  try {
    await api.ensureSession()
    data.value = await api.get('/api/wallet')
    if (session.user.value) session.user.value.balance = data.value!.balance
  } catch (e: any) {
    toast.error(e.message)
  }
}

const fmtDate = (s: string) => new Date(s).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' })
</script>

<template>
  <div>
    <AppHeader />
    <main class="wrap">
      <h1>Saldo</h1>
      <p class="muted">Fitur AI (Magic Design, gambar AI, edit gambar AI, copywriting) memotong saldo setiap kali dipakai. Fitur edit desain biasa tetap gratis.</p>

      <div v-if="data" class="grid">
        <div class="card sec bal">
          <span class="label">Saldo kamu</span>
          <div class="amt">{{ rupiah(data.balance) }}</div>
          <p class="muted small">
            Top up saldo segera hadir. Selama tahap uji coba, hubungi admin untuk menambah saldo.
            <template v-if="session.user.value?.isGuest"><br /><NuxtLink to="/login?mode=register" class="lnk">Simpan akun tamu</NuxtLink> supaya saldo tidak hilang.</template>
          </p>
        </div>
        <div class="card sec">
          <span class="label">Tarif AI</span>
          <table class="tbl">
            <tbody>
              <tr><td>Magic Design / Perbaiki desain</td><td class="r">{{ rupiah(data.prices.text) }} <span class="muted">+ {{ rupiah(data.prices.image) }}/gambar</span></td></tr>
              <tr><td>Gambar AI</td><td class="r">{{ rupiah(data.prices.image) }} / gambar</td></tr>
              <tr><td>Edit gambar AI</td><td class="r">{{ rupiah(data.prices.edit) }} / edit</td></tr>
              <tr><td>Copywriting</td><td class="r">{{ rupiah(data.prices.text) }} / permintaan</td></tr>
            </tbody>
          </table>
          <p class="muted small">Gambar maks {{ data.maxImagesPerDesign }} per desain. Kalau AI gagal, saldo otomatis dikembalikan.</p>
        </div>
      </div>

      <div v-if="data" class="card sec">
        <span class="label">Riwayat transaksi</span>
        <p v-if="!data.transactions.length" class="muted">Belum ada transaksi.</p>
        <table v-else class="tbl">
          <thead><tr><th>Waktu</th><th>Jenis</th><th>Keterangan</th><th class="r">Jumlah</th><th class="r">Saldo</th></tr></thead>
          <tbody>
            <tr v-for="t in data.transactions" :key="t.id">
              <td class="nowrap">{{ fmtDate(t.createdAt) }}</td>
              <td><span class="kind" :class="t.kind">{{ KIND_LABEL[t.kind] || t.kind }}</span></td>
              <td>{{ t.description }}</td>
              <td class="r nowrap" :class="t.amount < 0 ? 'neg' : 'pos'">{{ t.amount > 0 ? '+' : '' }}{{ rupiah(t.amount) }}</td>
              <td class="r nowrap">{{ rupiah(t.balanceAfter) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <div v-else class="muted">Memuat…</div>
    </main>
  </div>
</template>

<style scoped>
.wrap { max-width: 960px; margin: 0 auto; padding: 28px 16px 60px; }
h1 { margin: 0 0 6px; }
.grid { display: grid; grid-template-columns: 1fr 1.4fr; gap: 16px; margin: 18px 0 16px; }
.sec { padding: 18px; }
.amt { font-size: 34px; font-weight: 800; letter-spacing: -.02em; margin: 4px 0 8px; }
.small { font-size: 12px; line-height: 1.5; margin: 0; }
.lnk { color: var(--accent); font-weight: 600; }
.tbl { width: 100%; border-collapse: collapse; font-size: 13px; margin-bottom: 8px; }
.tbl th { text-align: left; font-size: 12px; color: var(--muted); font-weight: 600; padding: 6px 8px; border-bottom: 1px solid var(--line); }
.tbl td { padding: 8px; border-bottom: 1px solid var(--line); vertical-align: top; }
.r { text-align: right; }
.nowrap { white-space: nowrap; }
.neg { color: var(--danger); font-weight: 600; }
.pos { color: var(--ok); font-weight: 600; }
.kind { font-size: 11px; font-weight: 700; padding: 2px 8px; border-radius: 999px; background: var(--panel-2); }
.kind.topup, .kind.refund { background: #e7f8ee; color: var(--ok); }
.kind.charge { background: var(--accent-soft); color: var(--accent); }
@media (max-width: 720px) { .grid { grid-template-columns: 1fr; } .tbl { display: block; overflow-x: auto; } }
</style>
