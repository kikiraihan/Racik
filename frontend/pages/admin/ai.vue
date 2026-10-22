<script setup lang="ts">
// Konfigurasi AI: model untuk tiap fitur AI.
definePageMeta({ layout: 'admin', title: 'Konfigurasi AI' })

// Katalog OpenRouter menambah pilihan model → muat ulang dropdown konfigurasi.
const cfg = ref<{ load: () => Promise<void> } | null>(null)
</script>

<template>
  <div>
    <AdminAiConfig ref="cfg" />
    <AdminOpenRouterCatalog style="margin-top: 16px" @changed="cfg?.load()" />
  </div>
</template>
