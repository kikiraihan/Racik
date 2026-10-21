<script setup lang="ts">
// Logo Racik untuk sidebar & pil sidebar tertutup — kotak hitam berhuruf "R", sama dengan logo di halaman awal.
withDefaults(defineProps<{ size?: number }>(), { size: 28 })
</script>

<template>
  <span class="mark" :style="{ width: `${size}px`, height: `${size}px`, fontSize: `${Math.round(size * .54)}px`, borderRadius: `${Math.round(size * .27)}px` }" aria-hidden="true">R</span>
</template>

<style scoped>
.mark { background: var(--ink); color: #fff; display: grid; place-items: center; font-weight: 700; flex: 0 0 auto; }
</style>
