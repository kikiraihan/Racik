<script setup lang="ts">
// Konfigurasi harga: harga ke user = harga vendor + markup %, model gratis = tarif flat per hit.
import { rupiah } from '~/composables/useApi'

interface PriceRow {
  model: string
  label: string
  kinds: string[]
  active: string[]
  vendorPrice: number
  markupPercent: number | null
  userPrice: number
  isSet: boolean
  available: boolean
}

const api = useApi()
const toast = useToast()
const markup = ref(5)
const freeHit = ref(100)
const rows = ref<(PriceRow & { custom: boolean; dirty: boolean })[]>([])
const features = ref<{ text: number; image: number; edit: number } | null>(null)
const saving = ref(false)

function apply(res: any) {
  markup.value = res.markupPercent
  freeHit.value = res.freeHitPrice
  features.value = res.features
  rows.value = res.models.map((m: PriceRow) => ({ ...m, custom: m.markupPercent !== null, dirty: false }))
}

async function load() {
  try {
    apply(await api.get('/api/admin/pricing'))
  } catch (e: any) {
    toast.error(e.message)
  }
}
onMounted(load)

// Pratinjau harga user (sama dengan rumus backend: dibulatkan ke atas).
function preview(r: PriceRow & { custom: boolean }) {
  if (!(r.vendorPrice > 0)) return Math.max(freeHit.value || 0, 0)
  const m = r.custom && r.markupPercent !== null ? r.markupPercent : markup.value
  return Math.ceil(Math.round(r.vendorPrice * (1 + m / 100) * 100) / 100)
}

async function save() {
  saving.value = true
  try {
    const models = rows.value
      .filter((r) => r.dirty)
      .map((r) => ({ model: r.model, vendorPrice: Number(r.vendorPrice) || 0, markupPercent: r.custom ? Number(r.markupPercent ?? markup.value) : null }))
    apply(await api.put('/api/admin/pricing', { markupPercent: Number(markup.value), freeHitPrice: Math.round(Number(freeHit.value)), models }))
    toast.success('Harga tersimpan — langsung berlaku')
  } catch (e: any) {
    toast.error(e.message)
  } finally {
    saving.value = false
  }
}

async function reset(r: PriceRow) {
  if (!confirm(`Hapus harga vendor ${r.label}? Model ini akan dikenai tarif model gratis.`)) return
  try {
    apply(await api.put('/api/admin/pricing', { models: [{ model: r.model, remove: true }] }))
  } catch (e: any) {
    toast.error(e.message)
  }
}

const KIND: Record<string, string> = { text: 'teks', image: 'gambar', edit: 'edit' }
const unset = computed(() => rows.value.filter((r) => r.available && !r.isSet && r.active.length))
</script>

<template>
  <div class="adm">
    <div class="card sec">
      <div class="grid3 glob">
        <label><span class="label">Markup global (%)</span><input v-model.number="markup" type="number" step="0.1" class="input" /></label>
        <label><span class="label">Tarif model gratis (Rp / hit)</span><input v-model.number="freeHit" type="number" step="10" class="input" /></label>
        <div v-if="features" class="feat">
          <span class="label">Tarif aktif ke user</span>
          <div class="small">Teks/layout <b>{{ rupiah(features.text) }}</b> · Gambar <b>{{ rupiah(features.image) }}</b> · Edit <b>{{ rupiah(features.edit) }}</b></div>
        </div>
      </div>
      <p class="muted small">
        Harga user = harga vendor × (1 + markup%), dibulatkan ke atas. Model dengan harga vendor 0 / belum diatur dikenai tarif model gratis.
        1 hit = 1 kali susun layout/teks, 1 gambar, atau 1 edit gambar. Isi harga vendor dalam Rupiah per hit.
      </p>
      <div v-if="unset.length" class="note">
        Harga vendor belum diatur untuk model aktif: <b>{{ unset.map((r) => r.label).join(', ') }}</b> — saat ini dikenai tarif model gratis ({{ rupiah(freeHit) }}/hit).
      </div>
      <div class="tbl-wrap">
        <table class="tbl">
          <thead>
            <tr><th>Model</th><th>Dipakai untuk</th><th class="r">Harga vendor (Rp/hit)</th><th>Markup khusus</th><th class="r">Harga user</th><th /></tr>
          </thead>
          <tbody>
            <tr v-for="r in rows" :key="r.model" :class="{ off: !r.available }">
              <td>
                <b>{{ r.label }}</b>
                <div class="muted small">{{ r.model }}<template v-if="!r.available"> · provider tidak aktif</template></div>
              </td>
              <td>
                <span v-for="k in r.kinds" :key="k" class="tag" :class="r.active.includes(k) ? 'acc' : ''" style="margin-right: 4px">{{ KIND[k] || k }}{{ r.active.includes(k) ? ' ✓' : '' }}</span>
              </td>
              <td class="r"><input v-model.number="r.vendorPrice" type="number" min="0" step="0.01" class="input sm num" @input="r.dirty = true" /></td>
              <td class="nowrap">
                <label class="row small"><input v-model="r.custom" type="checkbox" @change="(r.dirty = true), r.custom && r.markupPercent === null && (r.markupPercent = markup)" /> khusus</label>
                <input v-if="r.custom" v-model.number="r.markupPercent" type="number" step="0.1" class="input sm num" style="margin-top: 4px" @input="r.dirty = true" />
              </td>
              <td class="r nowrap">
                <b>{{ rupiah(preview(r)) }}</b>
                <div v-if="!r.isSet && !r.dirty" class="small"><span class="tag warn">belum diatur</span></div>
                <div v-else-if="!(r.vendorPrice > 0)" class="small"><span class="tag">gratis</span></div>
              </td>
              <td class="r"><button v-if="r.isSet" class="btn sm ghost danger" title="Hapus harga" @click="reset(r)"><Icon name="trash" :size="14" /></button></td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="row" style="justify-content: flex-end; margin-top: 12px">
        <button class="btn" @click="load">Batalkan perubahan</button>
        <button class="btn primary" :disabled="saving" @click="save"><span v-if="saving" class="spin" />Simpan harga</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.glob { align-items: end; margin-bottom: 8px; }
.feat .small { line-height: 36px; }
.num { width: 110px; text-align: right; display: inline-block; }
tr.off { opacity: .55; }
@media (max-width: 720px) { .glob { grid-template-columns: 1fr; } }
</style>
