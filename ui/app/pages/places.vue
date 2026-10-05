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

interface PlaceLabel {
  status: 'resolved' | 'pending' | 'none'
  name: string | null
  category: string | null
  address: string | null
  confidence: number
  sources: string[]
}

interface PlaceItem {
  id: number
  label: PlaceLabel | null
  lat: number
  lon: number
  stops: number
  stop_seconds: number
  quick: number
  medium: number
  long: number
  longest_s: number
  longest_category: StopCategory | null
  arrivals: number
  departures: number
  trips: string[]
  last_at: string | null
}

const { data, status, refresh } = useFetch<{ trips: PlaceTrip[]; places: PlaceItem[]; pending: boolean; attribution: string[] }>('/api/places')

// Names are looked up in the background the first time; ask again until done.
let polls = 0
let pollTimer: ReturnType<typeof setTimeout> | null = null
watch(() => data.value?.pending, (pending) => {
  if (pending && polls < 24) {
    pollTimer = setTimeout(() => { polls++; refresh() }, 5000)
  }
}, { immediate: true })
onUnmounted(() => { if (pollTimer) clearTimeout(pollTimer) })
const trips = computed(() => data.value?.trips ?? [])
const places = computed(() => data.value?.places ?? [])

const route = useRoute()
const selectedId = ref<number | null>(null)

const mapEl = ref<HTMLDivElement>()
const mapReady = ref(false)
let map: any = null
let leaflet: any = null
let markers = new Map<number, any>()

const tripsWithGps = computed(() =>
  trips.value.filter(t => (t.start_lat && t.start_lon) || (t.end_lat && t.end_lon))
)

const stopCount = computed(() => places.value.reduce((n, p) => n + p.stops, 0))

function placeColor(p: PlaceItem): string {
  return p.longest_category ? STOP_COLOR[p.longest_category] : ENDPOINT_COLOR
}

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

function formatDwell(seconds: number): string {
  if (seconds < 60) return `${Math.round(seconds)}s`
  const m = Math.floor(seconds / 60)
  if (m < 60) return `${m}m`
  return `${Math.floor(m / 60)}h ${m % 60}m`
}

function placeSummary(p: PlaceItem): string {
  return p.stops > 0 ? `${p.stops} stop${p.stops === 1 ? '' : 's'} · ${formatDwell(p.stop_seconds)} total` : 'Trip start / end'
}

function placeTitle(p: PlaceItem): string {
  return p.label?.name ?? placeSummary(p)
}

function escapeHtml(s: string): string {
  return s.replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c] as string))
}

function select(id: number | null, fly = true) {
  selectedId.value = id
  const p = places.value.find(x => x.id === id)
  if (p && fly && map) map.flyTo([p.lat, p.lon], Math.max(map.getZoom(), 16), { duration: 0.6 })
  renderMarkers()
  if (id != null) {
    nextTick(() => document.getElementById(`place-${id}`)?.scrollIntoView({ block: 'nearest', behavior: 'smooth' }))
  }
}

// A trip's stop list links here with ?lat=&lon=; open the place it belongs to.
function selectFromQuery() {
  const lat = parseFloat(route.query.lat as string)
  const lon = parseFloat(route.query.lon as string)
  if (!Number.isFinite(lat) || !Number.isFinite(lon) || !places.value.length) return false
  let best: PlaceItem | null = null
  let bestD = Infinity
  for (const p of places.value) {
    const d = Math.hypot((p.lat - lat) * 110540, (p.lon - lon) * 111320 * Math.cos(lat * Math.PI / 180))
    if (d < bestD) { best = p; bestD = d }
  }
  if (best && bestD < 300) { select(best.id); return true }
  return false
}

onMounted(async () => {
  if (!mapEl.value) return
  const leafletMod = await import('leaflet')
  await import('leaflet/dist/leaflet.css')
  const L = leafletMod.default ?? leafletMod
  leaflet = L

  const { cartoKey } = useRuntimeConfig().public
  const tileUrl = cartoKey
    ? `https://basemaps.cartocdn.com/rastertiles/dark_all/{z}/{x}/{y}{r}.png?key=${cartoKey}`
    : 'https://basemaps.cartocdn.com/rastertiles/dark_all/{z}/{x}/{y}{r}.png'

  map = L.map(mapEl.value, { zoomControl: false, attributionControl: false }).setView([34.03, -118.47], 13)
  L.control.zoom({ position: 'topright' }).addTo(map)
  L.tileLayer(tileUrl, { maxZoom: 19 }).addTo(map)
  mapReady.value = true
  renderMarkers(true)
  selectFromQuery()
})

