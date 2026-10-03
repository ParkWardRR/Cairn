<script setup lang="ts">
import { Map as MaplibreMap, NavigationControl, Marker, type GeoJSONSource } from 'maplibre-gl'

const props = defineProps<{
  coordinates: [number, number, number][]
  speeds: number[]
  startLabel?: string
  endLabel?: string
}>()

const mapEl = ref<HTMLDivElement>()
const ready = ref(false)
const mapError = ref<string | null>(null)
let map: MaplibreMap | null = null

function speedToColor(speed: number): string {
  const kph = speed * 3.6
  if (kph < 10) return '#6b7280'
  if (kph < 40) return '#3b82f6'
  if (kph < 80) return '#22c55e'
  if (kph < 120) return '#f59e0b'
  return '#ef4444'
}

function buildSegmentedGeoJSON() {
  const validCoords: [number, number][] = []
  const validSpeeds: number[] = []

  props.coordinates.forEach((c, i) => {
    if (c[0] !== 0 && c[1] !== 0) {
      validCoords.push([c[0], c[1]])
      validSpeeds.push(props.speeds[i] ?? 0)
    }
  })

  if (validCoords.length < 2) return null

  const segmentSize = Math.max(1, Math.floor(validCoords.length / 120))
  const features: GeoJSON.Feature[] = []

  for (let i = 0; i < validCoords.length - 1; i += segmentSize) {
    const end = Math.min(i + segmentSize + 1, validCoords.length)
    const segCoords = validCoords.slice(i, end)
    const avgSpeed = validSpeeds.slice(i, end).reduce((a, b) => a + b, 0) / (end - i)

    features.push({
      type: 'Feature',
      geometry: { type: 'LineString', coordinates: segCoords },
      properties: { color: speedToColor(avgSpeed) },
    })
  }

  return {
    geojson: { type: 'FeatureCollection' as const, features },
    bounds: [
      [Math.min(...validCoords.map(c => c[0])), Math.min(...validCoords.map(c => c[1]))],
      [Math.max(...validCoords.map(c => c[0])), Math.max(...validCoords.map(c => c[1]))],
    ] as [[number, number], [number, number]],
    start: validCoords[0],
    end: validCoords[validCoords.length - 1],
  }
}

function buildRoute() {
  if (!map) return

  const data = buildSegmentedGeoJSON()
  if (!data) return

  if (map.getSource('route')) {
    ;(map.getSource('route') as GeoJSONSource).setData(data.geojson)
  } else {
    map.addSource('route', { type: 'geojson', data: data.geojson })

    map.addLayer({
      id: 'route-line',
      type: 'line',
      source: 'route',
      paint: {
        'line-color': ['get', 'color'],
        'line-width': 4,
        'line-opacity': 0.9,
      },
      layout: {
        'line-cap': 'round',
        'line-join': 'round',
      },
    })
  }

  const existingMarkers = document.querySelectorAll('.trip-marker')
  existingMarkers.forEach(m => m.remove())

  const startEl = document.createElement('div')
  startEl.className = 'trip-marker'
  startEl.innerHTML = `<div style="width:24px;height:24px;border-radius:50%;background:#22c55e;border:3px solid #0f1117;display:flex;align-items:center;justify-content:center;font-size:10px;font-weight:700;color:#0f1117">A</div>`
  new Marker({ element: startEl }).setLngLat(data.start).addTo(map)

  const endEl = document.createElement('div')
  endEl.className = 'trip-marker'
  endEl.innerHTML = `<div style="width:24px;height:24px;border-radius:50%;background:#ef4444;border:3px solid #0f1117;display:flex;align-items:center;justify-content:center;font-size:10px;font-weight:700;color:#0f1117">B</div>`
  new Marker({ element: endEl }).setLngLat(data.end).addTo(map)

  map.fitBounds(data.bounds, { padding: 50, maxZoom: 16, duration: 600 })
}

onMounted(() => {
  if (!mapEl.value) return

  try {
    map = new MaplibreMap({
      container: mapEl.value,
      style: {
        version: 8,
        sources: {
          carto: {
            type: 'raster',
            tiles: ['https://basemaps.cartocdn.com/dark_all/{z}/{x}/{y}@2x.png'],
            tileSize: 256,
            attribution: '&copy; OpenStreetMap &copy; CARTO',
          },
        },
        layers: [{
          id: 'carto-dark',
          type: 'raster',
          source: 'carto',
        }],
      },
      center: [-118.47, 34.03],
      zoom: 12,
      attributionControl: false,
    })

    map.addControl(new NavigationControl({ showCompass: false }), 'top-right')

    const onReady = () => {
      if (ready.value) return
      ready.value = true
      buildRoute()
    }

    map.on('load', onReady)
    map.on('idle', onReady)
  } catch (e: any) {
    mapError.value = e.message ?? 'Map failed to load'
  }
})

watch(() => [props.coordinates, props.speeds], () => {
  if (ready.value && map) buildRoute()
}, { deep: true })

onUnmounted(() => {
  if (map) {
    map.remove()
    map = null
  }
})
</script>

<template>
  <div class="relative w-full h-full rounded-xl overflow-hidden" style="min-height: 320px">
    <div ref="mapEl" class="absolute inset-0" />

    <!-- Loading overlay -->
    <div v-if="!ready && !mapError" class="absolute inset-0 flex items-center justify-center" style="background: var(--color-surface-elevated)">
      <div class="spinner" />
    </div>

    <!-- Error overlay -->
    <div v-if="mapError" class="absolute inset-0 flex items-center justify-center" style="background: var(--color-surface-elevated)">
      <div class="text-center">
        <p class="text-[13px] font-medium" style="color: var(--color-danger)">Map unavailable</p>
        <p class="text-[11px] mt-1" style="color: var(--color-text-secondary)">{{ mapError }}</p>
      </div>
    </div>

    <!-- Speed legend -->
    <div
      v-if="ready && coordinates.length > 0"
      class="absolute bottom-3 left-3 flex items-center gap-2 px-3 py-1.5 rounded-lg text-[10px] font-semibold"
      style="background: rgba(15, 17, 23, 0.85); backdrop-filter: blur(8px); color: var(--color-text-secondary)"
    >
      <span class="flex items-center gap-1"><span class="w-2 h-2 rounded-full" style="background: #6b7280" /> Idle</span>
      <span class="flex items-center gap-1"><span class="w-2 h-2 rounded-full" style="background: #3b82f6" /> City</span>
      <span class="flex items-center gap-1"><span class="w-2 h-2 rounded-full" style="background: #22c55e" /> Cruise</span>
      <span class="flex items-center gap-1"><span class="w-2 h-2 rounded-full" style="background: #f59e0b" /> Fast</span>
      <span class="flex items-center gap-1"><span class="w-2 h-2 rounded-full" style="background: #ef4444" /> WOT</span>
    </div>
  </div>
</template>
