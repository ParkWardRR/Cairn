<script setup lang="ts">
interface TripSummary {
  boot_id: string
  observed_at: string
  duration_s: number
  distance_m: number
  max_speed_kph: number
  max_rpm: number
  gap_count: number
}

interface Insight {
  label: string
  value: string
  unit?: string
  detail?: string
}

interface TripEvent {
  type: string
  description: string
  timestamp: string
}

const route = useRoute()
const bootId = computed(() => route.params.bootId as string)

const { data: trip, status: tripStatus } = useFetch<TripSummary>(
  () => `/api/trips/${bootId.value}`,
)
const { data: insights } = useFetch<Insight[]>(
  () => `/api/trips/${bootId.value}/insights`,
)
const { data: events } = useFetch<TripEvent[]>(
  () => `/api/trips/${bootId.value}/events`,
)

const eventDotColors: Record<string, string> = {
  warning: 'var(--color-warning)',
  danger: 'var(--color-danger)',
  error: 'var(--color-danger)',
  info: 'var(--color-info)',
  success: 'var(--color-success)',
}

function dotColor(type: string): string {
  return eventDotColors[type] ?? 'var(--color-accent)'
}

function formatTimestamp(iso: string): string {
  const d = new Date(iso)
  return d.toLocaleTimeString('en-US', {
    hour: 'numeric',
    minute: '2-digit',
    second: '2-digit',
    hour12: true,
  })
}
</script>

<template>
  <div>
    <LayoutPageHeader title="Trip Detail" :subtitle="`Boot ID: ${bootId}`">
      <template #actions>
        <NuxtLink
          to="/trips"
          class="inline-flex items-center gap-2 px-4 py-2 text-sm font-medium rounded-xl transition-colors"
          :style="{
            backgroundColor: 'var(--color-surface)',
            border: '1px solid var(--color-border)',
            color: 'var(--color-text)',
          }"
        >
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M15 19l-7-7 7-7" />
          </svg>
          Back to Trips
        </NuxtLink>
      </template>
    </LayoutPageHeader>

    <div v-if="tripStatus === 'pending'" class="flex items-center justify-center py-16">
      <div class="spinner" />
    </div>

    <template v-else>
      <!-- Stat cards row -->
      <div v-if="trip" class="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
        <DataStatCard
          label="Duration"
          :value="trip.duration_s >= 3600
            ? `${Math.floor(trip.duration_s / 3600)}:${String(Math.floor((trip.duration_s % 3600) / 60)).padStart(2, '0')}`
            : `${Math.floor(trip.duration_s / 60)}:${String(Math.floor(trip.duration_s % 60)).padStart(2, '0')}`"
          :subtitle="trip.duration_s >= 3600 ? 'hours' : 'minutes'"
        />
        <DataStatCard
          label="Distance"
          :value="`${(trip.distance_m / 1000).toFixed(1)}`"
          subtitle="km"
        />
        <DataStatCard
          label="Max Speed"
          :value="`${trip.max_speed_kph}`"
          subtitle="kph"
        />
        <DataStatCard
          label="Max RPM"
          :value="`${trip.max_rpm}`"
          :color="trip.max_rpm >= 6000 ? 'warning' : 'default'"
        />
      </div>

      <!-- Two-panel layout -->
      <div class="grid grid-cols-1 lg:grid-cols-[65fr_35fr] gap-6">
        <!-- Left panel -->
        <div class="space-y-4">
          <!-- Map placeholder -->
          <div
            class="h-80 rounded-xl flex items-center justify-center"
            :style="{
              backgroundColor: 'var(--color-surface-elevated)',
              border: '1px solid var(--color-border)',
            }"
          >
            <span class="text-[13px] font-sans" style="color: var(--color-text-secondary)">
              Map loads here
            </span>
          </div>

          <!-- Elevation / speed chart placeholder -->
          <div
            class="h-48 rounded-xl flex items-center justify-center"
            :style="{
              backgroundColor: 'var(--color-surface-elevated)',
              border: '1px solid var(--color-border)',
            }"
          >
            <span class="text-[13px] font-sans" style="color: var(--color-text-secondary)">
              Elevation chart placeholder
            </span>
          </div>
        </div>

        <!-- Right panel -->
        <div class="space-y-6">
          <!-- Insights section -->
          <div>
            <h2 class="text-[11px] font-bold uppercase tracking-wider mb-3" style="color: var(--color-text-secondary)">
              Insights
            </h2>
            <div class="space-y-3">
              <div
                v-for="(insight, i) in (insights ?? [])"
                :key="i"
                class="rounded-xl p-4"
                :style="{
                  backgroundColor: 'var(--color-surface)',
                  border: '1px solid var(--color-border)',
                }"
              >
                <p class="text-[11px] font-semibold uppercase tracking-wider font-sans" style="color: var(--color-text-secondary)">
                  {{ insight.label }}
                </p>
                <p class="text-xl font-bold font-mono mt-1">
                  {{ insight.value }}<span v-if="insight.unit" class="text-sm font-normal ml-1" style="color: var(--color-text-secondary)">{{ insight.unit }}</span>
                </p>
                <p v-if="insight.detail" class="text-xs mt-1 font-sans" style="color: var(--color-text-secondary)">
                  {{ insight.detail }}
                </p>
              </div>
              <DataEmptyState
                v-if="(insights ?? []).length === 0"
                title="No insights"
                message="Insight data is not available for this trip."
              />
            </div>
          </div>

          <!-- Events timeline -->
          <div>
            <h2 class="text-[11px] font-bold uppercase tracking-wider mb-3" style="color: var(--color-text-secondary)">
              Events
            </h2>
            <div v-if="(events ?? []).length > 0" class="relative pl-6">
              <!-- Vertical line -->
              <div
                class="absolute left-[7px] top-2 bottom-2 w-px"
                :style="{ backgroundColor: 'var(--color-border)' }"
              />

              <div v-for="(event, i) in events" :key="i" class="relative pb-5 last:pb-0">
                <!-- Dot -->
                <div
                  class="absolute -left-6 top-1.5 w-[9px] h-[9px] rounded-full ring-[3px]"
                  :style="{
                    backgroundColor: dotColor(event.type),
                    ringColor: 'var(--color-bg)',
                  }"
                />
                <p class="text-xs font-mono" style="color: var(--color-text-secondary)">
                  {{ formatTimestamp(event.timestamp) }}
                </p>
                <p class="text-sm mt-0.5">
                  {{ event.description }}
                </p>
              </div>
            </div>
            <DataEmptyState
              v-else
              title="No events"
              message="No events were recorded during this trip."
            />
          </div>

          <!-- Actions -->
          <div class="space-y-3">
            <button
              class="w-full px-4 py-2.5 text-sm font-medium rounded-xl hover:opacity-90 transition-all"
              :style="{
                backgroundColor: 'var(--color-accent)',
                color: 'var(--color-bg)',
              }"
            >
              Export Trip
            </button>
            <div
              class="rounded-xl p-4"
              :style="{
                backgroundColor: 'var(--color-surface)',
                border: '1px solid var(--color-border)',
              }"
            >
              <p class="text-[11px] font-semibold uppercase tracking-wider font-sans" style="color: var(--color-text-secondary)">
                Tags
              </p>
              <p class="text-sm mt-2" style="color: var(--color-text-secondary)">
                No tags added yet
              </p>
            </div>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>
