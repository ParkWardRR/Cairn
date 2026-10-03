<script setup lang="ts">
import 'echarts'
import VChart from 'vue-echarts'

definePageMeta({ layout: 'default' })

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface Trip {
  boot_id: string
  observed_at?: string
  hard_brake_count?: number
  hard_accel_count?: number
  event_count?: number
}

interface TelemetryResponse {
  accel_x: number[]
  accel_y: number[]
}

interface BehaviorEvent {
  type: 'hard_brake' | 'hard_accel' | 'other'
  description: string
  timestamp: string
  boot_id: string
}

// ---------------------------------------------------------------------------
// Data fetching
// ---------------------------------------------------------------------------

const { data: trips, pending: tripsPending } = useFetch<Trip[]>('/api/trips')
const { data: telemetry, pending: telemetryPending } = useFetch<TelemetryResponse>(
  '/api/analytics/telemetry?boot_id=latest',
)

// ---------------------------------------------------------------------------
// Computed stats
// ---------------------------------------------------------------------------

const totalHardBrakes = computed(() =>
  (trips.value ?? []).reduce((sum, t) => sum + (t.hard_brake_count ?? 0), 0),
)

const totalHardAccels = computed(() =>
  (trips.value ?? []).reduce((sum, t) => sum + (t.hard_accel_count ?? 0), 0),
)

const totalEvents = computed(() =>
  (trips.value ?? []).reduce((sum, t) => sum + (t.event_count ?? 0), 0),
)

// ---------------------------------------------------------------------------
// G-Force chart
// ---------------------------------------------------------------------------

const hasImuData = computed(() => {
  const t = telemetry.value
  return t && t.accel_x?.length > 0 && t.accel_y?.length > 0
})

const gForceChartOption = computed(() => {
  if (!hasImuData.value) return {}
  const t = telemetry.value!
  const data = t.accel_x.map((x, i) => [x, t.accel_y[i]])

  return {
    backgroundColor: 'transparent',
    grid: { top: 24, right: 24, bottom: 40, left: 48 },
    tooltip: {
      trigger: 'item',
      backgroundColor: '#1a1d27',
      borderColor: '#2e3347',
      textStyle: { color: '#e8eaf0', fontSize: 12 },
      formatter: (p: any) => `Lateral: ${p.value[0].toFixed(3)} g<br>Longitudinal: ${p.value[1].toFixed(3)} g`,
    },
    xAxis: {
      type: 'value',
      name: 'Lateral G',
      nameLocation: 'center',
      nameGap: 24,
      nameTextStyle: { color: '#8b90a0', fontSize: 12 },
      min: -1.5,
      max: 1.5,
      axisLabel: { color: '#8b90a0' },
      splitLine: { lineStyle: { color: '#2e3347' } },
      axisLine: { lineStyle: { color: '#2e3347' } },
    },
    yAxis: {
      type: 'value',
      name: 'Longitudinal G',
      nameLocation: 'center',
      nameGap: 32,
      nameTextStyle: { color: '#8b90a0', fontSize: 12 },
      min: -1.5,
      max: 1.5,
      axisLabel: { color: '#8b90a0' },
      splitLine: { lineStyle: { color: '#2e3347' } },
      axisLine: { lineStyle: { color: '#2e3347' } },
    },
    series: [
      {
        type: 'scatter',
        data,
        symbolSize: 4,
        itemStyle: { color: '#3b82f6', opacity: 0.5 },
        markLine: {
          silent: true,
          symbol: 'none',
          lineStyle: { color: '#2e3347', type: 'solid', width: 1 },
          data: [
            { xAxis: 0 },
            { yAxis: 0 },
          ],
          label: { show: false },
        },
      },
    ],
  }
})

// ---------------------------------------------------------------------------
// Recent events (synthetic from trip data)
// ---------------------------------------------------------------------------

