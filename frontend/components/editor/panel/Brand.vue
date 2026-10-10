<script setup lang="ts">
const editor = useEditor()
const api = useApi()
const kit = ref<any>(null)
onMounted(async () => (kit.value = await api.get('/api/brand-kit')))
</script>

<template>
  <div v-if="kit" class="col">
    <p class="muted" style="font-size: 12px; margin: 0">Brand kit dipakai otomatis oleh AI. <NuxtLink to="/brand" class="lnk">Atur brand kit →</NuxtLink></p>
    <template v-if="kit.logoUrl">
      <span class="label">Logo</span>
      <img :src="kit.logoUrl" class="logo" alt="logo" @click="editor.addImage(kit.logoUrl, { name: 'Logo' })" />
    </template>
    <span class="label">Warna</span>
    <div class="row" style="flex-wrap: wrap">
      <button v-for="c in kit.colors" :key="c" class="c" :style="{ background: c }" :title="'Terapkan ' + c" @click="editor.state.sel.kind !== 'none' ? editor.set({ fill: c }) : editor.setBackgroundColor(c)" />
      <span v-if="!kit.colors.length" class="muted">Belum ada warna brand.</span>
    </div>
    <span class="label">Font</span>
    <button v-for="f in kit.fonts" :key="f" class="pre" :style="{ fontFamily: f }" @click="editor.state.sel.kind === 'text' ? editor.setFont(f) : editor.addText({ text: kit.name || 'Teks brand', fontSize: 64, fontFamily: f, fontWeight: 'bold' })">{{ f }}</button>
    <span v-if="!kit.fonts.length" class="muted">Belum ada font brand.</span>
  </div>
</template>

<style scoped>
.lnk { color: var(--accent); font-weight: 600; }
.logo { max-width: 120px; border-radius: 8px; border: 1px solid var(--line); cursor: pointer; }
.c { width: 40px; height: 40px; border-radius: 10px; border: 1px solid rgba(0,0,0,.12); cursor: pointer; }
.pre { text-align: left; border: 1px solid var(--line); background: var(--panel-2); border-radius: 8px; padding: 10px 12px; cursor: pointer; font-size: 18px; }
</style>
