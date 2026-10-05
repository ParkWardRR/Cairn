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
  kind: string
  address: string | null
  confidence: number
  sources: string[]
  saved_id: number | null
}

interface PlaceItem {
  id: number
  label: PlaceLabel | null
  suggestions: Suggestion[]
  lat: number
  lon: number
  stops: number
  stop_seconds: number
  short: number
  medium: number
  long: number
  longest_s: number
  longest_category: StopCategory | null
  arrivals: number
  departures: number
  trips: string[]
  last_at: string | null
}

const { data, status, refresh } = useFetch<{ trips: PlaceTrip[]; places: PlaceItem[]; saved: SavedPlace[]; hints: { home_place_id: number | null }; storage: Storage; pending: boolean; attribution: string[] }>('/api/places')

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

interface SavedPlace {
  id: number
  name: string
  category: string | null
  kind: string
  lat: number
  lon: number
  radius_m: number
  note: string | null
  source: 'user' | 'learned'
  learned_from: string | null
}

interface Storage {
  saved: number
  learned: number
  backup_at: number | null
  trips_recorded: number
}

interface Suggestion {
  name: string
  category: string | null
  kind: string
  source: string
  dist_m: number
}

const KIND_CHOICES = PICKER_KINDS
const RADII = [50, 100, 150, 250, 400]

// What the user is changing. lat/lon follow the pin if it is dragged.
interface Draft {
  placeId: number
  savedId: number | null
  name: string
  category: string | null
  radius_m: number
  lat: number
  lon: number
}

const draft = ref<Draft | null>(null)
const saving = ref(false)
const saveError = ref<string | null>(null)
let editCircle: any = null

const homeSuggestion = computed(() => {
  const id = data.value?.hints?.home_place_id
  return id == null ? null : places.value.find(p => p.id === id) ?? null
})

function kindOfPlace(p: PlaceItem): string {
  return p.label?.kind ?? 'other'
}

// The kind picked in the editor, from the label it stores.
function draftKind(): string {
  return kindForCategory(draft.value?.category)
}

// Names offered for the spot, those that look like the chosen kind first.
function rankedSuggestions(p: PlaceItem): Suggestion[] {
  return rankByKind(p.suggestions, draftKind())
}

function pickKind(id: string) {
  const d = draft.value
  if (!d) return
  const k = placeKind(id)
  if (draftKind() === id) { d.category = null; return }
  d.category = k.label
  // A kind with an obvious name (Home, Work) fills it in if nothing is there yet.
  if (!d.name.trim() && k.defaultName) d.name = k.defaultName
}

// The engine thinks this spot is home: open the editor already set to Home.
function setAsHome(p: PlaceItem) {
  select(p.id)
  startEdit(p)
  if (draft.value) { draft.value.category = 'Home'; draft.value.name = 'Home' }
}

function savedFor(p: PlaceItem): SavedPlace | null {
  const id = p.label?.saved_id
  return id == null ? null : (data.value?.saved ?? []).find(s => s.id === id) ?? null
}

function startEdit(p: PlaceItem) {
  const s = savedFor(p)
  saveError.value = null
  draft.value = {
    placeId: p.id,
    savedId: s?.id ?? null,
    // Start from what is shown, so fixing a near-miss is a small edit.
    name: s?.name ?? p.label?.name ?? '',
    category: s?.category ?? null,
    radius_m: s?.radius_m ?? 100,
    lat: s?.lat ?? p.lat,
    lon: s?.lon ?? p.lon,
  }
  renderMarkers()
}

function cancelEdit() {
  draft.value = null
  saveError.value = null
  renderMarkers()
}

function pickSuggestion(s: Suggestion) {
  if (!draft.value) return
  draft.value.name = s.name
}

