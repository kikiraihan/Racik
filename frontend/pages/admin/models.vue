<script setup lang="ts">
// Katalog model: model AI yang tersedia di sistem (OpenRouter, kenari, dan model env provider
// lain). Pilihan model per fitur ada di halaman terpisah, Konfigurasi AI.
definePageMeta({ layout: 'admin', title: 'Katalog model' })
</script>

<template>
  <AdminModelCatalog />
</template>
