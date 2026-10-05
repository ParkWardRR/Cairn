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
      htmlAttrs: { lang: 'en' },
      meta: [
        { name: 'description', content: 'Vehicle trip journal & telemetry dashboard' },
        { name: 'theme-color', content: '#0f1117' },
        { name: 'apple-mobile-web-app-capable', content: 'yes' },
        { name: 'apple-mobile-web-app-status-bar-style', content: 'black-translucent' },
      ],
      link: [
        { rel: 'icon', type: 'image/svg+xml', href: '/favicon.svg' },
        { rel: 'icon', type: 'image/x-icon', href: '/favicon.ico' },
        { rel: 'apple-touch-icon', href: '/apple-touch-icon.svg' },
        { rel: 'manifest', href: '/site.webmanifest' },
        { rel: 'preconnect', href: 'https://fonts.googleapis.com' },
        { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: '' },
        {
          rel: 'stylesheet',
          href: 'https://fonts.googleapis.com/css2?family=Inter:opsz,wght@14..32,400;14..32,500;14..32,600;14..32,700&family=JetBrains+Mono:wght@400;500;600&display=swap',
        },
      ],
    },
    pageTransition: { name: 'page', mode: 'out-in' },
  },

  runtimeConfig: {
    tsdbUrl: 'http://127.0.0.1:8480',
    // Loopback cairn-server local API, for vehicle display names. Empty = ids only.
    cairnLocalUrl: '',
    // Place naming. Coordinates are sent to Overpass (and Geoapify, if a key is
    // set) to find names; results are cached on disk and never re-fetched.
    // Set NUXT_PLACES_EXTERNAL=false to keep all coordinates local.
    placesExternal: 'true',
    placesDataDir: '.data/places',
    overpassUrl: 'https://overpass-api.de/api/interpreter,https://overpass.private.coffee/api/interpreter',
    geoapifyKey: '',
    public: {
      cartoKey: '',
    },
  },

  routeRules: {
    '/_nuxt/**': { headers: { 'cache-control': 'public, max-age=31536000, immutable' } },
  },
})
