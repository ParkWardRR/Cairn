<script setup lang="ts">
const mapEl = ref<HTMLDivElement>()
const ready = ref(false)
const mapError = ref<string | null>(null)
let map: any = null
let leafletRef: any = null

const { data: heatData } = useFetch<{ points: [number, number, number][] }>('/api/heatmap')

const pointCount = computed(() => heatData.value?.points?.length ?? 0)

onMounted(async () => {
  if (!mapEl.value) return

  try {
    const leafletMod = await import('leaflet')
    await import('leaflet/dist/leaflet.css')

    const L = leafletMod.default ?? leafletMod
    ;(window as any).L = L
    leafletRef = L
    await import('leaflet.heat')

    const { cartoKey } = useRuntimeConfig().public
    const tileUrl = cartoKey
      ? `https://basemaps.cartocdn.com/rastertiles/voyager_labels_under/{z}/{x}/{y}{r}.png?key=${cartoKey}`
      : 'https://basemaps.cartocdn.com/rastertiles/voyager_labels_under/{z}/{x}/{y}{r}.png'

    map = L.map(mapEl.value, {
      zoomControl: false,
      attributionControl: false,
    }).setView([34.03, -118.47], 13)

    L.control.zoom({ position: 'topright' }).addTo(map)

    L.tileLayer(tileUrl, { maxZoom: 19 }).addTo(map)

    const tilePane = map.getPane('tilePane')
    if (tilePane) {
      tilePane.style.filter = 'brightness(0.6) invert(1) contrast(3) hue-rotate(200deg) saturate(0.3) brightness(0.7)'
    }

    ready.value = true
    renderHeat()
  } catch (e: any) {
    mapError.value = e.message ?? 'Map failed to load'
  }
})

function renderHeat() {
  if (!map || !leafletRef || !heatData.value?.points?.length) return

  const L = leafletRef
  const points = heatData.value.points
  const heatFn = L.heatLayer ?? (window as any).L?.heatLayer
  if (!heatFn) return

  const heat = heatFn(points, {
    radius: 12,
    blur: 15,
    maxZoom: 17,
    max: 60,
    minOpacity: 0.35,
    gradient: {
      0.0: '#1e3a8f',
      0.15: '#3b82f6',
      0.35: '#22d3ee',
      0.55: '#22c55e',
      0.75: '#f59e0b',
      0.9: '#ef4444',
      1.0: '#fbbf24',
    },
  })
  heat.addTo(map)

  const lats = points.map((p: number[]) => p[0])
  const lngs = points.map((p: number[]) => p[1])
  const bounds = L.latLngBounds(
    [Math.min(...lats), Math.min(...lngs)],
    [Math.max(...lats), Math.max(...lngs)],
  )
  map.fitBounds(bounds, { padding: [30, 30], maxZoom: 14 })
}

watch(heatData, () => {
  if (ready.value && map) renderHeat()
})

onUnmounted(() => {
  if (map) {
    map.remove()
    map = null
  }
})
</script>

<template>
  <div class="relative w-full rounded-xl overflow-hidden" style="height: 360px" :style="{ border: '1px solid var(--color-border)' }">
    <div ref="mapEl" class="absolute inset-0" />

    <div v-if="!ready && !mapError" class="absolute inset-0 flex items-center justify-center" style="background: var(--color-surface-elevated)">
      <div class="spinner" />
    </div>

    <div v-if="mapError" class="absolute inset-0 flex items-center justify-center" style="background: var(--color-surface-elevated)">
      <div class="text-center">
        <p class="text-[13px] font-medium" style="color: var(--color-danger)">Map unavailable</p>
        <p class="text-[11px] mt-1" style="color: var(--color-text-secondary)">{{ mapError }}</p>
      </div>
    </div>

    <div
      v-if="ready"
      class="absolute top-3 left-3 z-[1000] px-3 py-1.5 rounded-lg text-[11px] font-semibold"
      style="background: rgba(15, 17, 23, 0.85); backdrop-filter: blur(8px); color: var(--color-text-secondary)"
    >
      {{ pointCount.toLocaleString() }} GPS points across all trips
    </div>
  </div>
</template>
