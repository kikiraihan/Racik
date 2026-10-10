<script setup lang="ts">
// Sidebar kanan editor: akun, Export, aksi dokumen (undo/redo/simpan), tab Riwayat AI & Properti, saldo.
import { rupiah } from '~/composables/useApi'

const props = defineProps<{ tab: 'history' | 'props' }>()
const emit = defineEmits<{ collapse: []; save: []; fit: []; 'update:tab': ['history' | 'props']; 'open-tool': [string] }>()
const editor = useEditor()
const doc = useDoc()
const session = useSession()
const turns = useAiTurns()
const st = editor.state
const showExport = ref(false)
const showResize = ref(false)
const showKeys = ref(false)
const tipOpen = ref(true)

const user = computed(() => session.user.value)
const name = computed(() => user.value?.name || (user.value?.email ? user.value.email.split('@')[0] : 'Tamu'))
const status = computed(() => (doc.saving ? 'Menyimpan…' : doc.dirty ? 'Belum tersimpan' : doc.savedAt ? 'Tersimpan' : ''))
const MODE: Record<string, string> = { refine: 'Perbaiki halaman', design: 'Halaman baru', image: 'Gambar AI' }
const time = (t: number) => new Date(t).toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' })
</script>

<template>
  <aside class="rs">
    <div class="hd">
      <NuxtLink to="/wallet" target="_blank" class="me" title="Akun & saldo">
        <span class="av">{{ name.slice(0, 1).toUpperCase() }}</span>
        <span class="nm">{{ name }}</span>
      </NuxtLink>
      <button class="ib" title="Tutup sidebar" @click="emit('collapse')"><Icon name="panelRight" :size="20" /></button>
    </div>

    <div class="pad">
      <button class="btn primary exp" @click="showExport = true"><Icon name="download" :size="18" /> Export <Icon name="down" :size="16" /></button>
      <div class="acts">
        <button class="ib sm" :disabled="!st.canUndo" title="Undo (Ctrl+Z)" @click="editor.undo()"><Icon name="undo" :size="16" /></button>
        <button class="ib sm" :disabled="!st.canRedo" title="Redo (Ctrl+Y)" @click="editor.redo()"><Icon name="redo" :size="16" /></button>
        <span class="status grow">{{ status }}</span>
        <button class="ib sm" title="Ubah ukuran" @click="showResize = true"><Icon name="resize" :size="16" /></button>
        <button class="ib sm" title="Pintasan keyboard" @click="showKeys = true"><Icon name="keyboard" :size="16" /></button>
        <button class="ib sm" title="Simpan (Ctrl+S)" @click="emit('save')"><Icon name="check" :size="16" /></button>
      </div>
      <div class="seg">
        <button :class="{ on: props.tab === 'history' }" @click="emit('update:tab', 'history')">Riwayat</button>
        <button :class="{ on: props.tab === 'props' }" @click="emit('update:tab', 'props')">Properti</button>
      </div>
    </div>

    <div class="body scroll">
      <!-- Riwayat instruksi AI dari kotak chat -->
      <template v-if="props.tab === 'history'">
        <p v-if="!turns.length" class="muted empty">Belum ada riwayat. Tulis instruksi di kotak bawah untuk memulai.</p>
        <div v-for="t in [...turns].reverse()" :key="t.id" class="turn" :class="t.status">
          <div class="row th"><span class="badge">{{ MODE[t.mode] }}</span><span class="grow" /><span class="muted small">{{ time(t.at) }}</span></div>
          <p class="pr">{{ t.prompt }}</p>
          <p class="msg small"><span v-if="t.status === 'busy'" class="spin" /> {{ t.message }}</p>
        </div>
      </template>
      <!-- Properti objek terpilih -->
      <template v-else>
        <EditorPropsPanel v-if="st.sel.kind !== 'none'" @close="emit('update:tab', 'history')" />
        <p v-else class="muted empty">Pilih elemen di kanvas untuk melihat posisi, ukuran, bayangan, dan filter.</p>
      </template>
    </div>

    <div class="foot">
      <EditorAiFeedback v-if="doc.generationId && !doc.rated" />
      <div class="tip">
        <div class="art"><span v-for="ic in ['sparkles', 'image', 'text', 'wand', 'template']" :key="ic" class="ti"><Icon :name="ic" :size="20" /></span></div>
        <div class="row tp"><b class="grow">AI Magic lengkap</b><button class="ib sm" @click="tipOpen = !tipOpen"><Icon :name="tipOpen ? 'up' : 'down'" :size="16" /></button></div>
        <template v-if="tipOpen">
          <p class="muted small">Atur jumlah halaman, gaya, gambar AI, edit gambar, dan copywriting dari panel AI Magic.</p>
          <button class="btn primary full" @click="emit('open-tool', 'ai')"><Icon name="sparkles" :size="16" /> Buka AI Magic <Icon name="right" :size="14" /></button>
        </template>
      </div>
      <NuxtLink to="/wallet" target="_blank" class="saldo"><Icon name="wallet" :size="18" /><span class="grow">Saldo</span><b>{{ rupiah(user?.balance ?? 0) }}</b></NuxtLink>
    </div>

    <EditorExportDialog v-if="showExport" :title="doc.title" @close="showExport = false" />
    <EditorResizeDialog v-if="showResize" @close="showResize = false; emit('fit')" />
    <EditorShortcutsDialog v-if="showKeys" @close="showKeys = false" />
  </aside>
