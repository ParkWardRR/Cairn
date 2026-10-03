<script setup lang="ts">
definePageMeta({ layout: 'default' })

interface PlaceTrip {
  boot_id: string
  duration_s: number
  max_speed_kph: number
  start_time: string
  start_lat: number | null
  start_lon: number | null
  end_lat: number | null
  end_lon: number | null
}

const { data, status } = useFetch<{ trips: PlaceTrip[] }>('/api/places')
const trips = computed(() => data.value?.trips ?? [])

const mapEl = ref<HTMLDivElement>()
const mapReady = ref(false)
let map: any = null

const tripsWithGps = computed(() =>
  trips.value.filter(t => (t.start_lat && t.start_lon) || (t.end_lat && t.end_lon))
)

const locationCount = computed(() => {
  let n = 0
  for (const t of tripsWithGps.value) {
    if (t.start_lat && t.start_lon) n++
    if (t.end_lat && t.end_lon) n++
  }
  return n
})

function formatDate(iso: string | null): string {
  if (!iso) return '--'
  const d = new Date(iso)
  if (isNaN(d.getTime()) || d.getFullYear() < 2000) return '--'
  return d.toLocaleDateString('en-US', {
    month: 'short', day: 'numeric',
    hour: 'numeric', minute: '2-digit', hour12: true,
  })
}

function formatDuration(seconds: number | null): string {
  if (!seconds || seconds <= 0) return '--'
  const m = Math.floor(seconds / 60)
  const s = Math.floor(seconds % 60)
  return `${m}m ${s}s`
}

onMounted(async () => {
  if (!mapEl.value) return
  const leafletMod = await import('leaflet')
  await import('leaflet/dist/leaflet.css')
  const L = leafletMod.default ?? leafletMod

  const { cartoKey } = useRuntimeConfig().public
  const tileUrl = cartoKey
    ? `https://basemaps.cartocdn.com/rastertiles/dark_all/{z}/{x}/{y}{r}.png?key=${cartoKey}`
    : 'https://basemaps.cartocdn.com/rastertiles/dark_all/{z}/{x}/{y}{r}.png'

  map = L.map(mapEl.value, { zoomControl: false, attributionControl: false }).setView([34.03, -118.47], 13)
  L.control.zoom({ position: 'topright' }).addTo(map)
  L.tileLayer(tileUrl, { maxZoom: 19 }).addTo(map)
  mapReady.value = true
  renderMarkers(L)
})

function renderMarkers(L: any) {
  if (!map || !tripsWithGps.value.length) return

  const allPts: [number, number][] = []

  for (const t of tripsWithGps.value) {
    const mph = Math.round(t.max_speed_kph / 1.60934)
    const dateStr = formatDate(t.start_time)
    const dur = formatDuration(t.duration_s)

    if (t.start_lat && t.start_lon) {
      addMarker(L, t.start_lat, t.start_lon, '#22c55e', 'Start', t.boot_id, dateStr, dur, mph)
      allPts.push([t.start_lat, t.start_lon])
    }
    if (t.end_lat && t.end_lon) {
      addMarker(L, t.end_lat, t.end_lon, '#ef4444', 'End', t.boot_id, dateStr, dur, mph)
      allPts.push([t.end_lat, t.end_lon])
    }
  }

  if (allPts.length) {
    map.fitBounds(L.latLngBounds(allPts), { padding: [40, 40], maxZoom: 15 })
  }
}

function addMarker(L: any, lat: number, lon: number, color: string, type: string, bootId: string, date: string, dur: string, mph: number) {
  const icon = L.divIcon({
    className: '',
    html: `<div style="width:14px;height:14px;border-radius:50%;background:${color};border:2px solid #0f1117;box-shadow:0 2px 6px rgba(0,0,0,.4)"></div>`,
    iconSize: [14, 14],
    iconAnchor: [7, 7],
  })

  const html = `<div style="font-size:12px;line-height:1.5;font-family:Inter,sans-serif">
    <div style="font-weight:600;margin-bottom:2px">${type} · ${date}</div>
    <div style="opacity:.8">${dur} · ${mph} mph peak</div>
    <div style="opacity:.5;font-size:10px;margin-top:2px">${bootId.slice(0, 12)}…</div>
  </div>`

  L.marker([lat, lon], { icon })
    .bindTooltip(html, { direction: 'top', offset: [0, -8], className: 'trip-tooltip' })
    .on('click', () => navigateTo(`/trips/${bootId}`))
    .addTo(map)
}

