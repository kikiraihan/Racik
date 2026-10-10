<script setup lang="ts">
const { toasts } = useToast()
</script>

<template>
  <div class="toasts">
    <TransitionGroup name="t">
      <div v-for="t in toasts" :key="t.id" class="toast" :class="t.kind">{{ t.text }}</div>
    </TransitionGroup>
  </div>
</template>

<style scoped>
.toasts { position: fixed; bottom: 20px; left: 50%; transform: translateX(-50%); z-index: 200; display: flex; flex-direction: column; gap: 8px; align-items: center; pointer-events: none; }
.toast { background: #1d2129; color: #fff; padding: 10px 16px; border-radius: 10px; box-shadow: var(--shadow-lg); font-weight: 500; max-width: 90vw; }
.toast.error { background: var(--danger); }
.toast.success { background: var(--ok); }
.t-enter-active, .t-leave-active { transition: all .2s; }
.t-enter-from, .t-leave-to { opacity: 0; transform: translateY(8px); }
</style>
