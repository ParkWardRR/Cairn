export const useUiStore = defineStore('ui', () => {
  const theme = ref<'dark' | 'light' | 'auto'>('auto')
  const sidebarCollapsed = ref(false)
  const mapProvider = ref<'apple' | 'local'>('apple')

  const resolvedTheme = computed(() => {
    if (theme.value !== 'auto') return theme.value
    if (import.meta.client) {
      return window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark'
    }
    return 'dark'
  })

  function toggleTheme() {
    const order: Array<'dark' | 'light' | 'auto'> = ['dark', 'light', 'auto']
    const idx = order.indexOf(theme.value)
    theme.value = order[(idx + 1) % order.length]
  }

  function toggleSidebar() {
    sidebarCollapsed.value = !sidebarCollapsed.value
  }

  function setMapProvider(provider: 'apple' | 'local') {
    mapProvider.value = provider
  }

  return { theme, resolvedTheme, sidebarCollapsed, mapProvider, toggleTheme, toggleSidebar, setMapProvider }
}, { persist: import.meta.client })
