<script setup lang="ts">
// Kotak chat AI melayang di bawah kanvas: satu tempat untuk memperbaiki halaman, menambah halaman,
// atau membuat gambar AI. Setiap instruksi dicatat di tab Riwayat (sidebar kanan).
import { rupiah, type AIModels, type Asset } from '~/composables/useApi'
import type { AiTurn } from '~/composables/useEditor'

const editor = useEditor()
const doc = useDoc()
const api = useApi()
const toast = useToast()
const turns = useAiTurns()
const st = editor.state

const MODES = [
  { k: 'refine', label: 'Perbaiki', icon: 'wand', hint: 'AI menyusun ulang halaman aktif' },
  { k: 'design', label: 'Halaman baru', icon: 'sparkles', hint: 'AI membuat halaman baru dari brief' },
  { k: 'image', label: 'Gambar', icon: 'image', hint: 'AI membuat gambar dan menaruhnya di halaman aktif' },
] as const
const mode = ref<AiTurn['mode']>('refine')
const text = ref('')
const busy = ref(false)
const models = ref<AIModels | null>(null)
const ta = ref<HTMLTextAreaElement>()

onMounted(async () => {
  try {
    models.value = await api.get<AIModels>('/api/ai/models')
  } catch {}
})

const cost = computed(() => {
  const p = models.value?.prices
  if (!p) return ''
  if (mode.value === 'image') return rupiah(p.image)
  return mode.value === 'design' ? `${rupiah(p.text)} + ${rupiah(p.image)}/gambar` : rupiah(p.text)
})
const placeholder = computed(() => ({
  refine: 'Mau ubah apa di halaman ini? mis. buat lebih minimalis, warna lebih cerah',
  design: 'Mau buat halaman apa? mis. slide penutup dengan ajakan follow Instagram',
  image: st.sel.kind === 'placeholder' ? (st.sel.imagePrompt || 'Deskripsikan gambar untuk placeholder ini') : 'Deskripsikan gambarnya, mis. kopi susu di meja kayu, cahaya pagi',
}[mode.value]))

function designType() {
  const r = st.width / st.height
  if (r > 1.6) return 'presentation'
  if (r < 0.7) return 'story'
  return 'custom'
}

function autosize() {
  const el = ta.value
  if (!el) return
  el.style.height = 'auto'
  el.style.height = Math.min(el.scrollHeight, 160) + 'px'
}

async function send() {
  const prompt = text.value.trim() || (mode.value === 'image' ? st.sel.imagePrompt : '')
  if (!prompt || busy.value) return
  const turn = reactive<AiTurn>({ id: Math.random().toString(36).slice(2), mode: mode.value, prompt, status: 'busy', message: 'Sedang diproses…', at: Date.now() })
  turns.value.push(turn)
  busy.value = true
  text.value = ''
  nextTick(autosize)
  try {
    if (turn.mode === 'refine') {
      const res = await api.post<any>('/api/ai/refine', { spec: { pages: [editor.currentPageSpec()] }, instruction: prompt, width: st.width, height: st.height, generateImages: false })
      await editor.applySpec({ pages: res.spec.pages.slice(0, 1) }, 'replace-page')
      afterDesign(res)
      turn.message = `Halaman ${st.pageIndex + 1} diperbaiki — tekan Undo kalau kurang cocok`
    } else if (turn.mode === 'design') {
      const res = await api.post<any>('/api/ai/design', { prompt, designType: designType(), width: st.width, height: st.height, pages: 1, style: '', generateImages: true })
      await editor.applySpec(res.spec, 'append')
      afterDesign(res)
      turn.message = `Halaman baru ditambahkan (halaman ${st.pageIndex + 1})`
    } else {
      const sel = st.sel
      const [w, h] = sel.kind === 'placeholder' || sel.kind === 'image' ? [sel.width, sel.height] : [st.width, st.height]
      const res = await api.post<{ asset: Asset }>('/api/ai/image', { prompt, style: '', width: w, height: h })
      if (sel.kind === 'placeholder') await editor.replaceActiveImage(res.asset.url, prompt)
      else await editor.addImage(res.asset.url, { name: 'Gambar AI', imagePrompt: prompt })
      turn.message = 'Gambar ditambahkan ke halaman'
    }
    turn.status = 'ok'
  } catch (e: any) {
    turn.status = 'error'
    turn.message = e.code === 'insufficient_balance' ? 'Saldo tidak cukup — top up di halaman Saldo' : e.message
    toast.error(turn.message)
  } finally {
    busy.value = false
    api.refreshUser() // saldo terpotong / dikembalikan
  }
}

function afterDesign(res: { generationId: string; warnings: string[] }) {
  res.warnings?.forEach((w) => toast.error(w))
  doc.generationId = res.generationId
  doc.rated = false
}

function onKey(e: KeyboardEvent) {
  if (e.key === 'Enter' && !e.shiftKey && !e.isComposing) {
    e.preventDefault()
    send()
  }
}
</script>

<template>
  <div class="chat" :class="{ busy }">
    <textarea ref="ta" v-model="text" rows="1" :placeholder="placeholder" @input="autosize" @keydown="onKey" />
    <div class="bar">
      <div class="modes">
        <button v-for="m in MODES" :key="m.k" :class="{ on: mode === m.k }" :title="m.hint" @click="mode = m.k"><Icon :name="m.icon" :size="17" /> <span>{{ m.label }}</span></button>
      </div>
      <span class="grow" />
      <span v-if="cost" class="cost muted">{{ cost }}</span>
      <button class="send" :class="{ ready: !!text.trim() }" :disabled="busy" title="Kirim (Enter)" @click="send">
        <span v-if="busy" class="spin" /><Icon v-else name="arrowUp" :size="22" />
      </button>
    </div>
  </div>
</template>

<style scoped>
.chat { background: #fff; border: 1px solid var(--line); border-radius: 22px; box-shadow: 0 12px 40px rgba(60,40,120,.14); padding: 14px 14px 12px; display: flex; flex-direction: column; gap: 10px; }
textarea { border: 0; outline: none; resize: none; font-size: 16px; line-height: 1.5; padding: 4px 8px; background: none; max-height: 160px; }
textarea::placeholder { color: #8a8796; }
.bar { display: flex; align-items: center; gap: 8px; }
.modes { display: flex; background: #f1f0f7; border-radius: 12px; padding: 3px; gap: 2px; }
.modes button { border: 0; background: none; height: 34px; padding: 0 12px; border-radius: 9px; display: inline-flex; align-items: center; gap: 6px; cursor: pointer; color: var(--muted); font-weight: 600; font-size: 13px; white-space: nowrap; }
.modes button.on { background: #fff; color: var(--accent); box-shadow: 0 1px 3px rgba(16,24,40,.12); }
.cost { font-size: 12px; white-space: nowrap; }
.send { width: 44px; height: 44px; border-radius: 12px; border: 0; background: #b89cf0; color: #fff; display: grid; place-items: center; cursor: pointer; flex: 0 0 44px; transition: background .15s; }
.send.ready { background: var(--accent); }
.send:disabled { cursor: progress; }
@media (max-width: 640px) {
  .modes button span { display: none; }
  .cost { display: none; }
}
</style>
