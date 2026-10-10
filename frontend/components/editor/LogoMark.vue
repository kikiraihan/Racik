<script setup lang="ts">
// Logo Racik (berlian bergradien) untuk sidebar & pil sidebar tertutup.
withDefaults(defineProps<{ size?: number }>(), { size: 28 })
</script>

<template>
  <svg :width="size" :height="size" viewBox="0 0 32 32" aria-hidden="true">
    <defs>
      <linearGradient id="racik-lg" x1="0" y1="0" x2="1" y2="1">
        <stop offset="0" stop-color="#a66bff" />
        <stop offset="1" stop-color="#6a1fe0" />
      </linearGradient>
    </defs>
    <path d="M16 2l14 14-14 14L2 16z" fill="url(#racik-lg)" />
    <path d="M16 2v28M2 16h28" stroke="#fff" stroke-opacity=".55" stroke-width="1.6" />
    <path d="M16 9l7 7-7 7-7-7z" fill="#fff" fill-opacity=".28" />
  </svg>
</template>
