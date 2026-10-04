<script setup lang="ts">
interface RouteData {
  type: string
  features: Array<{
    type: string
    geometry: { type: string; coordinates: [number, number, number][] }
    properties: { speeds: number[]; timestamps: string[]; headings: number[]; accuracies: number[]; sats: number[] }
  }>
}

interface TripSummary {
  boot_id: string
  duration_s: number
  max_speed_kph: number
  max_rpm: number
  obd_samples: number
  gnss_samples: number
  gap_count: number
  start: { observed_at: string; lat: number; lon: number } | null
  end: { observed_at: string; lat: number; lon: number } | null
  harsh_event_count: number
  first_obd_ms: number | null
  last_obd_ms: number | null
  first_pos_ms: number | null
  first_fix_ms: number | null
  prev_end: { lat: number; lon: number; observed_at: string } | null
}

interface Insight {
  label: string
  value: string
  unit?: string
  detail?: string
  icon?: string
}

const route = useRoute()
const bootId = computed(() => route.params.bootId as string)

const { data: trip, status: tripStatus } = useFetch<TripSummary>(
  () => `/api/trips/${bootId.value}`,
)
const { data: routeData } = useFetch<RouteData>(
  () => `/api/trips/${bootId.value}/route`,
)
interface TripStop {
  start_at: string | null
  start_offset_s: number
  duration_s: number
  lat: number
  lon: number
  category: 'quick' | 'medium' | 'long'
  inferred: boolean
}

const { data: stopsData } = useFetch<{ stops: TripStop[] }>(
  () => `/api/trips/${bootId.value}/stops`,
)
const stops = computed(() => stopsData.value?.stops ?? [])

interface FuelSample { speed_kph: number; maf_cgps: number; lambda_ratio: number }
const { data: fuelData } = useFetch<{ samples: FuelSample[]; distance_m: number; duration_s: number | null }>(
  () => `/api/trips/${bootId.value}/fuel`,
)

const blend = ref(DEFAULT_ETHANOL_BLEND)
onMounted(() => { blend.value = readEthanolBlend() })

// Estimated from the MAF samples the device caught, not a continuous series, so
// the sample count is always shown. Trip MPG is total speed over total fuel flow
// (idle samples included); cruise MPG covers only samples above 5 km/h.
const fuel = computed(() => {
  const d = fuelData.value
  if (!d || d.samples.length < 3) return null
  const rows = d.samples.map(s => ({ kph: s.speed_kph, gph: fuelGalPerHr(s.maf_cgps, s.lambda_ratio, blend.value) }))
    .filter(r => r.gph > 0 && r.gph < 40)
  if (rows.length < 3) return null

  const sum = (xs: number[]) => xs.reduce((a, b) => a + b, 0)
  const mph = (r: { kph: number }) => r.kph / 1.60934
  const tripMpg = sum(rows.map(mph)) / sum(rows.map(r => r.gph))
  const moving = rows.filter(r => r.kph > 5)
  const cruiseMpg = moving.length ? sum(moving.map(mph)) / sum(moving.map(r => r.gph)) : null
  const idleShare = rows.filter(r => r.kph <= 5).length / rows.length
  const miles = d.distance_m / 1609.34
  const idleGph = rows.filter(r => r.kph <= 5)
  return {
    tripMpg,
    cruiseMpg,
    miles,
    gallons: tripMpg > 0 ? miles / tripMpg : null,
    idleShare,
    idleGph: idleGph.length ? sum(idleGph.map(r => r.gph)) / idleGph.length : null,
    samples: rows.length,
  }
})

const STOP_LABEL = { quick: 'Quick', medium: 'Medium', long: 'Long' } as const
const STOP_COLOR = { quick: '#94a3b8', medium: '#f59e0b', long: '#ef4444' } as const

function fmtStop(s: number): string {
  if (s < 60) return `${s}s`
  const m = Math.floor(s / 60)
  if (m < 60) return `${m}m`
  return `${Math.floor(m / 60)}h ${m % 60}m`
}

function fmtClock(s: TripStop): string {
  if (!s.start_at) return `+${Math.round(s.start_offset_s / 60)}m`
  return new Date(s.start_at).toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit' })
}

const { data: insightsData } = useFetch<{ insights: Insight[] }>(
  () => `/api/trips/${bootId.value}/insights`,
)

