<script setup lang="ts">
// Panel layer: urutan tumpukan (atas = paling depan), drag untuk pindah, sembunyikan, kunci, ganti nama.
const editor = useEditor()
const st = editor.state
const dragId = ref('')
const overId = ref('')
const renaming = ref('')
const ICON: Record<string, string> = { text: 'text', image: 'image', group: 'group', path: 'brush', line: 'minus' }

function onDrop(targetId: string) {
  const from = dragId.value
  dragId.value = overId.value = ''
  if (!from || from === targetId) return
  // state.layers terbalik (atas = index terakhir di canvas)
  const total = st.layers.length
  const targetIdx = st.layers.findIndex((l) => l.id === targetId)
  editor.moveLayerTo(from, total - 1 - targetIdx)
}

function finishRename(id: string, e: Event) {
  editor.renameLayer(id, (e.target as HTMLInputElement).value)
  renaming.value = ''
}
</script>

<template>
  <div>
    <div v-if="!st.layers.length" class="muted">Belum ada objek di halaman ini.</div>
    <div class="row acts" v-if="st.sel.kind !== 'none'">
      <button class="btn sm icon" title="Paling depan" @click="editor.arrange('front')"><Icon name="up" :size="14" /><Icon name="up" :size="14" style="margin-left: -10px" /></button>
      <button class="btn sm icon" title="Maju" @click="editor.arrange('forward')"><Icon name="up" :size="14" /></button>
      <button class="btn sm icon" title="Mundur" @click="editor.arrange('backward')"><Icon name="down" :size="14" /></button>
      <button class="btn sm icon" title="Paling belakang" @click="editor.arrange('back')"><Icon name="down" :size="14" /><Icon name="down" :size="14" style="margin-left: -10px" /></button>
      <div class="grow" />
      <button v-if="st.sel.kind === 'multi'" class="btn sm" @click="editor.group()"><Icon name="group" :size="14" /> Grup</button>
      <button v-if="st.sel.kind === 'group'" class="btn sm" @click="editor.ungroup()"><Icon name="ungroup" :size="14" /> Pisah</button>
    </div>
    <ul class="list">
      <li v-for="l in st.layers" :key="l.id" class="ly" :class="{ sel: l.selected, over: overId === l.id, hidden: !l.visible }" draggable="true"
        @dragstart="dragId = l.id" @dragover.prevent="overId = l.id" @dragleave="overId = ''" @drop.prevent="onDrop(l.id)"
        @click="(e) => editor.selectLayer(l.id, e.shiftKey || e.ctrlKey || e.metaKey)" @dblclick="renaming = l.id">
        <span class="ic"><Icon :name="ICON[l.type] || 'shapes'" :size="15" /></span>
        <input v-if="renaming === l.id" class="input sm grow" :value="l.name" autofocus @blur="finishRename(l.id, $event)" @keyup.enter="($event.target as HTMLInputElement).blur()" @click.stop />
        <span v-else class="nm grow">{{ l.name }}</span>
        <button class="b" :title="l.locked ? 'Buka kunci' : 'Kunci'" @click.stop="editor.toggleLock(editor.byId(l.id) || undefined)"><Icon :name="l.locked ? 'lock' : 'unlock'" :size="14" /></button>
        <button class="b" :title="l.visible ? 'Sembunyikan' : 'Tampilkan'" @click.stop="editor.toggleVisible(l.id)"><Icon :name="l.visible ? 'eye' : 'eyeoff'" :size="14" /></button>
      </li>
    </ul>
    <p class="muted" style="font-size: 12px">Seret untuk mengubah urutan. Shift+klik untuk memilih banyak. Klik dua kali untuk ganti nama.</p>
  </div>
</template>

<style scoped>
.acts { margin-bottom: 10px; gap: 4px; }
.list { list-style: none; padding: 0; margin: 0; display: flex; flex-direction: column; gap: 4px; }
.ly { display: flex; align-items: center; gap: 8px; padding: 6px 8px; border-radius: 8px; border: 1px solid var(--line); cursor: grab; background: #fff; }
.ly:hover { border-color: #c9cdd6; }
.ly.sel { border-color: var(--accent); background: var(--accent-soft); }
.ly.over { border-top: 3px solid var(--accent); }
.ly.hidden .nm { opacity: .45; }
.ic { width: 26px; height: 26px; border-radius: 6px; background: var(--panel-2); display: grid; place-items: center; flex: 0 0 26px; }
.nm { font-size: 13px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.b { border: 0; background: none; cursor: pointer; color: var(--muted); padding: 4px; border-radius: 6px; display: grid; }
.b:hover { color: var(--ink); background: var(--panel-2); }
</style>
