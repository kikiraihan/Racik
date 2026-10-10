<script setup lang="ts">
// Ikon garis sederhana (gaya lucide), digambar inline agar tanpa dependency tambahan.
import { ICON_PATHS as P } from '~/lib/icons'

const props = withDefaults(defineProps<{ name: string; size?: number; stroke?: number }>(), { size: 18, stroke: 2 })

const d = computed(() => P[props.name] || P.sparkles)
</script>

<template>
  <svg :width="size" :height="size" viewBox="0 0 24 24" fill="none" stroke="currentColor" :stroke-width="stroke"
    stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <path :d="d" />
  </svg>
</template>
