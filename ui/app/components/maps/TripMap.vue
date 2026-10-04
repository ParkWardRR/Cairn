<script setup lang="ts">
const props = defineProps<{
  coordinates: [number, number, number][]
  speeds: number[]
  headings?: number[]
  accuracies?: number[]
  sats?: number[]
  timestamps?: string[]
  firstObdMs?: number | null
  firstFixMs?: number | null
  prevEnd?: { lat: number; lon: number } | null
  highlightPos?: { lat: number; lon: number } | null
  stops?: Array<{ lat: number; lon: number; start_at: string | null; start_offset_s: number; duration_s: number; category: 'quick' | 'medium' | 'long'; inferred: boolean }>
}>()

const mapEl = ref<HTMLDivElement>()
const ready = ref(false)
const mapError = ref<string | null>(null)
const estStartEnabled = ref(true)
const hasEstStart = ref(false)
let map: any = null
let highlightMarker: any = null
let estStartLayers: any[] = []

function setEstStartVisible(visible: boolean) {
  if (!map) return
  for (const layer of estStartLayers) {
    if (visible) layer.addTo(map)
    else map.removeLayer(layer)
  }
}

function toggleEstStart() {
  estStartEnabled.value = !estStartEnabled.value
  setEstStartVisible(estStartEnabled.value)
}

function onMapMouseEnter() {
  if (estStartEnabled.value && estStartLayers.length) setEstStartVisible(false)
}

function onMapMouseLeave() {
  if (estStartEnabled.value && estStartLayers.length) setEstStartVisible(true)
}

watch(() => props.highlightPos, async (pos) => {
  if (!map || !ready.value) return
  const L = await import('leaflet')

  if (highlightMarker) {
    map.removeLayer(highlightMarker)
    highlightMarker = null
  }

  if (pos) {
    const icon = L.divIcon({
      className: '',
      html: `<div style="width:16px;height:16px;border-radius:50%;background:#fff;border:3px solid #3b82f6;box-shadow:0 0 12px rgba(59,130,246,.6);animation:pulse-highlight 1s infinite"></div>`,
      iconSize: [16, 16],
      iconAnchor: [8, 8],
    })
    highlightMarker = L.marker([pos.lat, pos.lon], { icon, interactive: false }).addTo(map)
  }
})

function speedToColor(mph: number): string {
  if (mph < 5) return '#6b7280'
  if (mph < 25) return '#3b82f6'
  if (mph < 50) return '#22c55e'
  if (mph < 75) return '#f59e0b'
  return '#ef4444'
}

const STOP_STYLE = {
  quick: { color: '#94a3b8', size: 12, label: 'Quick stop' },
  medium: { color: '#f59e0b', size: 16, label: 'Medium stop' },
  long: { color: '#ef4444', size: 20, label: 'Long stop' },
} as const

function fmtStopDuration(s: number): string {
  if (s < 60) return `${s}s`
  const m = Math.floor(s / 60)
  if (m < 60) return `${m}m ${s % 60}s`
  return `${Math.floor(m / 60)}h ${m % 60}m`
}

function addStopMarkers(L: any) {
  for (const stop of props.stops ?? []) {
    const st = STOP_STYLE[stop.category]
    const border = stop.inferred ? '2px dashed' : '2px solid'
    const icon = L.divIcon({
      className: '',
      html: `<div style="width:${st.size}px;height:${st.size}px;border-radius:50%;background:${st.color}cc;border:${border} #0f1117;box-shadow:0 2px 6px rgba(0,0,0,.5)"></div>`,
      iconSize: [st.size, st.size],
      iconAnchor: [st.size / 2, st.size / 2],
    })
    const when = stop.start_at
      ? `at ${new Date(stop.start_at).toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' })}`
      : `${Math.round(stop.start_offset_s / 60)} min into the trip`
    const note = stop.inferred ? `<div style="opacity:.7">no GPS fixes while stopped</div>` : ''
    L.marker([stop.lat, stop.lon], { icon })
      .bindTooltip(
        `<div style="font-size:12px;line-height:1.5"><div style="font-weight:600">${st.label} · ${fmtStopDuration(stop.duration_s)}</div><div style="opacity:.8">${when}</div>${note}</div>`,
        { direction: 'top', offset: [0, -8], className: 'trip-tooltip' },
      )
      .addTo(map)
  }
}

