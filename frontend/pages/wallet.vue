<script setup lang="ts">
// Saldo user: fitur AI dibayar per hit dari saldo; fitur edit biasa gratis.
import { rupiah, type AIPrices, type WalletTx } from '~/composables/useApi'

definePageMeta({ layout: 'app' })

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
  <div class="page">
    <div class="page-head">
      <div class="grow">
        <h1>Saldo</h1>
        <p>Fitur AI (Magic Design, gambar AI, edit gambar AI, copywriting) memotong saldo setiap kali dipakai. Fitur edit desain biasa tetap gratis.</p>
      </div>
    </div>

    <div v-if="data" class="grid">
      <div class="panel sec">
        <div class="muted">Saldo kamu</div>
        <div class="amt">{{ rupiah(data.balance) }}</div>
        <p class="muted small">
          Top up saldo segera hadir. Selama tahap uji coba, hubungi admin untuk menambah saldo.
          <template v-if="session.user.value?.isGuest"><br /><NuxtLink to="/login?mode=register" class="lnk">Simpan akun tamu</NuxtLink> supaya saldo tidak hilang.</template>
        </p>
      </div>
      <div class="panel sec">
        <div class="sec-title">Tarif AI</div>
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

    <template v-if="data">
      <div class="sec-head"><h2>Riwayat transaksi</h2></div>
      <div v-if="!data.transactions.length" class="empty"><b>Belum ada transaksi</b>Pemakaian fitur AI akan tercatat di sini.</div>
      <div v-else class="panel tbl-wrap">
        <table class="tbl hist">
          <thead><tr><th>Waktu</th><th>Jenis</th><th>Keterangan</th><th class="r">Jumlah</th><th class="r">Saldo</th></tr></thead>
          <tbody>
            <tr v-for="t in data.transactions" :key="t.id">
              <td class="nowrap muted">{{ fmtDate(t.createdAt) }}</td>
              <td><span class="kind" :class="t.kind">{{ KIND_LABEL[t.kind] || t.kind }}</span></td>
              <td>{{ t.description }}</td>
              <td class="r nowrap" :class="t.amount < 0 ? 'neg' : 'pos'">{{ t.amount > 0 ? '+' : '' }}{{ rupiah(t.amount) }}</td>
              <td class="r nowrap">{{ rupiah(t.balanceAfter) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </template>
    <div v-else class="muted">Memuat…</div>
  </div>
</template>

<style scoped>
.grid { display: grid; grid-template-columns: 1fr 1.4fr; gap: 16px; }
.sec { padding: 20px; }
.panel + .panel { margin-top: 0; }
.sec-title { font-weight: 600; margin-bottom: 8px; }
.amt { font-size: 32px; font-weight: 600; letter-spacing: -.02em; margin: 6px 0 10px; }
.small { font-size: 12px; line-height: 1.5; margin: 0; }
.tbl-wrap { overflow-x: auto; }
.tbl { width: 100%; border-collapse: collapse; font-size: 13px; margin-bottom: 8px; }
.tbl th { text-align: left; font-size: 13px; color: var(--muted); font-weight: 500; padding: 10px 12px; border-bottom: 1px solid var(--line); }
.tbl td { padding: 9px 12px; border-bottom: 1px solid var(--line); vertical-align: top; }
.sec .tbl td { padding: 9px 0; }
.tbl tr:last-child td { border-bottom: 0; }
.hist { margin: 0; }
.hist th, .hist td { padding-left: 16px; padding-right: 16px; }
.r { text-align: right; }
.nowrap { white-space: nowrap; }
.neg { color: var(--danger); font-weight: 600; }
.pos { color: var(--ok); font-weight: 600; }
.kind { font-size: 12px; font-weight: 500; padding: 2px 8px; border-radius: 6px; background: var(--panel-2); color: var(--ink-2); white-space: nowrap; }
.kind.topup, .kind.refund { background: #ecfdf3; color: #067647; }
.kind.charge { background: var(--panel-2); color: var(--ink-2); }
@media (max-width: 720px) { .grid { grid-template-columns: 1fr; } }
</style>