async function saveDraft() {
  const d = draft.value
  if (!d) return
  saving.value = true
  saveError.value = null
  try {
    const body = { name: d.name, category: d.category, radius_m: d.radius_m, lat: d.lat, lon: d.lon }
    if (d.savedId != null) await $fetch(`/api/places/saved/${d.savedId}`, { method: 'PATCH', body })
    else await $fetch('/api/places/saved', { method: 'POST', body })
    draft.value = null
    await refresh()
    selectNear(d.lat, d.lon)
  } catch (e: any) {
    saveError.value = e?.data?.statusMessage ?? e?.statusMessage ?? e?.message ?? 'Could not save'
  } finally {
    saving.value = false
  }
}

async function removeSaved(p: PlaceItem) {
  const s = savedFor(p)
  if (!s) return
  if (!confirm(`Stop using "${s.name}" for this place? Visits go back to their automatic names.`)) return
  try {
    await $fetch(`/api/places/saved/${s.id}`, { method: 'DELETE' })
    draft.value = null
    await refresh()
    selectNear(s.lat, s.lon)
  } catch (e: any) {
    saveError.value = e?.data?.statusMessage ?? 'Could not remove'
  }
}

function selectNear(lat: number, lon: number) {
  let best: PlaceItem | null = null
  let bestD = Infinity
  for (const p of places.value) {
    const d = Math.hypot((p.lat - lat) * 110540, (p.lon - lon) * 111320 * Math.cos(lat * Math.PI / 180))
    if (d < bestD) { best = p; bestD = d }
  }
  if (best) select(best.id, false)
}

function isLearned(p: PlaceItem): boolean {
  return savedFor(p)?.source === 'learned'
}

function learnedTrips(p: PlaceItem): number | null {
  try { return JSON.parse(savedFor(p)?.learned_from ?? 'null')?.trips ?? null } catch { return null }
}

async function confirmLearned(p: PlaceItem) {
  const s = savedFor(p)
  if (!s) return
  await $fetch(`/api/places/saved/${s.id}`, { method: 'PATCH', body: { confirm: true } })
  await refresh()
  selectNear(s.lat, s.lon)
}

const importMsg = ref<string | null>(null)
async function importFile(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  try {
    const r = await $fetch<{ added: number; skipped: number }>('/api/places/saved/import', { method: 'POST', body: JSON.parse(await file.text()) })
    importMsg.value = `Imported ${r.added} place${r.added === 1 ? '' : 's'}, skipped ${r.skipped} already here`
    await refresh()
  } catch (err: any) {
    importMsg.value = err?.data?.statusMessage ?? 'That file is not a Cairn saved-places export'
  } finally {
    (e.target as HTMLInputElement).value = ''
  }
}

