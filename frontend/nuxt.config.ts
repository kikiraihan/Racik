// Konfigurasi Nuxt untuk Racik.
// Editor berbasis canvas (fabric.js) sehingga app dijalankan sebagai SPA (ssr: false).
const apiTarget = process.env.NUXT_API_TARGET || 'http://localhost:8080'

export default defineNuxtConfig({
  compatibilityDate: '2025-07-01',
  ssr: false,
  devtools: { enabled: false },
  css: ['~/assets/css/main.css', '~/assets/css/admin.css', '~/assets/css/public.css'],
  app: {
    head: {
      title: 'Racik — Desain cepat dengan AI',
      htmlAttrs: { lang: 'id' },
      meta: [
        { name: 'viewport', content: 'width=device-width, initial-scale=1' },
        { name: 'description', content: 'Racik: editor desain mirip Canva dengan AI auto-generate desain, presentasi, dan gambar.' },
      ],
      link: [
        { rel: 'preconnect', href: 'https://fonts.googleapis.com' },
        { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: '' },
        {
          rel: 'stylesheet',
          href: 'https://fonts.googleapis.com/css2?family=Anton&family=Archivo+Black&family=Bebas+Neue&family=Caveat:wght@400;700&family=DM+Serif+Display&family=Dancing+Script:wght@400;700&family=Inter:wght@400;500;600;700;800&family=Lato:ital,wght@0,400;0,700;1,400&family=Lora:ital,wght@0,400;0,700;1,400&family=Merriweather:wght@400;700&family=Montserrat:wght@400;600;700;800&family=Nunito:wght@400;700;800&family=Open+Sans:wght@400;600;700&family=Oswald:wght@400;600;700&family=Pacifico&family=Playfair+Display:ital,wght@0,400;0,700;1,400&family=Poppins:wght@400;600;700;800&family=Raleway:wght@400;600;800&family=Roboto:wght@400;500;700&family=Space+Grotesk:wght@400;600;700&display=swap',
        },
      ],
    },
  },
  runtimeConfig: {
    public: {
      // Kosong = pakai proxy same-origin (/api, /uploads) agar export canvas tidak kena CORS.
      apiBase: '',
    },
  },
  // Proxy ke backend Go, berlaku di dev maupun production (node server).
  nitro: {
    routeRules: {
      '/api/**': { proxy: `${apiTarget}/api/**` },
      '/uploads/**': { proxy: `${apiTarget}/uploads/**` },
    } as Record<string, any>,
  },
  vite: {
    optimizeDeps: { include: ['fabric', 'jspdf', 'pptxgenjs', 'jszip'] },
  },
})