const recentEvents = computed<BehaviorEvent[]>(() => {
  const t = trips.value ?? []
  const events: BehaviorEvent[] = []

  for (const trip of t) {
    const ts = trip.observed_at ?? ''
    for (let i = 0; i < (trip.hard_brake_count ?? 0); i++) {
      events.push({
        type: 'hard_brake',
        description: 'Hard braking detected',
        timestamp: ts,
        boot_id: trip.boot_id,
      })
    }
    for (let i = 0; i < (trip.hard_accel_count ?? 0); i++) {
      events.push({
        type: 'hard_accel',
        description: 'Hard acceleration detected',
        timestamp: ts,
        boot_id: trip.boot_id,
      })
    }
  }

  events.sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime())
  return events.slice(0, 20)
})

function eventDotColor(type: string): string {
  if (type === 'hard_brake') return 'var(--color-danger)'
  if (type === 'hard_accel') return 'var(--color-warning)'
  return 'var(--color-info)'
}

function formatTimestamp(iso: string): string {
  if (!iso) return '--'
  const d = new Date(iso)
  return d.toLocaleString('en-US', {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
    hour12: true,
  })
}
</script>

<template>
  <div>
    <LayoutPageHeader title="Drive Behavior" subtitle="Driving style and event analysis" />

    <!-- Stat cards -->
    <div class="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-8">
      <template v-if="tripsPending">
        <div
          v-for="i in 3"
          :key="i"
          class="skeleton h-[100px]"
        />
      </template>
      <template v-else>
        <DataStatCard
          label="Hard Brakes"
          :value="String(totalHardBrakes)"
          subtitle="total across all trips"
          :color="totalHardBrakes > 10 ? 'danger' : totalHardBrakes > 0 ? 'warning' : undefined"
        />
        <DataStatCard
          label="Hard Accels"
          :value="String(totalHardAccels)"
          subtitle="total across all trips"
          :color="totalHardAccels > 10 ? 'danger' : totalHardAccels > 0 ? 'warning' : undefined"
        />
        <DataStatCard
          label="Total Events"
          :value="String(totalEvents)"
          subtitle="all event types"
        />
      </template>
    </div>

    <!-- G-Force chart -->
    <div
      class="rounded-xl p-6 mb-6"
      :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }"
    >
      <h2 class="text-base font-semibold mb-4">G-Force Distribution</h2>

      <template v-if="telemetryPending">
        <div class="flex items-center justify-center" style="height: 400px">
          <div class="spinner" />
        </div>
      </template>

      <template v-else-if="hasImuData">
        <v-chart
          :option="gForceChartOption"
          style="height: 400px; width: 100%"
          autoresize
        />
      </template>

      <DataEmptyState
        v-else
        title="No IMU data"
        message="G-force data will appear once accelerometer telemetry is available."
      />
    </div>

    <!-- Recent events -->
    <div
      class="rounded-xl p-6"
      :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }"
    >
      <h2 class="text-base font-semibold mb-4">Recent Events</h2>

      <template v-if="tripsPending">
        <p class="text-sm" style="color: var(--color-text-secondary)">Loading...</p>
      </template>

      <template v-else-if="recentEvents.length > 0">
        <div class="space-y-3">
          <div
            v-for="(event, i) in recentEvents"
            :key="i"
            class="flex items-start gap-3 py-2"
            :style="i < recentEvents.length - 1
              ? { borderBottom: '1px solid var(--color-border)' }
              : {}"
          >
            <!-- Color dot -->
            <div
              class="w-2.5 h-2.5 rounded-full mt-1.5 shrink-0"
              :style="{ backgroundColor: eventDotColor(event.type) }"
            />

            <!-- Content -->
            <div class="flex-1 min-w-0">
              <p class="text-sm">{{ event.description }}</p>
              <p class="text-xs font-mono mt-0.5" style="color: var(--color-text-secondary)">
                Boot {{ event.boot_id }} -- {{ formatTimestamp(event.timestamp) }}
              </p>
            </div>
          </div>
        </div>
      </template>

      <DataEmptyState
        v-else
        title="No events recorded"
        message="Drive events will appear here as trips are logged."
      />
    </div>
  </div>
</template>
