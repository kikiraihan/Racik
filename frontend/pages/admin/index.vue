<script setup lang="ts">
// Ringkasan panel admin. Link lama /admin?tab=… diarahkan ke halaman menu yang baru.
definePageMeta({
  layout: 'admin',
  title: 'Ringkasan',
  middleware: (to) => {
    const legacy: Record<string, string> = { users: '/admin/users', ai: '/admin/ai', pricing: '/admin/pricing', logs: '/admin/logs' }
    const tab = to.query.tab as string | undefined
    if (tab && legacy[tab]) return navigateTo(legacy[tab], { replace: true })
  },
})
</script>

<template>
  <AdminOverview />
</template>