function fmtBackup(ms: number | null): string {
  return ms ? new Date(ms).toLocaleString('en-US', { month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' }) : 'not yet'
}

// A spot you keep coming back to is worth naming once.
function isFrequent(p: PlaceItem): boolean {
  return p.trips.length >= 2 && p.label?.saved_id == null
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
    const size = p.longest_category ? STOP_SIZE[p.longest_category] : 24
    const color = placeColor(p)
    const ring = selected ? '3px solid #fff' : '2px solid #0f1117'
    const icon = L.divIcon({
      className: '',
      html: p.longest_category
        ? stopBadgeHtml(p.longest_category, { selected, kind: kindOfPlace(p) })
        : endpointMarkerHtml(kindOfPlace(p), size, color, ring),
      iconSize: [size, size],
      iconAnchor: [size / 2, size / 2],
    })

    const parts: string[] = []
    if (p.long) parts.push(`${p.long} long`)
    if (p.medium) parts.push(`${p.medium} medium`)
    if (p.short) parts.push(`${p.short} short`)
    const html = `<div style="font-size:12px;line-height:1.5;font-family:Inter,sans-serif">
      <div style="font-weight:600;margin-bottom:2px">${escapeHtml(placeTitle(p))}</div>
      ${p.label?.name ? `<div style="opacity:.8">${placeSummary(p)}</div>` : ''}
      ${p.label?.category && p.label.category !== 'address' && p.label.category !== 'street' ? `<div style="opacity:.6;font-size:11px">${escapeHtml(p.label.category)}</div>` : ''}
      ${parts.length ? `<div style="opacity:.8">${parts.join(' · ')}</div>` : ''}
      <div style="opacity:.6;font-size:11px">${p.arrivals} arrival${p.arrivals === 1 ? '' : 's'} · ${p.departures} departure${p.departures === 1 ? '' : 's'}</div>
    </div>`

    // The place being edited follows its draft, and its pin can be dragged.
    const editingThis = draft.value?.placeId === p.id
    const at: [number, number] = editingThis ? [draft.value!.lat, draft.value!.lon] : [p.lat, p.lon]
    const marker = L.marker(at, { icon, zIndexOffset: selected ? 1000 : 0, draggable: editingThis })
      .bindTooltip(html, { direction: 'top', offset: [0, -size / 2], className: 'trip-tooltip' })
      .on('click', () => select(p.id, false))
      .on('dragend', (e: any) => {
        const ll = e.target.getLatLng()
        if (draft.value) { draft.value.lat = ll.lat; draft.value.lon = ll.lng; drawEditCircle() }
      })
      .addTo(map)
    markers.set(p.id, marker)
    allPts.push([p.lat, p.lon])
  }

  drawEditCircle()

  if (fit && allPts.length) {
    map.fitBounds(L.latLngBounds(allPts), { padding: [40, 40], maxZoom: 15 })
  }
}

// A trip start/end place: a plain dot, or a circle holding its kind's icon.
function endpointMarkerHtml(kind: string, size: number, color: string, ring: string): string {
  const frame = `width:${size}px;height:${size}px;border-radius:50%;background:${color};border:${ring};box-shadow:0 2px 6px rgba(0,0,0,.5)`
  return `<div style="${frame};display:flex;align-items:center;justify-content:center">${placeIconSvg(kind, { size: Math.round(size * 0.62), color: '#fff', strokeWidth: 2.2 })}</div>`
}

// The circle shows which visits the place will claim.
function drawEditCircle() {
  const L = leaflet
  if (!map || !L) return
  if (editCircle) { map.removeLayer(editCircle); editCircle = null }
  const d = draft.value
  if (!d) return
  editCircle = L.circle([d.lat, d.lon], {
    radius: d.radius_m, color: '#a78bfa', weight: 1.5, fillColor: '#a78bfa', fillOpacity: 0.08, dashArray: '4 4', interactive: false,
  }).addTo(map)
}

watch(() => draft.value?.radius_m, () => drawEditCircle())

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
        <span class="flex items-center gap-1.5"><span class="w-2.5 h-2.5 rounded-[3px]" style="background: #38bdf8" /> Short</span>
        <span class="flex items-center gap-1.5"><span class="w-2.5 h-2.5 rounded-[3px]" style="background: #2dd4bf" /> Medium</span>
        <span class="flex items-center gap-1.5"><span class="w-2.5 h-2.5 rounded-[3px]" style="background: #a78bfa" /> Long</span>
        <span class="flex items-center gap-1.5"><span class="w-2.5 h-2.5 rounded-full" style="background: #64748b" /> Trip start / end</span>
        <span style="opacity: .6">colour = longest stop · icon = kind of place</span>
      </div>
    </div>

    <div v-if="status === 'pending' && !data" class="space-y-2">
      <div v-for="i in 4" :key="i" class="skeleton h-16" />
    </div>

    <div v-else-if="places.length === 0" class="rounded-xl p-6" :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }">
      <p class="text-[13px]" style="color: var(--color-text-secondary)">No trips with GPS data yet.</p>
    </div>

    <template v-else>
      <!-- The engine's guess at home -->
      <div
        v-if="homeSuggestion"
        class="flex flex-wrap items-center gap-3 rounded-xl px-4 py-3 mb-4 text-[13px]"
        style="background: rgba(167,139,250,.08); border: 1px solid rgba(167,139,250,.3)"
      >
        <span class="w-8 h-8 rounded-full flex items-center justify-center shrink-0" style="background: rgba(167,139,250,.18); color: #a78bfa">
          <PlaceIcon kind="home" :size="17" />
        </span>
        <span class="flex-1 min-w-[14rem]">
          <span class="font-semibold">This looks like home.</span>
          <span style="color: var(--color-text-secondary)"> Your trips start and end here more than anywhere else. Marking it helps the engine name your other places and trips.</span>
        </span>
        <button type="button" class="px-3 py-1.5 rounded-lg text-[12px] font-semibold" style="background: var(--color-accent); color: #fff" @click="setAsHome(homeSuggestion)">Set as Home</button>
      </div>

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
            <span
              class="w-8 h-8 shrink-0 flex items-center justify-center"
              :class="p.longest_category ? 'rounded-[9px]' : 'rounded-full'"
              :style="{ background: placeColor(p) + '26', color: placeColor(p) }"
              :title="placeKind(kindOfPlace(p)).label"
            >
              <PlaceIcon :kind="kindOfPlace(p)" :size="17" />
            </span>
            <div class="flex-1 min-w-0">
              <p class="text-[13px] font-medium truncate">
                <span v-if="p.label?.saved_id != null && !isLearned(p)" class="mr-1" style="color: #a78bfa" title="Named by you">★</span>{{ placeTitle(p) }}
                <span v-if="isLearned(p)" class="ml-1.5 px-1.5 py-0.5 rounded text-[10px] font-semibold align-middle" style="background: rgba(45,212,191,.15); color: #2dd4bf" title="Found automatically because you keep coming here">Learned</span>
                <span v-if="p.label?.status === 'pending'" class="text-[11px] font-normal" style="color: var(--color-text-secondary)">identifying…</span>
              </p>
              <p v-if="p.label?.name" class="text-[12px] mt-0.5" style="color: var(--color-text-secondary)">
                {{ placeSummary(p) }}<template v-if="p.label.category && p.label.category !== 'address' && p.label.category !== 'street'"> · {{ p.label.category }}</template>
              </p>
              <p class="text-[11px] font-mono mt-0.5" style="color: var(--color-text-secondary)">
                {{ p.lat.toFixed(4) }}, {{ p.lon.toFixed(4) }}
                <template v-if="p.trips.length"> · {{ p.trips.length }} trip{{ p.trips.length === 1 ? '' : 's' }}</template>
                <template v-if="p.last_at"> · last {{ formatDate(p.last_at) }}</template>
              </p>
            </div>
            <div class="flex items-center gap-1.5 text-[10px] font-semibold shrink-0">
              <button
                v-if="data?.hints?.home_place_id === p.id && selectedId !== p.id"
                type="button"
                class="inline-flex items-center gap-1 px-2 py-0.5 rounded-md transition-colors hover:brightness-125"
                style="background: rgba(167,139,250,.15); color: #a78bfa"
                @click.stop="setAsHome(p)"
              >
                <PlaceIcon kind="home" :size="12" />Looks like home
              </button>
              <button
                v-else-if="isFrequent(p) && selectedId !== p.id"
                type="button"
                class="px-2 py-0.5 rounded-md transition-colors hover:brightness-125"
                style="background: rgba(167,139,250,.15); color: #a78bfa"
                @click.stop="select(p.id); startEdit(p)"
              >Name this place</button>
              <span v-if="p.long" class="px-1.5 py-0.5 rounded" style="background: rgba(167,139,250,.15); color: #a78bfa">{{ p.long }} long</span>
              <span v-if="p.medium" class="px-1.5 py-0.5 rounded" style="background: rgba(45,212,191,.15); color: #2dd4bf">{{ p.medium }} medium</span>
              <span v-if="p.short" class="px-1.5 py-0.5 rounded" style="background: rgba(56,189,248,.15); color: #38bdf8">{{ p.short }} short</span>
            </div>
          </div>

          <div v-if="selectedId === p.id" class="mt-3 pt-3 text-[12px]" style="border-top: 1px solid var(--color-border)" @click.stop>
            <div class="flex flex-wrap items-center gap-x-4 gap-y-1.5">
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
              >
                trip {{ b.slice(0, 8) }}
              </NuxtLink>
              <span v-if="p.trips.length > 6" style="color: var(--color-text-secondary)">+{{ p.trips.length - 6 }} more</span>
              <span v-if="isLearned(p)" class="text-[11px]" style="color: var(--color-text-secondary)">
                Learned from {{ learnedTrips(p) ?? 'several' }} trips.
                <button type="button" class="font-semibold underline-offset-2 hover:underline" style="color: #2dd4bf" @click="confirmLearned(p)">Confirm</button>
              </span>
              <button
                v-if="draft?.placeId !== p.id"
                type="button"
                class="ml-auto px-3 py-1 rounded-lg text-[12px] font-semibold transition-colors hover:brightness-125"
                style="background: var(--color-accent-soft); color: var(--color-accent)"
                @click="startEdit(p)"
              >{{ p.label?.saved_id != null ? 'Edit place' : 'Name or fix this place' }}</button>
            </div>

            <!-- Editor -->
            <form v-if="draft && draft.placeId === p.id" class="mt-3 space-y-3 rounded-lg p-3" style="background: var(--color-surface-elevated)" @submit.prevent="saveDraft">
              <div>
                <label class="block text-[10px] font-semibold uppercase tracking-wider mb-1" style="color: var(--color-text-secondary)">Name</label>
                <input
                  v-model="draft.name"
                  type="text"
                  maxlength="80"
                  placeholder="e.g. Home, Gym, Mom's house"
                  class="w-full rounded-md px-2.5 py-1.5 text-[13px] outline-none"
                  style="background: var(--color-surface); border: 1px solid var(--color-border); color: var(--color-text)"
                  autofocus
                >
              </div>

              <div>
                <p class="text-[10px] font-semibold uppercase tracking-wider mb-1" style="color: var(--color-text-secondary)">
                  Kind <span class="normal-case font-normal tracking-normal opacity-70">a hint: it ranks the names below and sets the icon</span>
                </p>
                <div class="flex flex-wrap gap-1.5">
                  <button
                    v-for="k in KIND_CHOICES"
                    :key="k.id"
                    type="button"
                    class="inline-flex items-center gap-1.5 pl-1.5 pr-2.5 py-1 rounded-lg text-[12px] transition-colors"
                    :style="{ background: draftKind() === k.id ? 'var(--color-accent-soft)' : 'var(--color-surface)', border: '1px solid ' + (draftKind() === k.id ? 'var(--color-accent)' : 'var(--color-border)'), color: draftKind() === k.id ? 'var(--color-accent)' : 'var(--color-text)' }"
                    @click="pickKind(k.id)"
                  >
                    <PlaceIcon :kind="k.id" :size="15" />{{ k.label }}
                  </button>
                </div>
              </div>

              <div v-if="p.suggestions.length">
                <p class="text-[10px] font-semibold uppercase tracking-wider mb-1" style="color: var(--color-text-secondary)">Nearby, tap to use</p>
                <div class="flex flex-wrap gap-1.5">
                  <button
                    v-for="s in rankedSuggestions(p)"
                    :key="s.name"
                    type="button"
                    class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-md text-[11px] transition-colors hover:brightness-125"
                    :style="{ background: draft.name === s.name ? 'var(--color-accent-soft)' : 'var(--color-surface)', border: '1px solid var(--color-border)', color: draft.name === s.name ? 'var(--color-accent)' : 'var(--color-text)' }"
                    @click="pickSuggestion(s)"
                  >
                    <PlaceIcon v-if="s.kind !== 'other'" :kind="s.kind" :size="12" />
                    {{ s.name }}<span v-if="s.dist_m" class="opacity-50"> · {{ s.dist_m }} m</span>
                  </button>
                </div>
              </div>

              <div class="flex flex-wrap items-center gap-x-5 gap-y-2">
                <div>
                  <p class="text-[10px] font-semibold uppercase tracking-wider mb-1" style="color: var(--color-text-secondary)">Covers visits within</p>
                  <div class="flex gap-1.5">
                    <button
                      v-for="r in RADII"
                      :key="r"
                      type="button"
                      class="px-2 py-0.5 rounded-md text-[11px] font-mono transition-colors"
                      :style="{ background: draft.radius_m === r ? 'var(--color-accent-soft)' : 'var(--color-surface)', border: '1px solid var(--color-border)', color: draft.radius_m === r ? 'var(--color-accent)' : 'var(--color-text)' }"
                      @click="draft.radius_m = r"
                    >{{ r }} m</button>
                  </div>
                </div>
                <p class="text-[11px]" style="color: var(--color-text-secondary)">Drag the highlighted pin on the map to move it. The dashed circle shows what it covers.</p>
              </div>

              <p v-if="saveError" class="text-[12px]" style="color: var(--color-danger)">{{ saveError }}</p>

              <div class="flex items-center gap-2">
                <button
                  type="submit"
                  :disabled="saving || !draft.name.trim()"
                  class="px-3.5 py-1.5 rounded-lg text-[12px] font-semibold disabled:opacity-40"
                  style="background: var(--color-accent); color: #fff"
                >{{ saving ? 'Saving…' : 'Save' }}</button>
                <button type="button" class="px-3 py-1.5 rounded-lg text-[12px]" style="color: var(--color-text-secondary)" @click="cancelEdit">Cancel</button>
                <button
                  v-if="draft.savedId != null"
                  type="button"
                  class="ml-auto px-3 py-1.5 rounded-lg text-[12px]"
                  style="color: var(--color-danger)"
                  @click="removeSaved(p)"
                >Stop using this name</button>
              </div>
              <p class="text-[10px]" style="color: var(--color-text-secondary)">
                Saving applies to every past and future visit inside the circle, and nothing here is sent to any lookup service.
              </p>
            </form>
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


    <!-- Where saved places are kept -->
    <div v-if="data?.storage" class="mt-6 rounded-xl px-4 py-3 text-[12px]" :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }">
      <div class="flex flex-wrap items-center gap-x-4 gap-y-1.5">
        <span class="font-semibold">Saved places</span>
        <span style="color: var(--color-text-secondary)">
          {{ data.storage.saved }} yours · {{ data.storage.learned }} learned · {{ data.storage.trips_recorded }} trip{{ data.storage.trips_recorded === 1 ? '' : 's' }} in the visit history
        </span>
        <span style="color: var(--color-text-secondary)">last backup {{ fmtBackup(data.storage.backup_at) }}</span>
        <span class="ml-auto flex items-center gap-3">
          <a href="/api/places/saved/export" download class="font-semibold underline-offset-2 hover:underline" style="color: var(--color-accent)">Export</a>
          <label class="font-semibold cursor-pointer underline-offset-2 hover:underline" style="color: var(--color-accent)">
            Import
            <input type="file" accept="application/json,.json" class="hidden" @change="importFile">
          </label>
        </span>
      </div>
      <p v-if="importMsg" class="mt-1.5" style="color: var(--color-text-secondary)">{{ importMsg }}</p>
      <p class="mt-1.5 text-[11px]" style="color: var(--color-text-secondary)">
        Every change is also written to a JSON file beside the database and copied into rotating backups on the server, and restored from it if the database is lost.
      </p>
    </div>

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
