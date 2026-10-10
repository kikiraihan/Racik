<script setup lang="ts">
const emit = defineEmits<{ fit: [] }>()
const editor = useEditor()
const st = editor.state
const expanded = ref(true)
const zoomPct = computed(() => Math.round(st.zoom * 100))
</script>

<template>
  <div class="pb">
    <div class="tools row">
      <button class="btn ghost sm" @click="expanded = !expanded"><Icon :name="expanded ? 'down' : 'up'" :size="14" /> Halaman {{ st.pageIndex + 1 }} / {{ st.pages.length }}</button>
      <div class="grow" />
      <button class="btn ghost sm icon" title="Perkecil" @click="editor.setZoom(st.zoom / 1.15)"><Icon name="zoomOut" :size="16" /></button>
      <input class="zr" type="range" min="0.05" max="3" step="0.01" :value="st.zoom" @input="editor.setZoom(+($event.target as HTMLInputElement).value)" />
      <button class="btn ghost sm icon" title="Perbesar" @click="editor.setZoom(st.zoom * 1.15)"><Icon name="zoomIn" :size="16" /></button>
      <span class="pct">{{ zoomPct }}%</span>
      <button class="btn ghost sm icon" title="Pas ke layar (Ctrl+0)" @click="emit('fit')"><Icon name="fit" :size="16" /></button>
    </div>
    <div v-if="expanded" class="strip scroll">
      <div v-for="(p, i) in st.pages" :key="p.id" class="pg" :class="{ on: i === st.pageIndex }" @click="editor.goTo(i)">
        <div class="th" :style="{ aspectRatio: `${st.width}/${st.height}` }">
          <img v-if="p.thumb" :src="p.thumb" alt="" />
          <span class="n">{{ i + 1 }}</span>
        </div>
        <div v-if="i === st.pageIndex" class="pa">
          <button title="Geser kiri" @click.stop="editor.movePage(i, -1)"><Icon name="left" :size="12" /></button>
          <button title="Duplikat halaman" @click.stop="editor.duplicatePage(i)"><Icon name="copy" :size="12" /></button>
          <button title="Hapus halaman" :disabled="st.pages.length < 2" @click.stop="editor.deletePage(i)"><Icon name="trash" :size="12" /></button>
          <button title="Geser kanan" @click.stop="editor.movePage(i, 1)"><Icon name="right" :size="12" /></button>
        </div>
      </div>
      <button class="add" title="Tambah halaman" @click="editor.addPage()"><Icon name="plus" :size="20" /></button>
    </div>
  </div>
</template>

<style scoped>
.pb { background: #fff; border-top: 1px solid var(--line); flex: 0 0 auto; }
.tools { height: 40px; padding: 0 10px; gap: 4px; }
.zr { width: 120px; }
.pct { font-size: 12px; width: 42px; text-align: right; color: var(--ink-2); }
.strip { display: flex; gap: 10px; padding: 4px 12px 12px; align-items: flex-start; }
.pg { flex: 0 0 auto; height: 78px; cursor: pointer; position: relative; }
.th { height: 64px; border-radius: 6px; overflow: hidden; border: 2px solid transparent; box-shadow: 0 0 0 1px var(--line); background: #fff; position: relative; }
.pg.on .th { border-color: var(--accent); }
.th img { width: 100%; height: 100%; object-fit: cover; display: block; }
.n { position: absolute; bottom: 2px; left: 4px; font-size: 10px; background: rgba(0,0,0,.6); color: #fff; border-radius: 4px; padding: 0 4px; }
.pa { display: flex; justify-content: center; gap: 2px; margin-top: 1px; }
.pa button { border: 0; background: var(--panel-2); border-radius: 4px; width: 20px; height: 14px; display: grid; place-items: center; cursor: pointer; padding: 0; }
.pa button:hover { background: var(--accent-soft); color: var(--accent); }
.add { flex: 0 0 64px; height: 64px; border: 2px dashed var(--line); border-radius: 6px; background: none; cursor: pointer; color: var(--muted); display: grid; place-items: center; }
.add:hover { border-color: var(--accent); color: var(--accent); }
</style>
