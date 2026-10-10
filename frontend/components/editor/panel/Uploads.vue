<script setup lang="ts">
import type { Asset } from '~/composables/useApi'

const editor = useEditor()
const api = useApi()
const toast = useToast()
const assets = ref<Asset[]>([])
const uploading = ref(0)
const filter = ref<'' | 'upload' | 'ai'>('')
const st = editor.state

async function load() {
  assets.value = await api.get<Asset[]>('/api/assets', { source: filter.value })
}
onMounted(load)
watch(filter, load)

async function onFiles(files: FileList | File[] | null) {
  for (const f of [...(files || [])]) {
    uploading.value++
    try {
      assets.value.unshift(await api.upload(f))
    } catch (e: any) {
      toast.error(`${f.name}: ${e.message}`)
    } finally {
      uploading.value--
    }
  }
}

async function use(a: Asset) {
  // Jika bingkai/placeholder atau gambar terpilih, ganti isinya; jika tidak, tambahkan gambar baru.
  if (st.sel.kind === 'placeholder' || st.sel.kind === 'image') await editor.replaceActiveImage(a.url)
  else await editor.addImage(a.url, { name: a.filename })
}

async function remove(a: Asset) {
  if (!confirm('Hapus file ini dari upload?')) return
  await api.del(`/api/assets/${a.id}`)
  assets.value = assets.value.filter((x) => x.id !== a.id)
}
</script>

<template>
  <div class="col">
    <label class="drop" @dragover.prevent @drop.prevent="onFiles($event.dataTransfer?.files || null)">
      <Icon name="upload" :size="22" />
      <b>Upload file</b>
      <span class="muted">Klik atau seret gambar ke sini (PNG, JPG, WEBP, SVG, GIF)</span>
      <input type="file" accept="image/*" multiple hidden @change="onFiles(($event.target as HTMLInputElement).files)" />
    </label>
    <div v-if="uploading" class="muted row"><span class="spin" /> Mengunggah {{ uploading }} file…</div>
    <p v-if="st.sel.kind === 'placeholder' || st.sel.kind === 'image'" class="muted tip">Klik gambar untuk mengganti isi {{ st.sel.kind === 'placeholder' ? 'bingkai' : 'gambar' }} terpilih.</p>
    <div class="row">
      <button class="chip" :class="{ on: filter === '' }" @click="filter = ''">Semua</button>
      <button class="chip" :class="{ on: filter === 'upload' }" @click="filter = 'upload'">Upload</button>
      <button class="chip" :class="{ on: filter === 'ai' }" @click="filter = 'ai'">Dari AI</button>
    </div>
    <div class="grid">
      <div v-for="a in assets" :key="a.id" class="it">
        <img :src="a.url" :alt="a.filename" draggable="true" @dragstart="(e) => e.dataTransfer?.setData('text/racik-image', a.url)" @click="use(a)" />
        <div class="acts">
          <button class="mini" title="Jadikan latar" @click="editor.setBackgroundImage(a.url)"><Icon name="image" :size="13" /></button>
          <button class="mini" title="Hapus" @click="remove(a)"><Icon name="trash" :size="13" /></button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.drop { border: 2px dashed var(--line); border-radius: 12px; padding: 18px; display: flex; flex-direction: column; align-items: center; gap: 6px; text-align: center; cursor: pointer; font-size: 12px; }
.drop:hover { border-color: var(--accent); color: var(--accent); }
.tip { font-size: 12px; margin: 0; }
.grid { columns: 2; column-gap: 8px; }
.it { break-inside: avoid; margin-bottom: 8px; position: relative; }
.it img { width: 100%; display: block; border-radius: 8px; cursor: pointer; border: 1px solid var(--line); background: repeating-conic-gradient(#eee 0 25%, #fff 0 50%) 0 0/12px 12px; }
.it img:hover { outline: 2px solid var(--accent); }
.acts { position: absolute; top: 4px; right: 4px; display: none; gap: 4px; }
.it:hover .acts { display: flex; }
.mini { width: 24px; height: 24px; border-radius: 6px; border: 0; background: rgba(0,0,0,.65); color: #fff; display: grid; place-items: center; cursor: pointer; }
</style>