const coordinates = computed<[number, number, number][]>(() => {
  const feature = routeData.value?.features?.[0]
  if (!feature || feature.geometry.type !== 'LineString') return []
  return feature.geometry.coordinates
})

const speeds = computed<number[]>(() => {
  return routeData.value?.features?.[0]?.properties?.speeds ?? []
})

const headings = computed(() => routeData.value?.features?.[0]?.properties?.headings ?? [])
const accuracies = computed(() => routeData.value?.features?.[0]?.properties?.accuracies ?? [])
const sats = computed(() => routeData.value?.features?.[0]?.properties?.sats ?? [])

function fmtMs(ms: number): string {
  const sec = ms / 1000
  if (sec < 60) return `${Math.round(sec)}s`
  return `${Math.floor(sec / 60)}m ${Math.round(sec % 60)}s`
}

const gpsAcq = computed(() => {
  if (!trip.value) return null
  const { first_obd_ms, last_obd_ms, first_fix_ms, duration_s, prev_end } = trip.value
  if (first_obd_ms == null || last_obd_ms == null) return null

  const totalMs = last_obd_ms - first_obd_ms
  const totalLabel = fmtMs(totalMs)

  if (first_fix_ms == null) {
    return {
      status: 'never' as const,
      blindMs: totalMs,
      totalMs,
      blindPct: 100,
      totalLabel,
      blindLabel: totalLabel,
      fixedLabel: null,
      prevEnd: prev_end,
    }
  }

  const blindMs = Math.max(0, first_fix_ms - first_obd_ms)
  if (blindMs <= 0) {
    return {
      status: 'instant' as const,
      blindMs: 0,
      totalMs,
      blindPct: 0,
      totalLabel,
      blindLabel: null,
      fixedLabel: totalLabel,
      prevEnd: prev_end,
    }
  }

  const fixedMs = totalMs - blindMs
  return {
    status: 'delayed' as const,
    blindMs,
    totalMs,
    blindPct: Math.round((blindMs / totalMs) * 100),
    totalLabel,
    blindLabel: fmtMs(blindMs),
    fixedLabel: fmtMs(fixedMs),
    prevEnd: prev_end,
  }
})

const insights = computed(() => insightsData.value?.insights ?? [])

const timelineHighlight = ref<{ lat: number; lon: number } | null>(null)

function onTimelineHover(pos: { lat: number; lon: number; mono_ms: number } | null) {
  timelineHighlight.value = pos ? { lat: pos.lat, lon: pos.lon } : null
}

function onTimelineSelect(data: any) {
  if (data) {
    timelineHighlight.value = { lat: data.lat, lon: data.lon }
  }
}

const hasValidRoute = computed(() => {
  return coordinates.value.filter(c => c[0] !== 0 && c[1] !== 0).length >= 2
})

function formatDuration(seconds: number | null | undefined): string {
  if (!seconds || seconds <= 0) return '--'
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = Math.floor(seconds % 60)
  if (h > 0) return `${h}h ${m}m`
  return `${m}m ${s}s`
}

function formatDate(iso: string | null | undefined): string {
  if (!iso) return '--'
  const d = new Date(iso)
  if (isNaN(d.getTime())) return '--'
  return d.toLocaleDateString('en-US', {
    weekday: 'short', month: 'short', day: 'numeric',
    hour: 'numeric', minute: '2-digit', hour12: true,
  })
}

function insightIcon(icon: string | undefined): string {
  const map: Record<string, string> = {
    clock: 'M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z',
    road: 'M9 20l-5.447-2.724A1 1 0 013 16.382V5.618a1 1 0 011.447-.894L9 7m0 13l6-3m-6 3V7m6 10l4.553 2.276A1 1 0 0021 18.382V7.618a1 1 0 00-.553-.894L15 4m0 13V4m0 0L9 7',
    gauge: 'M13 10V3L4 14h7v7l9-11h-7z',
    speedometer: 'M13 10V3L4 14h7v7l9-11h-7z',
    mountain: 'M3 21l6-9 4 5 4-7 4 11H3z',
    valley: 'M3 3l6 9 4-5 4 7 4-11H3z',
    turbo: 'M13 10V3L4 14h7v7l9-11h-7z',
    tachometer: 'M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z',
    rocket: 'M13 10V3L4 14h7v7l9-11h-7z',
    flame: 'M17.657 18.657A8 8 0 016.343 7.343S7 9 9 10c0-2 .5-5 2.986-7C14 5 16.09 5.777 17.656 7.343A7.975 7.975 0 0120 13a7.975 7.975 0 01-2.343 5.657z',
    satellite: 'M17.657 16.657L13.414 20.9a1.998 1.998 0 01-2.827 0l-4.244-4.243a8 8 0 1111.314 0z M15 11a3 3 0 11-6 0 3 3 0 016 0z',
    thermometer: 'M12 9v3m0 0v3m0-3h3m-3 0H9m12 0a9 9 0 11-18 0 9 9 0 0118 0z',
    pause: 'M10 9v6m4-6v6m7-3a9 9 0 11-18 0 9 9 0 0118 0z',
  }
  return map[icon ?? ''] ?? 'M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z'
}
</script>

