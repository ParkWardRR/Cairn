const loaded = ref(false)
const loading = ref(false)
const error = ref<string | null>(null)

export function useMapKit() {
  async function init() {
    if (loaded.value || loading.value) return
    loading.value = true

    try {
      const { token } = await $fetch<{ token: string }>('/api/map/token')

      await new Promise<void>((resolve, reject) => {
        if ((window as any).mapkit) {
          resolve()
          return
        }
        const script = document.createElement('script')
        script.src = 'https://cdn.apple-mapkit.com/mk/5.x.x/mapkit.core.js'
        script.crossOrigin = 'anonymous'
        script.dataset.libraries = 'map,annotations,overlays'
        script.dataset.callback = '_mapkitInit'
        ;(window as any)._mapkitInit = () => resolve()
        script.onerror = () => reject(new Error('Failed to load MapKit JS'))
        document.head.appendChild(script)
      })

      const mk = (window as any).mapkit
      mk.init({ authorizationCallback: (done: (t: string) => void) => done(token) })
      loaded.value = true
    } catch (e: any) {
      error.value = e.message ?? 'MapKit init failed'
    } finally {
      loading.value = false
    }
  }

  return { loaded, loading, error, init }
}
