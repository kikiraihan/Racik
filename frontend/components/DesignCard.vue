<script setup lang="ts">
// Kartu desain milik user: thumbnail, judul, waktu ubah, dan aksi (bintang, duplikat, hapus).
import type { DesignSummary } from '~/composables/useApi'

defineProps<{ d: DesignSummary }>()
defineEmits<{ star: []; duplicate: []; remove: [] }>()
</script>

<template>
  <div class="dc">
    <NuxtLink :to="`/editor/${d.id}`" class="dthumb">
      <img v-if="d.thumbnail" :src="d.thumbnail" alt="" />
      <Icon v-else name="image" :size="22" :stroke="1.6" class="muted" />
    </NuxtLink>
    <div class="acts">
      <button class="btn sm icon" :class="{ starred: d.starred }" :title="d.starred ? 'Hapus bintang' : 'Beri bintang'" @click="$emit('star')"><Icon name="star" :size="15" /></button>
      <button class="btn sm icon" title="Duplikat" @click="$emit('duplicate')"><Icon name="copy" :size="15" /></button>
      <button class="btn sm icon danger" title="Hapus" @click="$emit('remove')"><Icon name="trash" :size="15" /></button>
    </div>
    <NuxtLink :to="`/editor/${d.id}`" class="dtitle">{{ d.title }}</NuxtLink>
    <div class="dmeta"><Icon v-if="d.starred" name="star" :size="12" class="star-on" /> {{ d.width }}×{{ d.height }} · {{ timeAgo(d.updatedAt) }}</div>
  </div>
</template>

<style scoped>
.dc { position: relative; min-width: 0; }
.dthumb { position: relative; display: grid; place-items: center; aspect-ratio: 4 / 3; border-radius: 10px; background: var(--panel-2); border: 1px solid var(--line); overflow: hidden; padding: 14px; transition: border-color .15s; }
.dthumb:hover { border-color: var(--line-2); }
.dthumb img { position: absolute; inset: 14px; width: calc(100% - 28px); height: calc(100% - 28px); object-fit: contain; filter: drop-shadow(0 1px 2px rgba(0, 0, 0, .12)); }
.acts { position: absolute; top: 8px; right: 8px; display: flex; gap: 4px; opacity: 0; transition: opacity .15s; }
.dc:hover .acts, .acts:focus-within { opacity: 1; }
.acts .btn { background: var(--panel); }
.acts .starred { opacity: 1; color: #d97706; }
.dtitle { display: block; font-weight: 500; margin-top: 8px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.dmeta { font-size: 12px; color: var(--muted); display: flex; align-items: center; gap: 4px; margin-top: 2px; }
.star-on { color: #d97706; fill: #d97706; }
@media (hover: none) { .acts { opacity: 1; } }
</style>
