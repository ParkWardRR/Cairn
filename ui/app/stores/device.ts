export const useDeviceStore = defineStore('device', () => {
  const healthHistory = ref<any[]>([])
  const bundles = ref<any[]>([])
  const tsdbStatus = ref<any>(null)
  const loading = ref(false)

  async function fetchHealth(limit = 100) {
    loading.value = true
    try {
      healthHistory.value = await $fetch<any[]>(`/api/device/health?limit=${limit}`)
    } finally {
      loading.value = false
    }
  }

  async function fetchBundles() {
    bundles.value = await $fetch<any[]>('/api/device/bundles')
  }

  async function fetchTsdbStatus() {
    tsdbStatus.value = await $fetch('/api/device/tsdb-status')
  }

  return { healthHistory, bundles, tsdbStatus, loading, fetchHealth, fetchBundles, fetchTsdbStatus }
})