function renderMarkers(fit = false) {
  const L = leaflet
  if (!map || !L) return
  markers.forEach(m => map.removeLayer(m))
  markers = new Map()
  if (!places.value.length) return

  const allPts: [number, number][] = []
  for (const p of places.value) {
    const selected = p.id === selectedId.value
    const size = p.longest_category ? STOP_SIZE[p.longest_category] : 14
    const color = placeColor(p)
    const ring = selected ? '3px solid #fff' : '2px solid #0f1117'
    const icon = L.divIcon({
      className: '',
      html: p.longest_category
        ? stopBadgeHtml(p.longest_category, { selected })
        : `<div style="width:${size}px;height:${size}px;border-radius:50%;background:${color};border:${ring};box-shadow:0 2px 6px rgba(0,0,0,.5)"></div>`,
      iconSize: [size, size],
      iconAnchor: [size / 2, size / 2],
    })

    const parts: string[] = []
    if (p.long) parts.push(`${p.long} long`)
    if (p.medium) parts.push(`${p.medium} medium`)
    if (p.quick) parts.push(`${p.quick} quick`)
    const html = `<div style="font-size:12px;line-height:1.5;font-family:Inter,sans-serif">
      <div style="font-weight:600;margin-bottom:2px">${escapeHtml(placeTitle(p))}</div>
      ${p.label?.name ? `<div style="opacity:.8">${placeSummary(p)}</div>` : ''}
      ${p.label?.category && p.label.category !== 'address' && p.label.category !== 'street' ? `<div style="opacity:.6;font-size:11px">${escapeHtml(p.label.category)}</div>` : ''}
      ${parts.length ? `<div style="opacity:.8">${parts.join(' · ')}</div>` : ''}
      <div style="opacity:.6;font-size:11px">${p.arrivals} arrival${p.arrivals === 1 ? '' : 's'} · ${p.departures} departure${p.departures === 1 ? '' : 's'}</div>
    </div>`

    const marker = L.marker([p.lat, p.lon], { icon, zIndexOffset: selected ? 1000 : 0 })
      .bindTooltip(html, { direction: 'top', offset: [0, -size / 2], className: 'trip-tooltip' })
      .on('click', () => select(p.id, false))
      .addTo(map)
    markers.set(p.id, marker)
    allPts.push([p.lat, p.lon])
  }

  if (fit && allPts.length) {
    map.fitBounds(L.latLngBounds(allPts), { padding: [40, 40], maxZoom: 15 })
  }
}

let fitted = false
watch(places, () => {
  if (!mapReady.value) return
  // Refit only the first time, so a background refresh does not reset the view.
  renderMarkers(!fitted)
  if (places.value.length) fitted = true
  if (!fitted || !selectedId.value) selectFromQuery()
}, { deep: true })

onUnmounted(() => {
  if (map) { map.remove(); map = null }
})
</script>

