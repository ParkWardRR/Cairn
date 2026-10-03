export const useAnalyticsStore = defineStore('analytics', () => {
  const selectedBootId = ref<string | null>(null)
  const telemetry = ref<any[]>([])
  const boostCurve = ref<any[]>([])
  const pulls = ref<any[]>([])
  const trimMap = ref<any[]>([])
  const speedAgreement = ref<any[]>([])
  const driveSummaries = ref<any[]>([])
  const loading = ref(false)

  async function fetchTelemetry(bootId: string) {
    selectedBootId.value = bootId
    loading.value = true
    try {
      telemetry.value = await $fetch<any[]>(`/api/analytics/telemetry?boot_id=${bootId}`)
    } finally {
      loading.value = false
    }
  }

  async function fetchBoostCurve(bootId?: string) {
    const url = bootId ? `/api/analytics/boost-curve?boot_id=${bootId}` : '/api/analytics/boost-curve'
    boostCurve.value = await $fetch<any[]>(url)
  }

  async function fetchPulls(bootId?: string) {
    const url = bootId ? `/api/analytics/pulls?boot_id=${bootId}` : '/api/analytics/pulls'
    pulls.value = await $fetch<any[]>(url)
  }

  async function fetchTrimMap(bootId?: string) {
    const url = bootId ? `/api/analytics/trim-map?boot_id=${bootId}` : '/api/analytics/trim-map'
    trimMap.value = await $fetch<any[]>(url)
  }

  async function fetchSpeedAgreement(bootId?: string) {
    const url = bootId ? `/api/analytics/speed-agreement?boot_id=${bootId}` : '/api/analytics/speed-agreement'
    speedAgreement.value = await $fetch<any[]>(url)
  }

  async function fetchDriveSummaries() {
    driveSummaries.value = await $fetch<any[]>('/api/analytics/drive-summary')
  }

  return {
    selectedBootId, telemetry, boostCurve, pulls, trimMap, speedAgreement, driveSummaries, loading,
    fetchTelemetry, fetchBoostCurve, fetchPulls, fetchTrimMap, fetchSpeedAgreement, fetchDriveSummaries,
  }
})