function buildRoute(L: any) {
  if (!map) return

  const validPoints: { lat: number; lng: number; speed: number; alt: number; idx: number }[] = []
  props.coordinates.forEach((c, i) => {
    if (c[0] !== 0 && c[1] !== 0) {
      validPoints.push({ lat: c[1], lng: c[0], speed: props.speeds[i] ?? 0, alt: c[2] ?? 0, idx: i })
    }
  })

  if (validPoints.length < 2) return

  const segmentSize = Math.max(1, Math.floor(validPoints.length / 150))

  for (let i = 0; i < validPoints.length - 1; i += segmentSize) {
    const end = Math.min(i + segmentSize + 1, validPoints.length)
    const segPts = validPoints.slice(i, end)
    const avgSpeedMph = (segPts.reduce((a, p) => a + p.speed, 0) / segPts.length) * 2.23694

    const polyline = L.polyline(
      segPts.map((p: any) => [p.lat, p.lng]),
      { color: speedToColor(avgSpeedMph), weight: 5, opacity: 0.95, lineCap: 'round', lineJoin: 'round' },
    ).addTo(map)

    const midPt = segPts[Math.floor(segPts.length / 2)]
    const midIdx = midPt.idx
    const mph = (midPt.speed * 2.23694).toFixed(1)
    const altFt = (midPt.alt * 3.28084).toFixed(0)
    const heading = props.headings?.[midIdx]
    const acc = props.accuracies?.[midIdx]
    const sat = props.sats?.[midIdx]

    let html = `<div style="font-size:12px;line-height:1.5;font-family:Inter,sans-serif">`
    html += `<div style="font-weight:600;font-size:14px;margin-bottom:2px">${mph} <span style="font-weight:400;font-size:11px;opacity:.7">mph</span></div>`
    html += `<div style="opacity:.8">${altFt} ft elevation</div>`
    if (heading != null && heading > 0) html += `<div style="opacity:.8">${heading.toFixed(0)}° heading</div>`
    if (acc != null) html += `<div style="opacity:.8">±${acc.toFixed(1)}m accuracy</div>`
    if (sat != null) html += `<div style="opacity:.8">${sat} satellites</div>`
    html += `</div>`

    polyline.bindTooltip(html, {
      sticky: true,
      direction: 'top',
      offset: [0, -10],
      className: 'trip-tooltip',
    })
  }

  const startIcon = L.divIcon({
    className: '',
    html: `<div style="width:26px;height:26px;border-radius:50%;background:#22c55e;border:3px solid #0f1117;display:flex;align-items:center;justify-content:center;font-size:11px;font-weight:700;color:#0f1117;box-shadow:0 2px 8px rgba(0,0,0,.5)">A</div>`,
    iconSize: [26, 26],
    iconAnchor: [13, 13],
  })

  const endIcon = L.divIcon({
    className: '',
    html: `<div style="width:26px;height:26px;border-radius:50%;background:#ef4444;border:3px solid #0f1117;display:flex;align-items:center;justify-content:center;font-size:11px;font-weight:700;color:#0f1117;box-shadow:0 2px 8px rgba(0,0,0,.5)">B</div>`,
    iconSize: [26, 26],
    iconAnchor: [13, 13],
  })

  const first = validPoints[0]
  const last = validPoints[validPoints.length - 1]

  estStartLayers.forEach(l => map.removeLayer(l))
  estStartLayers = []
  hasEstStart.value = false

  if (props.prevEnd && props.firstObdMs != null && (props.firstFixMs == null || props.firstFixMs > props.firstObdMs)) {
    const prevIcon = L.divIcon({
      className: '',
      html: `<div style="width:22px;height:22px;border-radius:50%;background:transparent;border:2px dashed #f59e0b;display:flex;align-items:center;justify-content:center;font-size:10px;font-weight:600;color:#f59e0b;box-shadow:0 2px 6px rgba(0,0,0,.4)">?</div>`,
      iconSize: [22, 22],
      iconAnchor: [11, 11],
    })
    const marker = L.marker([props.prevEnd.lat, props.prevEnd.lon], { icon: prevIcon })
      .bindTooltip('Estimated start (last known position)', { direction: 'top', offset: [0, -12], className: 'trip-tooltip' })
    const line = L.polyline(
      [[props.prevEnd.lat, props.prevEnd.lon], [first.lat, first.lng]],
      { color: '#f59e0b', weight: 2, opacity: 0.5, dashArray: '6, 8' },
    )
    estStartLayers = [marker, line]
    hasEstStart.value = true
    if (estStartEnabled.value) {
      marker.addTo(map)
      line.addTo(map)
    }
  }

  addStopMarkers(L)

  L.marker([first.lat, first.lng], { icon: startIcon }).addTo(map)
  L.marker([last.lat, last.lng], { icon: endIcon }).addTo(map)

  const allPts = validPoints.map((p: any) => [p.lat, p.lng] as [number, number])
  if (props.prevEnd && props.firstObdMs != null && (props.firstFixMs == null || props.firstFixMs > props.firstObdMs)) {
    allPts.push([props.prevEnd.lat, props.prevEnd.lon])
  }
  const bounds = L.latLngBounds(allPts)
  map.fitBounds(bounds, { padding: [40, 40], maxZoom: 16 })
}

