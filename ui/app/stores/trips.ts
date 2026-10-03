export const useTripsStore = defineStore('trips', () => {
  const trips = ref<any[]>([])
  const currentTrip = ref<any>(null)
  const loading = ref(false)

  async function fetchTrips(params?: { limit?: number; offset?: number }) {
    loading.value = true
    try {
      const query = new URLSearchParams()
      if (params?.limit) query.set('limit', String(params.limit))
      if (params?.offset) query.set('offset', String(params.offset))
      const data = await $fetch<any[]>(`/api/trips?${query}`)
      trips.value = data
    } finally {
      loading.value = false
    }
  }

  async function fetchTrip(bootId: string) {
    loading.value = true
    try {
      currentTrip.value = await $fetch(`/api/trips/${bootId}`)
    } finally {
      loading.value = false
    }
  }

  return { trips, currentTrip, loading, fetchTrips, fetchTrip }
})
