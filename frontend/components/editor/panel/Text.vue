<script setup lang="ts">
const editor = useEditor()

const COMBOS = [
  { h: 'SALE BESAR', s: 'HANYA HARI INI', hf: 'Anton', sf: 'Poppins' },
  { h: 'Elegan', s: 'koleksi musim ini', hf: 'Playfair Display', sf: 'Lato' },
  { h: 'Halo Dunia', s: 'mulai dari sini', hf: 'Pacifico', sf: 'Nunito' },
  { h: 'TECH TALK', s: 'masa depan AI', hf: 'Space Grotesk', sf: 'Inter' },
  { h: 'Grand Opening', s: 'SABTU, 10.00 WIB', hf: 'DM Serif Display', sf: 'Montserrat' },
  { h: 'NEWS', s: 'update terkini', hf: 'Bebas Neue', sf: 'Roboto' },
]
</script>

<template>
  <div class="col">
    <button class="btn primary" @click="editor.addText('body')"><Icon name="text" :size="16" /> Tambahkan kotak teks</button>
    <h4>Gaya teks default</h4>
    <button class="pre h" @click="editor.addText('heading')">Tambahkan judul</button>
    <button class="pre s" @click="editor.addText('subheading')">Tambahkan subjudul</button>
    <button class="pre b" @click="editor.addText('body')">Tambahkan sedikit teks isi</button>
    <h4>Kombinasi font</h4>
    <div class="grid2">
      <button v-for="c in COMBOS" :key="c.h" class="combo" @click="editor.addTextCombo(c.h, c.s, c.hf, c.sf)">
        <span :style="{ fontFamily: c.hf }" class="ch">{{ c.h }}</span>
        <span :style="{ fontFamily: c.sf }" class="cs">{{ c.s }}</span>
      </button>
    </div>
  </div>
</template>

<style scoped>
.pre { text-align: left; border: 1px solid var(--line); background: var(--panel-2); border-radius: 8px; padding: 10px 12px; cursor: pointer; }
.pre:hover { border-color: var(--accent); }
.pre.h { font: 800 24px Montserrat; }
.pre.s { font: 600 18px Poppins; }
.pre.b { font: 400 14px Inter; }
.combo { aspect-ratio: 1.3; border: 1px solid var(--line); border-radius: 10px; background: #fff; cursor: pointer; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 4px; padding: 6px; overflow: hidden; }
.combo:hover { border-color: var(--accent); }
.ch { font-size: 20px; font-weight: 700; line-height: 1.1; text-align: center; }
.cs { font-size: 10px; letter-spacing: .15em; text-transform: uppercase; color: var(--muted); }
</style>
