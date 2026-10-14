<script setup lang="ts">
// Brand Kit: warna, font, logo, dan tone of voice — otomatis dipakai AI saat generate.
import { FONTS } from '~/lib/presets'

definePageMeta({ layout: 'app' })

const api = useApi()
const toast = useToast()
const kit = reactive({ name: 'Brand saya', colors: [] as string[], fonts: [] as string[], logoUrl: '', voice: '' })
const newColor = ref('#8B3DFF')
const saving = ref(false)
const brandColors = useState<string[]>('brandColors', () => [])

onMounted(async () => {
  await api.ensureSession()
  Object.assign(kit, await api.get('/api/brand-kit'))
})

async function save() {
  saving.value = true
  try {
    await api.put('/api/brand-kit', kit)
    brandColors.value = [...kit.colors]
    toast.success('Brand kit tersimpan — AI akan memakainya di desain berikutnya')
  } catch (e: any) {
    toast.error(e.message)
  } finally {
    saving.value = false
  }
}

async function uploadLogo(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0]
  if (!f) return
  const a = await api.upload(f)
  kit.logoUrl = a.url
}

function toggleFont(f: string) {
  kit.fonts = kit.fonts.includes(f) ? kit.fonts.filter((x) => x !== f) : [...kit.fonts, f].slice(-3)
}
</script>

<template>
  <div class="page narrow">
    <div class="page-head">
      <div class="grow">
        <h1>Brand Kit</h1>
        <p>Atur identitas brand sekali, AI akan konsisten memakainya setiap generate desain.</p>
      </div>
      <button class="btn primary" :disabled="saving" @click="save"><span v-if="saving" class="spin" /> Simpan</button>
    </div>

    <div class="panel">
      <div class="prow">
        <div><div class="prow-title">Nama brand</div></div>
        <input v-model="kit.name" class="input" />
      </div>
      <div class="prow">
        <div><div class="prow-title">Logo</div><div class="prow-desc">Dipakai AI saat menyusun desain.</div></div>
        <div class="row">
          <img v-if="kit.logoUrl" :src="kit.logoUrl" class="logo" alt="logo" />
          <label class="btn"><Icon name="upload" :size="16" /> Upload logo<input type="file" accept="image/*" hidden @change="uploadLogo" /></label>
          <button v-if="kit.logoUrl" class="btn ghost danger" @click="kit.logoUrl = ''">Hapus</button>
        </div>
      </div>
      <div class="prow">
        <div><div class="prow-title">Warna brand</div><div class="prow-desc">Palet utama desainmu.</div></div>
        <div class="row" style="flex-wrap: wrap">
          <div v-for="(c, i) in kit.colors" :key="c + i" class="sw" :style="{ background: c }" :title="c">
            <button class="x" title="Hapus warna" @click="kit.colors.splice(i, 1)">×</button>
          </div>
          <input v-model="newColor" type="color" />
          <button class="btn sm" @click="!kit.colors.includes(newColor) && kit.colors.push(newColor)">Tambah</button>
        </div>
      </div>
      <div class="prow">
        <div><div class="prow-title">Font brand</div><div class="prow-desc">Pilih maksimal 3.</div></div>
        <div class="row" style="flex-wrap: wrap; gap: 6px">
          <button v-for="f in FONTS" :key="f" class="chip" :class="{ on: kit.fonts.includes(f) }" :style="{ fontFamily: f }" @click="toggleFont(f)">{{ f }}</button>
        </div>
      </div>
      <div class="prow">
        <div><div class="prow-title">Tone of voice</div><div class="prow-desc">Gaya bahasa untuk teks yang ditulis AI.</div></div>
        <textarea v-model="kit.voice" class="textarea" placeholder="mis. Santai, akrab, pakai bahasa gaul anak muda, tanpa emoji berlebihan" />
      </div>
    </div>
  </div>
</template>

<style scoped>
.sw { width: 40px; height: 40px; border-radius: 8px; position: relative; border: 1px solid rgba(0,0,0,.1); }
.x { position: absolute; top: -6px; right: -6px; width: 18px; height: 18px; border-radius: 50%; border: 0; background: var(--ink); color: #fff; cursor: pointer; font-size: 12px; line-height: 1; display: none; }
.sw:hover .x { display: block; }
.logo { height: 44px; border-radius: 8px; border: 1px solid var(--line); }
input[type='color'] { width: 40px; height: 40px; }
@media (hover: none) { .x { display: block; } }
</style>