</template>

<style scoped>
.rs { width: 100%; height: 100%; display: flex; flex-direction: column; background: #fff; border-left: 1px solid var(--line); }
.hd { display: flex; align-items: center; gap: 8px; padding: 14px 16px 10px; }
.me { flex: 1; min-width: 0; display: flex; align-items: center; gap: 10px; }
.av { width: 38px; height: 38px; border-radius: 50%; background: var(--grad); color: #fff; display: grid; place-items: center; font-weight: 700; flex: 0 0 38px; }
.nm { font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.ib { border: 0; background: none; width: 36px; height: 36px; border-radius: 10px; display: grid; place-items: center; cursor: pointer; color: var(--ink-2); flex: 0 0 auto; }
.ib:hover:not(:disabled) { background: var(--panel-2); color: var(--ink); }
.ib:disabled { opacity: .35; cursor: default; }
.ib.sm { width: 30px; height: 30px; }
.pad { padding: 4px 16px 0; }
.exp { width: 100%; height: 48px; border-radius: 12px; font-size: 16px; box-shadow: 0 0 0 3px var(--accent-soft); }
.acts { display: flex; align-items: center; gap: 2px; margin: 10px 0 8px; }
.status { font-size: 12px; color: var(--muted); padding-left: 6px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.seg { background: #f1f0f7; border-radius: 12px; padding: 4px; display: grid; grid-template-columns: 1fr 1fr; gap: 4px; }
.seg button { border: 0; background: none; height: 38px; border-radius: 9px; cursor: pointer; font-weight: 600; color: var(--muted); }
.seg button.on { background: #fff; color: var(--ink); box-shadow: 0 1px 3px rgba(16,24,40,.1); }
.body { flex: 1; min-height: 0; padding: 8px 16px; }
.body :deep(.pp) { padding: 0; }
.body :deep(.pp .hd) { padding-top: 4px; }
.empty { font-size: 14px; line-height: 1.5; margin: 12px 2px; }
.turn { border: 1px solid var(--line); border-radius: 12px; padding: 10px 12px; margin-bottom: 8px; }
.turn.error { border-color: #f5c2c4; background: #fff7f7; }
.turn.ok .msg { color: var(--ok); }
.turn.error .msg { color: var(--danger); }
.pr { margin: 6px 0 4px; font-size: 13px; line-height: 1.45; white-space: pre-wrap; }
.msg { margin: 0; display: flex; align-items: center; gap: 6px; color: var(--muted); }
.msg .spin { width: 12px; height: 12px; }
.small { font-size: 12px; }
.foot { padding: 0 16px 16px; display: flex; flex-direction: column; gap: 10px; }
.foot :deep(.fb) { position: static; width: auto; box-shadow: none; }
.tip { border: 1px solid var(--line); border-radius: 16px; overflow: hidden; }
.art { background: #f4f2fb; display: flex; justify-content: center; gap: 8px; padding: 16px 10px; }
.ti { width: 40px; height: 40px; border-radius: 12px; background: #fff; display: grid; place-items: center; color: var(--accent); box-shadow: 0 2px 8px rgba(16,24,40,.08); }
.ti:nth-child(odd) { transform: rotate(-6deg); }
.ti:nth-child(even) { transform: rotate(5deg) translateY(-3px); }
.tp { padding: 10px 12px 0; }
.tip p { margin: 4px 12px 10px; line-height: 1.5; }
.tip .full { width: calc(100% - 24px); margin: 0 12px 12px; height: 42px; border-radius: 10px; }
.saldo { display: flex; align-items: center; gap: 10px; background: #f4f2fb; border-radius: 14px; padding: 12px 14px; color: var(--ink-2); }
.saldo b { color: var(--accent); }
@media (max-height: 760px) { .art { display: none; } }
</style>
