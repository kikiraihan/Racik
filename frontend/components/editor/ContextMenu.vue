<script setup lang="ts">
const props = defineProps<{ x: number; y: number }>()
const emit = defineEmits<{ close: [] }>()
const editor = useEditor()
const s = computed(() => editor.state.sel)
const style = computed(() => ({ left: Math.min(props.x, window.innerWidth - 230) + 'px', top: Math.min(props.y, window.innerHeight - 380) + 'px' }))
function run(fn: () => any) {
  fn()
  emit('close')
}
</script>

<template>
  <div class="popover menu cm" :style="style" @mousedown.stop @contextmenu.prevent>
    <template v-if="s.kind !== 'none'">
      <button class="menu-item" @click="run(() => editor.copy())"><Icon name="copy" :size="15" /> Salin <span class="kbd">Ctrl+C</span></button>
      <button class="menu-item" @click="run(() => editor.paste())"><Icon name="file" :size="15" /> Tempel <span class="kbd">Ctrl+V</span></button>
      <button class="menu-item" @click="run(() => editor.duplicate())"><Icon name="copy" :size="15" /> Duplikat <span class="kbd">Ctrl+D</span></button>
      <button class="menu-item danger" @click="run(() => editor.remove())"><Icon name="trash" :size="15" /> Hapus <span class="kbd">Del</span></button>
      <div class="sep" style="margin: 4px 0" />
      <button class="menu-item" @click="run(() => editor.arrange('front'))"><Icon name="up" :size="15" /> Bawa ke paling depan</button>
      <button class="menu-item" @click="run(() => editor.arrange('forward'))"><Icon name="up" :size="15" /> Maju satu</button>
      <button class="menu-item" @click="run(() => editor.arrange('backward'))"><Icon name="down" :size="15" /> Mundur satu</button>
      <button class="menu-item" @click="run(() => editor.arrange('back'))"><Icon name="down" :size="15" /> Kirim ke paling belakang</button>
      <div class="sep" style="margin: 4px 0" />
      <button v-if="s.kind === 'multi'" class="menu-item" @click="run(() => editor.group())"><Icon name="group" :size="15" /> Grup <span class="kbd">Ctrl+G</span></button>
      <button v-if="s.kind === 'group'" class="menu-item" @click="run(() => editor.ungroup())"><Icon name="ungroup" :size="15" /> Pisahkan grup</button>
      <button v-if="s.kind === 'image'" class="menu-item" @click="run(() => editor.activeToBackground())"><Icon name="image" :size="15" /> Jadikan latar</button>
      <button class="menu-item" @click="run(() => editor.toggleLock())"><Icon :name="s.locked ? 'unlock' : 'lock'" :size="15" /> {{ s.locked ? 'Buka kunci' : 'Kunci' }}</button>
    </template>
    <template v-else>
      <button class="menu-item" @click="run(() => editor.paste())"><Icon name="file" :size="15" /> Tempel</button>
      <button class="menu-item" @click="run(() => editor.selectAll())"><Icon name="grid" :size="15" /> Pilih semua</button>
      <button class="menu-item" @click="run(() => editor.addPage())"><Icon name="plus" :size="15" /> Tambah halaman</button>
      <button class="menu-item" @click="run(() => editor.duplicatePage())"><Icon name="copy" :size="15" /> Duplikat halaman</button>
    </template>
  </div>
</template>

<style scoped>
.cm { position: fixed; z-index: 150; }
</style>
