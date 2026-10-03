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
  first_pos_ms: number | null
  first_fix_ms: number | null
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

const gpsFixDelay = computed(() => {
  if (!trip.value) return null
  const { first_obd_ms, first_fix_ms } = trip.value
  if (first_obd_ms == null) return null
  if (first_fix_ms == null) return { delay: null, label: 'No GPS fix acquired' }
  const delayMs = first_fix_ms - first_obd_ms
  if (delayMs <= 0) return { delay: 0, label: 'GPS ready before OBD' }
  const delaySec = delayMs / 1000
  const label = delaySec >= 60
    ? `${Math.floor(delaySec / 60)}m ${Math.round(delaySec % 60)}s blind logging`
    : `${Math.round(delaySec)}s blind logging`
  return { delay: delayMs, label }
})

const insights = computed(() => insightsData.value?.insights ?? [])

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

      <!-- GPS acquisition timeline -->
      <div v-if="gpsFixDelay" class="rounded-xl p-4 mb-4" :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }">
        <div class="flex items-center gap-3 mb-2">
          <svg class="w-4 h-4 shrink-0" :style="{ color: gpsFixDelay.delay === null ? 'var(--color-danger)' : gpsFixDelay.delay === 0 ? '#22c55e' : '#f59e0b' }" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.75">
            <path stroke-linecap="round" stroke-linejoin="round" d="M17.657 16.657L13.414 20.9a1.998 1.998 0 01-2.827 0l-4.244-4.243a8 8 0 1111.314 0z M15 11a3 3 0 11-6 0 3 3 0 016 0z" />
          </svg>
          <span class="text-[11px] font-semibold uppercase tracking-wider" style="color: var(--color-text-secondary)">GPS Acquisition</span>
          <span class="text-[12px] font-mono font-medium" :style="{ color: gpsFixDelay.delay === null ? 'var(--color-danger)' : gpsFixDelay.delay === 0 ? '#22c55e' : '#f59e0b' }">
            {{ gpsFixDelay.label }}
          </span>
        </div>
        <div v-if="gpsFixDelay.delay != null && gpsFixDelay.delay > 0 && trip" class="relative h-3 rounded-full overflow-hidden" style="background: var(--color-surface-elevated)">
          <div
            class="absolute inset-y-0 left-0 rounded-full"
            style="background: linear-gradient(90deg, #f59e0b 0%, #ef4444 100%); opacity: 0.7"
            :style="{ width: Math.min((gpsFixDelay.delay / (trip.duration_s * 1000)) * 100, 100) + '%' }"
          />
          <div class="absolute inset-y-0 left-0 flex items-center pl-2">
            <span class="text-[9px] font-bold text-white drop-shadow-sm">OBD</span>
          </div>
          <div
            class="absolute inset-y-0 rounded-full"
            style="background: linear-gradient(90deg, #22c55e 0%, #3b82f6 100%); opacity: 0.8"
            :style="{
              left: Math.min((gpsFixDelay.delay / (trip.duration_s * 1000)) * 100, 100) + '%',
              width: (100 - Math.min((gpsFixDelay.delay / (trip.duration_s * 1000)) * 100, 100)) + '%'
            }"
          />
        </div>
        <div v-if="gpsFixDelay.delay != null && gpsFixDelay.delay > 0" class="flex justify-between mt-1 text-[9px]" style="color: var(--color-text-secondary)">
          <span>Boot</span>
          <span>GPS fix</span>
          <span>End</span>
        </div>
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