<template>
  <div>
    <LayoutPageHeader title="Places" subtitle="Where you stopped, and where trips started and ended">
      <template #actions>
        <span v-if="places.length" class="text-xs font-medium px-2.5 py-1 rounded-full" style="background: var(--color-accent-soft); color: var(--color-accent)">
          {{ places.length }} places · {{ stopCount }} stops
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
        <span class="flex items-center gap-1.5"><span class="w-2.5 h-2.5 rounded-[3px]" style="background: #38bdf8" /> Quick</span>
        <span class="flex items-center gap-1.5"><span class="w-2.5 h-2.5 rounded-[3px]" style="background: #2dd4bf" /> Medium</span>
        <span class="flex items-center gap-1.5"><span class="w-2.5 h-2.5 rounded-[3px]" style="background: #a78bfa" /> Long</span>
        <span class="flex items-center gap-1.5"><span class="w-2.5 h-2.5 rounded-full" style="background: #64748b" /> Trip start / end</span>
        <span style="opacity: .6">colour = longest stop</span>
      </div>
    </div>

    <div v-if="status === 'pending'" class="space-y-2">
      <div v-for="i in 4" :key="i" class="skeleton h-16" />
    </div>

    <div v-else-if="places.length === 0" class="rounded-xl p-6" :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }">
      <p class="text-[13px]" style="color: var(--color-text-secondary)">No trips with GPS data yet.</p>
    </div>

    <template v-else>
      <!-- Places -->
      <h2 class="text-[11px] font-bold uppercase tracking-wider mb-3" style="color: var(--color-text-secondary)">Places</h2>
      <div class="space-y-2 mb-8">
        <div
          v-for="p in places"
          :id="`place-${p.id}`"
          :key="p.id"
          class="rounded-xl p-4 cursor-pointer transition-colors hover:bg-[var(--color-surface-elevated)]"
          :style="{
            backgroundColor: 'var(--color-surface)',
            border: selectedId === p.id ? `1px solid ${placeColor(p)}` : '1px solid var(--color-border)',
          }"
          @click="select(p.id)"
        >
          <div class="flex items-center gap-3">
            <span class="w-3 h-3 shrink-0" :class="p.longest_category ? 'rounded-[3px]' : 'rounded-full'" :style="{ background: placeColor(p) }" />
            <div class="flex-1 min-w-0">
              <p class="text-[13px] font-medium truncate">
                {{ placeTitle(p) }}
                <span v-if="p.label?.status === 'pending'" class="text-[11px] font-normal" style="color: var(--color-text-secondary)">identifying…</span>
              </p>
              <p v-if="p.label?.name" class="text-[12px] mt-0.5" style="color: var(--color-text-secondary)">
                {{ placeSummary(p) }}<template v-if="p.label.category && p.label.category !== 'address' && p.label.category !== 'street'"> · {{ p.label.category }}</template>
              </p>
              <p class="text-[11px] font-mono mt-0.5" style="color: var(--color-text-secondary)">
                {{ p.lat.toFixed(4) }}, {{ p.lon.toFixed(4) }}
                <template v-if="p.last_at"> · last {{ formatDate(p.last_at) }}</template>
              </p>
            </div>
            <div class="flex items-center gap-1.5 text-[10px] font-semibold shrink-0">
              <span v-if="p.long" class="px-1.5 py-0.5 rounded" style="background: rgba(167,139,250,.15); color: #a78bfa">{{ p.long }} long</span>
              <span v-if="p.medium" class="px-1.5 py-0.5 rounded" style="background: rgba(45,212,191,.15); color: #2dd4bf">{{ p.medium }} medium</span>
              <span v-if="p.quick" class="px-1.5 py-0.5 rounded" style="background: rgba(56,189,248,.15); color: #38bdf8">{{ p.quick }} quick</span>
            </div>
          </div>

          <div v-if="selectedId === p.id" class="mt-3 pt-3 flex flex-wrap items-center gap-x-4 gap-y-1.5 text-[12px]" style="border-top: 1px solid var(--color-border)">
            <span style="color: var(--color-text-secondary)">
              {{ p.arrivals }} arrival{{ p.arrivals === 1 ? '' : 's' }} · {{ p.departures }} departure{{ p.departures === 1 ? '' : 's' }}
              <template v-if="p.longest_category"> · longest {{ STOP_LABEL[p.longest_category].toLowerCase() }} stop {{ formatDwell(p.longest_s) }}</template>
            </span>
            <NuxtLink
              v-for="b in p.trips.slice(0, 6)"
              :key="b"
              :to="`/trips/${b}`"
              class="font-mono underline-offset-2 hover:underline"
              style="color: var(--color-accent)"
              @click.stop
            >
              trip {{ b.slice(0, 8) }}
            </NuxtLink>
            <span v-if="p.trips.length > 6" style="color: var(--color-text-secondary)">+{{ p.trips.length - 6 }} more</span>
          </div>
        </div>
      </div>

      <!-- Trip list with locations -->
      <template v-if="tripsWithGps.length">
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
      </template>
    </template>

    <p v-if="data?.attribution?.length" class="text-[10px] mt-6 px-1" style="color: var(--color-text-secondary)">
      Place names: {{ data.attribution.join(' · ') }}
    </p>
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
