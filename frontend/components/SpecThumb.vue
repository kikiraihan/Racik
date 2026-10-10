<script setup lang="ts">
// Render preview halaman pertama dari sebuah spec (template) secara lazy.
import { specPreview } from '~/lib/spec'

const props = defineProps<{ spec: any; width: number; height: number }>()
const src = ref('')
const el = ref<HTMLElement>()

onMounted(() => {
  const io = new IntersectionObserver(async (entries) => {
    if (entries.some((e) => e.isIntersecting)) {
      io.disconnect()
      src.value = await specPreview(props.spec, props.width, props.height, 360)
    }
  })
  if (el.value) io.observe(el.value)
})
</script>

<template>
  <div ref="el" class="thumb" :style="{ aspectRatio: `${width} / ${height}` }">
    <img v-if="src" :src="src" alt="" />
    <div v-else class="shimmer fill" />
  </div>
</template>

<style scoped>
.thumb { width: 100%; border-radius: 8px; overflow: hidden; background: #eee; }
.thumb img { width: 100%; height: 100%; object-fit: cover; display: block; }
.fill { width: 100%; height: 100%; }
</style>