watch(tripsWithGps, async () => {
  if (!mapReady.value || !map) return
  map.eachLayer((layer: any) => {
    if (!layer._url && !layer._container?.classList?.contains('leaflet-control-container')) {
      map.removeLayer(layer)
    }
  })
  const L = await import('leaflet')
  renderMarkers(L.default ?? L)
}, { deep: true })

onUnmounted(() => {
  if (map) { map.remove(); map = null }
})
</script>

<template>
  <div>
    <LayoutPageHeader title="Places" subtitle="Trip start and end locations">
      <template #actions>
        <span v-if="locationCount" class="text-xs font-medium px-2.5 py-1 rounded-full" style="background: var(--color-accent-soft); color: var(--color-accent)">
          {{ locationCount }} locations
        </span>
      </template>
    </LayoutPageHeader>

    <!-- Map -->
    <div class="relative w-full rounded-xl overflow-hidden mb-6" style="height: 480px" :style="{ border: '1px solid var(--color-border)' }">
      <div ref="mapEl" class="absolute inset-0" />

      <div v-if="!mapReady" class="absolute inset-0 flex items-center justify-center" style="background: var(--color-surface-elevated)">
        <div class="spinner" />
      </div>

      <div
        v-if="mapReady"
        class="absolute top-3 left-3 z-[1000] flex items-center gap-4 px-3 py-1.5 rounded-lg text-[10px] font-semibold"
        style="background: rgba(15, 17, 23, 0.85); backdrop-filter: blur(8px); color: var(--color-text-secondary)"
      >
        <span class="flex items-center gap-1.5"><span class="w-2.5 h-2.5 rounded-full" style="background: #22c55e" /> Start</span>
        <span class="flex items-center gap-1.5"><span class="w-2.5 h-2.5 rounded-full" style="background: #ef4444" /> End</span>
      </div>
    </div>

    <!-- Trip list with locations -->
    <div v-if="status === 'pending'" class="space-y-2">
      <div v-for="i in 4" :key="i" class="skeleton h-16" />
    </div>

    <div v-else-if="tripsWithGps.length === 0" class="rounded-xl p-6" :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }">
      <p class="text-[13px]" style="color: var(--color-text-secondary)">No trips with GPS data yet.</p>
    </div>

    <div v-else>
      <h2 class="text-[11px] font-bold uppercase tracking-wider mb-3" style="color: var(--color-text-secondary)">Trips by Location</h2>
      <div class="space-y-2">
        <div
          v-for="t in tripsWithGps"
          :key="t.boot_id"
          class="rounded-xl p-4 cursor-pointer transition-colors hover:bg-[var(--color-surface-elevated)]"
          :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }"
          @click="navigateTo(`/trips/${t.boot_id}`)"
        >
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-3">
              <div class="flex items-center gap-1.5">
                <span v-if="t.start_lat && t.start_lon" class="w-2 h-2 rounded-full" style="background: #22c55e" />
                <span v-if="t.end_lat && t.end_lon" class="w-2 h-2 rounded-full" style="background: #ef4444" />
              </div>
              <div>
                <p class="text-[13px] font-medium">{{ formatDate(t.start_time) }}</p>
                <p class="text-[11px] font-mono mt-0.5" style="color: var(--color-text-secondary)">
                  {{ formatDuration(t.duration_s) }} · {{ Math.round(t.max_speed_kph / 1.60934) }} mph peak
                </p>
              </div>
            </div>
            <svg class="w-4 h-4 opacity-30" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7" />
            </svg>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style>
.trip-tooltip {
  background: rgba(15, 17, 23, 0.92) !important;
  backdrop-filter: blur(8px);
  border: 1px solid rgba(38, 43, 61, 0.8) !important;
  border-radius: 10px !important;
  color: #e8eaf0 !important;
  padding: 8px 12px !important;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.4) !important;
}
.trip-tooltip::before {
  border-top-color: rgba(15, 17, 23, 0.92) !important;
}
</style>
