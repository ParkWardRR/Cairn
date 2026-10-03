export const useDashboardStore = defineStore('dashboard', () => {
  const stats = ref<any>(null)
  const recentTrips = ref<any[]>([])
  const device = ref<any>(null)
  const loading = ref(false)

  async function fetchDashboard() {
    loading.value = true
    try {
      const [s, r, d] = await Promise.all([
        $fetch('/api/dashboard/stats'),
        $fetch<any[]>('/api/dashboard/recent'),
        $fetch('/api/dashboard/device'),
      ])
      stats.value = s
      recentTrips.value = r
      device.value = d
    } finally {
      loading.value = false
    }
  }

  return { stats, recentTrips, device, loading, fetchDashboard }
})
