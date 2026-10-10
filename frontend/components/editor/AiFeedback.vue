<script setup lang="ts">
// Umpan balik hasil AI: rating + catatan disimpan & dipakai sebagai contoh di prompt berikutnya,
// sehingga hasil desain makin sesuai selera user.
const doc = useDoc()
const api = useApi()
const toast = useToast()
const rating = ref(0)
const note = ref('')
const hidden = ref(false)

async function send(r: number) {
  rating.value = r
  if (r > 0) await submit()
}
async function submit() {
  try {
    await api.post(`/api/ai/generations/${doc.generationId}/rate`, { rating: rating.value, feedback: note.value })
    toast.success('Terima kasih! AI akan belajar dari masukanmu')
    doc.rated = true
  } catch (e: any) {
    toast.error(e.message)
  }
}
</script>

<template>
  <div v-if="!hidden" class="fb card">
    <button class="x" @click="hidden = true"><Icon name="x" :size="14" /></button>
    <b>Gimana hasil desain AI-nya?</b>
    <div class="row" style="margin-top: 8px">
      <button class="btn sm" :class="{ active: rating === 1 }" @click="send(1)"><Icon name="thumbUp" :size="15" /> Suka</button>
      <button class="btn sm" :class="{ active: rating === -1 }" @click="send(-1)"><Icon name="thumbDown" :size="15" /> Kurang</button>
    </div>
    <template v-if="rating === -1">
      <textarea v-model="note" class="textarea" rows="2" placeholder="Apa yang kurang? mis. terlalu ramai, font kurang tegas" style="margin-top: 8px; min-height: 60px" />
      <button class="btn primary sm" style="margin-top: 6px" @click="submit">Kirim masukan</button>
    </template>
  </div>
</template>

<style scoped>
.fb { position: fixed; right: 20px; bottom: 120px; z-index: 70; padding: 14px; width: 270px; }
.x { position: absolute; top: 8px; right: 8px; border: 0; background: none; cursor: pointer; color: var(--muted); }
</style>
