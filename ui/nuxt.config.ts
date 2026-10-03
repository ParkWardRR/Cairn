import tailwindcss from '@tailwindcss/vite'

export default defineNuxtConfig({
  compatibilityDate: '2025-07-15',
  devtools: { enabled: true },

  modules: ['@pinia/nuxt'],

  css: ['~/assets/css/main.css'],

  vite: {
    plugins: [tailwindcss()],
  },

  app: {
    head: {
      title: 'Cairn',
      meta: [
        { name: 'description', content: 'Vehicle trip journal & telemetry dashboard' },
        { name: 'theme-color', content: '#0f1117' },
      ],
      link: [
        { rel: 'preconnect', href: 'https://fonts.googleapis.com' },
        { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: '' },
        {
          rel: 'stylesheet',
          href: 'https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&family=JetBrains+Mono:wght@400;500;600&display=swap',
        },
      ],
    },
  },

  runtimeConfig: {
    tsdbUrl: 'http://127.0.0.1:8480',
    appleMapTeamId: '',
    appleMapKeyId: '',
    appleMapPrivateKeyPath: '',
    public: {
      mapProvider: 'apple',
      pmtilesUrl: '/tiles/region.pmtiles',
    },
  },

  routeRules: {
    '/_nuxt/**': { headers: { 'cache-control': 'public, max-age=31536000, immutable' } },
  },
})