<template>
  <div>
    <!-- Header -->
    <LayoutPageHeader title="Trip Detail" :subtitle="trip ? formatDate(trip.start?.observed_at) : `Boot: ${bootId.slice(0, 12)}…`">
      <template #actions>
        <NuxtLink
          to="/trips"
          class="inline-flex items-center gap-2 px-3.5 py-2 text-[13px] font-medium rounded-xl transition-colors hover:bg-[var(--color-surface-elevated)]"
          :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)', color: 'var(--color-text)' }"
        >
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M15 19l-7-7 7-7" />
          </svg>
          All Trips
        </NuxtLink>
      </template>
    </LayoutPageHeader>

    <!-- Loading -->
    <div v-if="tripStatus === 'pending'" class="flex items-center justify-center py-20">
      <div class="spinner" />
    </div>

    <template v-else-if="trip">
      <!-- Stat bar -->
      <div class="grid grid-cols-2 sm:grid-cols-4 gap-3 mb-6">
        <DataStatCard label="Duration" :value="formatDuration(trip.duration_s)" />
        <DataStatCard label="Max Speed" :value="`${Math.round(trip.max_speed_kph / 1.60934)}`" subtitle="mph" />
        <DataStatCard label="Max RPM" :value="`${trip.max_rpm?.toLocaleString()}`" :color="trip.max_rpm >= 6000 ? 'warning' : 'default'" />
        <DataStatCard label="OBD Samples" :value="`${trip.obd_samples?.toLocaleString()}`" />
      </div>

      <!-- Fuel economy -->
      <div v-if="fuel" class="mb-6">
        <div class="grid grid-cols-2 sm:grid-cols-4 gap-3">
          <DataStatCard label="Trip MPG" :value="fuel.tripMpg.toFixed(1)" :subtitle="`est. at E${blend}`" />
          <DataStatCard label="Cruise MPG" :value="fuel.cruiseMpg != null ? fuel.cruiseMpg.toFixed(1) : '--'" subtitle="moving, above 3 mph" />
          <DataStatCard label="Fuel Used" :value="fuel.gallons != null ? fuel.gallons.toFixed(2) : '--'" :subtitle="`gal over ${fuel.miles.toFixed(1)} mi`" />
          <DataStatCard label="Idle Burn" :value="fuel.idleGph != null ? fuel.idleGph.toFixed(2) : '--'" :subtitle="`gal/hr · ${Math.round(fuel.idleShare * 100)}% of samples`" />
        </div>
        <p class="text-[11px] mt-2 px-1" style="color: var(--color-text-secondary)">
          Estimated from {{ fuel.samples }} MAF airflow samples at E{{ blend }}; the device only catches MAF on some polls, so treat these as approximate.
          <NuxtLink to="/economy" class="underline-offset-2 hover:underline" style="color: var(--color-accent)">Fuel economy details</NuxtLink>
        </p>
      </div>

      <!-- Map -->
      <div class="rounded-xl overflow-hidden mb-4" :style="{ border: '1px solid var(--color-border)' }">
        <ClientOnly>
          <div v-if="hasValidRoute" style="height: 420px">
            <MapsTripMap
              :coordinates="coordinates"
              :speeds="speeds"
              :headings="headings"
              :accuracies="accuracies"
              :sats="sats"
              :first-obd-ms="trip?.first_obd_ms"
              :first-fix-ms="trip?.first_fix_ms"
              :prev-end="trip?.prev_end"
              :highlight-pos="timelineHighlight"
              :stops="stops"
            />
          </div>
          <template #fallback>
            <div class="flex items-center justify-center" style="height: 420px; background: var(--color-surface-elevated)">
              <div class="spinner" />
            </div>
          </template>
        </ClientOnly>
        <div v-if="!hasValidRoute" class="flex items-center justify-center" style="height: 240px; background: var(--color-surface-elevated)">
          <div class="text-center">
            <svg class="w-8 h-8 mx-auto mb-2" style="color: var(--color-text-secondary)" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
              <path stroke-linecap="round" stroke-linejoin="round" d="M17.657 16.657L13.414 20.9a1.998 1.998 0 01-2.827 0l-4.244-4.243a8 8 0 1111.314 0z M15 11a3 3 0 11-6 0 3 3 0 016 0z" />
            </svg>
            <p class="text-[13px]" style="color: var(--color-text-secondary)">No GPS route data for this trip</p>
          </div>
        </div>
      </div>

      <!-- Stops -->
      <div v-if="stops.length" class="rounded-xl p-4 mb-4" :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }">
        <div class="flex items-center gap-4 mb-3">
          <span class="text-[11px] font-semibold uppercase tracking-wider" style="color: var(--color-text-secondary)">Stops</span>
          <span class="text-[10px]" style="color: var(--color-text-secondary)">quick &lt; 2 min · medium 2–10 min · long 10+ min</span>
        </div>
        <div class="space-y-1.5">
          <NuxtLink v-for="(s, i) in stops" :key="i" :to="{ path: '/places', query: { lat: s.lat.toFixed(5), lon: s.lon.toFixed(5) } }" class="flex items-center gap-3 text-[12px] rounded-md px-1 -mx-1 hover:bg-[var(--color-surface-elevated)]" title="Show on Places">
            <span class="w-2.5 h-2.5 rounded-full shrink-0" :style="{ background: STOP_COLOR[s.category], opacity: s.inferred ? 0.6 : 1 }" />
            <span class="font-mono" style="color: var(--color-text-secondary)">{{ fmtClock(s) }}</span>
            <span class="font-semibold">{{ STOP_LABEL[s.category] }}</span>
            <span class="font-mono">{{ fmtStop(s.duration_s) }}</span>
            <span v-if="s.inferred" class="text-[10px]" style="color: var(--color-text-secondary)">no GPS fixes while stopped</span>
          </NuxtLink>
        </div>
      </div>

      <!-- GPS acquisition summary -->
      <div v-if="gpsAcq" class="flex items-center gap-3 px-1 mb-2 text-[12px]">
        <svg class="w-3.5 h-3.5 shrink-0" :style="{ color: gpsAcq.status === 'never' ? 'var(--color-danger)' : gpsAcq.status === 'instant' ? '#22c55e' : '#f59e0b' }" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M17.657 16.657L13.414 20.9a1.998 1.998 0 01-2.827 0l-4.244-4.243a8 8 0 1111.314 0z M15 11a3 3 0 11-6 0 3 3 0 016 0z" />
        </svg>
        <template v-if="gpsAcq.status === 'never'">
          <span class="font-mono font-semibold" style="color: var(--color-danger)">No GPS fix</span>
          <span style="color: var(--color-text-secondary)">— {{ gpsAcq.totalLabel }} logged blind</span>
        </template>
        <template v-else-if="gpsAcq.status === 'instant'">
          <span class="font-mono font-semibold" style="color: #22c55e">GPS ready at boot</span>
          <span style="color: var(--color-text-secondary)">— {{ gpsAcq.totalLabel }} fully tracked</span>
        </template>
        <template v-else>
          <span class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded" style="background: rgba(245, 158, 11, 0.15)">
            <span class="w-1.5 h-1.5 rounded-full" style="background: #f59e0b" />
            <span class="font-mono font-semibold" style="color: #f59e0b">{{ gpsAcq.blindLabel }}</span>
          </span>
          <span style="color: var(--color-text-secondary)">blind ·</span>
          <span class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded" style="background: rgba(34, 197, 94, 0.1)">
            <span class="w-1.5 h-1.5 rounded-full" style="background: #22c55e" />
            <span class="font-mono font-semibold" style="color: #22c55e">{{ gpsAcq.fixedLabel }}</span>
          </span>
          <span style="color: var(--color-text-secondary)">tracked · {{ gpsAcq.totalLabel }} total</span>
        </template>
        <template v-if="gpsAcq.prevEnd && gpsAcq.status !== 'instant'">
          <span class="w-3.5 h-3.5 rounded-full flex items-center justify-center shrink-0" style="border: 1.5px dashed #f59e0b; color: #f59e0b; font-size: 8px; font-weight: 700">?</span>
          <span class="text-[11px]" style="color: var(--color-text-secondary)">est. start</span>
        </template>
      </div>

      <!-- Interactive timeline + GPS health -->
      <div class="mb-4">
        <ClientOnly>
          <ChartsTripTimeline
            :boot-id="bootId"
            :first-obd-ms="trip?.first_obd_ms"
            :last-obd-ms="trip?.last_obd_ms"
            :first-fix-ms="trip?.first_fix_ms"
            @hover="onTimelineHover"
            @select="onTimelineSelect"
          />
          <template #fallback>
            <div class="skeleton rounded-xl" style="height: 180px" />
          </template>
        </ClientOnly>
      </div>

      <!-- Speed / Elevation sparkline -->
      <div class="rounded-xl p-4 mb-6" :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }">
        <div class="flex items-center gap-4 mb-2">
          <span class="text-[11px] font-semibold uppercase tracking-wider" style="color: var(--color-text-secondary)">Speed & Elevation</span>
          <div class="flex items-center gap-3 text-[10px]" style="color: var(--color-text-secondary)">
            <span class="flex items-center gap-1"><span class="w-2 h-0.5 rounded-full" style="background: #3b82f6" /> Speed</span>
            <span class="flex items-center gap-1"><span class="w-2 h-0.5 rounded-full" style="background: #8b5cf6" /> Altitude</span>
          </div>
        </div>
        <ChartsTripSparkline :coordinates="coordinates" :speeds="speeds" />
      </div>

      <!-- Insights grid -->
      <div v-if="insights.length" class="mb-6">
        <h2 class="text-[11px] font-bold uppercase tracking-wider mb-3" style="color: var(--color-text-secondary)">Trip Insights</h2>
        <div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-3">
          <div
            v-for="(insight, i) in insights"
            :key="i"
            class="rounded-xl p-4 transition-all duration-150 hover:-translate-y-0.5"
            :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }"
          >
            <div class="flex items-start gap-3">
              <div class="w-8 h-8 rounded-lg flex items-center justify-center shrink-0 mt-0.5" style="background: var(--color-accent-soft)">
                <svg class="w-4 h-4" style="color: var(--color-accent)" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.75">
                  <path stroke-linecap="round" stroke-linejoin="round" :d="insightIcon(insight.icon)" />
                </svg>
              </div>
              <div class="min-w-0">
                <p class="text-[11px] font-semibold uppercase tracking-wider" style="color: var(--color-text-secondary)">{{ insight.label }}</p>
                <p class="text-lg font-bold font-mono mt-0.5 tracking-tight">
                  {{ insight.value }}
                  <span v-if="insight.unit" class="text-xs font-normal" style="color: var(--color-text-secondary)">{{ insight.unit }}</span>
                </p>
                <p v-if="insight.detail" class="text-[11px] mt-0.5" style="color: var(--color-text-secondary)">{{ insight.detail }}</p>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Trip metadata footer -->
      <div class="rounded-xl p-4" :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }">
        <div class="flex flex-wrap gap-x-8 gap-y-2 text-[11px]" style="color: var(--color-text-secondary)">
          <span><span class="font-semibold">Boot ID:</span> <span class="font-mono">{{ bootId }}</span></span>
          <span v-if="trip.gnss_samples"><span class="font-semibold">GNSS:</span> {{ trip.gnss_samples }} fixes</span>
          <span v-if="trip.gap_count"><span class="font-semibold">Gaps:</span> {{ trip.gap_count }}</span>
          <span v-if="trip.harsh_event_count"><span class="font-semibold">Harsh events:</span> {{ trip.harsh_event_count }}</span>
          <span v-if="trip.start?.observed_at"><span class="font-semibold">Started:</span> {{ formatDate(trip.start.observed_at) }}</span>
          <span v-if="trip.end?.observed_at"><span class="font-semibold">Ended:</span> {{ formatDate(trip.end.observed_at) }}</span>
        </div>
      </div>
    </template>
  </div>
</template>