onMounted(async () => {
  if (!mapEl.value) return

  try {
    const L = await import('leaflet')
    await import('leaflet/dist/leaflet.css')

    const { cartoKey } = useRuntimeConfig().public
    const tileUrl = cartoKey
      ? `https://basemaps.cartocdn.com/rastertiles/dark_all/{z}/{x}/{y}{r}.png?key=${cartoKey}`
      : 'https://basemaps.cartocdn.com/rastertiles/dark_all/{z}/{x}/{y}{r}.png'

    map = L.map(mapEl.value, {
      zoomControl: false,
      attributionControl: false,
    }).setView([34.03, -118.47], 12)

    L.control.zoom({ position: 'topright' }).addTo(map)

    L.tileLayer(tileUrl, { maxZoom: 19 }).addTo(map)

    ready.value = true
    buildRoute(L)
  } catch (e: any) {
    mapError.value = e.message ?? 'Map failed to load'
  }
})

watch(() => [props.coordinates, props.speeds, props.stops], async () => {
  if (!ready.value || !map) return
  map.eachLayer((layer: any) => {
    if (!layer._url && !layer._container?.classList?.contains('leaflet-control-container')) {
      map.removeLayer(layer)
    }
  })
  const L = await import('leaflet')
  buildRoute(L)
}, { deep: true })

onUnmounted(() => {
  if (map) {
    map.remove()
    map = null
  }
})
</script>

<template>
  <div class="relative w-full h-full rounded-xl overflow-hidden" style="min-height: 320px" @mouseenter="onMapMouseEnter" @mouseleave="onMapMouseLeave">
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
      v-if="ready && coordinates.length > 0"
      class="absolute bottom-3 left-3 z-[1000] flex items-center gap-2 px-3 py-1.5 rounded-lg text-[10px] font-semibold"
      style="background: rgba(15, 17, 23, 0.85); backdrop-filter: blur(8px); color: var(--color-text-secondary)"
    >
      <span class="flex items-center gap-1"><span class="w-2 h-2 rounded-full" style="background: #6b7280" /> Idle</span>
      <span class="flex items-center gap-1"><span class="w-2 h-2 rounded-full" style="background: #3b82f6" /> City</span>
      <span class="flex items-center gap-1"><span class="w-2 h-2 rounded-full" style="background: #22c55e" /> Cruise</span>
      <span class="flex items-center gap-1"><span class="w-2 h-2 rounded-full" style="background: #f59e0b" /> Fast</span>
      <span class="flex items-center gap-1"><span class="w-2 h-2 rounded-full" style="background: #ef4444" /> WOT</span>
      <span v-if="stops?.length" class="flex items-center gap-1 pl-2" style="border-left: 1px solid var(--color-border)">Stops</span>
      <span v-if="stops?.length" class="flex items-center gap-1"><span class="w-2 h-2 rounded-full" style="background: #94a3b8" /> Quick</span>
      <span v-if="stops?.length" class="flex items-center gap-1"><span class="w-2.5 h-2.5 rounded-full" style="background: #f59e0b" /> Medium</span>
      <span v-if="stops?.length" class="flex items-center gap-1"><span class="w-3 h-3 rounded-full" style="background: #ef4444" /> Long</span>
    </div>

    <button
      v-if="ready && hasEstStart"
      class="absolute bottom-3 right-3 z-[1000] flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg text-[10px] font-semibold transition-opacity"
      :style="{
        background: 'rgba(15, 17, 23, 0.85)',
        backdropFilter: 'blur(8px)',
        color: estStartEnabled ? '#f59e0b' : 'var(--color-text-secondary)',
        opacity: estStartEnabled ? 1 : 0.5,
        border: estStartEnabled ? '1px solid rgba(245, 158, 11, 0.3)' : '1px solid transparent',
      }"
      @click.stop="toggleEstStart"
    >
      <span class="w-2 h-2 rounded-full" :style="{ background: estStartEnabled ? '#f59e0b' : '#6b7280', border: '1px dashed currentColor' }" />
      Est. start
    </button>
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
@keyframes pulse-highlight {
  0%, 100% { transform: scale(1); opacity: 1; }
  50% { transform: scale(1.4); opacity: 0.7; }
}
</style>
