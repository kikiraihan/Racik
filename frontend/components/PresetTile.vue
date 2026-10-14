<script setup lang="ts">
// Kartu ukuran desain (ala "You might want to try" / "Create new" di Canva):
// kotak abu-abu berisi bentuk rasio, label di bawahnya.
defineProps<{ label: string; width?: number; height?: number; icon?: string; sub?: string }>()
</script>

<template>
  <button class="tile" type="button">
    <span class="box">
      <span v-if="width && height" class="shape" :class="{ wide: width >= height }" :style="{ aspectRatio: `${width}/${height}` }" />
      <Icon v-else :name="icon || 'plus'" :size="22" :stroke="1.6" />
    </span>
    <span class="lbl">{{ label }}</span>
    <span v-if="sub" class="sub">{{ sub }}</span>
  </button>
</template>

<style scoped>
.tile { display: flex; flex-direction: column; align-items: flex-start; gap: 2px; padding: 0; border: 0; background: none; cursor: pointer; text-align: left; min-width: 0; }
.box { width: 100%; aspect-ratio: 4 / 3; border-radius: 10px; background: var(--panel-2); display: grid; place-items: center; margin-bottom: 8px; color: var(--ink-2); transition: background .15s; }
.tile:hover .box { background: var(--panel-3); }
.shape { height: 58%; max-width: 66%; background: var(--panel); border: 1px solid var(--line-2); border-radius: 4px; }
.shape.wide { height: auto; width: 66%; max-height: 58%; }
.lbl { font-weight: 500; font-size: 13px; color: var(--ink); line-height: 1.3; }
.sub { font-size: 12px; color: var(--muted); }
</style>
