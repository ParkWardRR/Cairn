<script setup lang="ts">
import 'echarts'
import VChart from 'vue-echarts'

definePageMeta({ layout: 'default' })

interface HealthSample {
  observed_at: string
  battery_mv: number | null
  device_temp_c: number | null
  sd_free_mib: number | null
  rssi_dbm: number | null
}

interface Bundle {
  bundle_id: string
  boot_id: string
  origin: string
  reproduced: boolean
  n_position: number
  n_obd: number
  n_boost: number
  n_imu: number
  n_status: number
  decode_ms: number
}

interface TsdbStatus {
  ok: boolean
  tables: number
  position_rows: number
  obd_rows: number
  boost_rows: number
  imu_rows: number
  status_rows: number
  transition_rows: number
  gap_rows: number
  bundle_rows: number
  trips: number
}

const { data: healthData, pending: healthPending } = useFetch<{ health: HealthSample[] }>('/api/device/health')
const { data: bundlesData, pending: bundlesPending } = useFetch<{ bundles: Bundle[] }>('/api/device/bundles')
const { data: tsdb, pending: tsdbPending } = useFetch<TsdbStatus>('/api/device/tsdb-status')

const series = computed(() => healthData.value?.health ?? [])
const bundles = computed(() => bundlesData.value?.bundles ?? [])

function formatDate(iso: string | null | undefined): string {
  if (!iso) return '--'
  const d = new Date(iso)
  if (isNaN(d.getTime())) return '--'
  return d.toLocaleDateString('en-US', {
    month: 'short', day: 'numeric', year: 'numeric',
    hour: 'numeric', minute: '2-digit', hour12: true,
  })
}

const timestamps = computed(() => series.value.map(s => s.observed_at))

function makeLineChart(dataKey: keyof HealthSample, color: string, yLabel: string) {
  return computed(() => ({
    backgroundColor: 'transparent',
    grid: { top: 16, right: 16, bottom: 32, left: 48 },
    tooltip: {
      trigger: 'axis' as const,
      backgroundColor: '#1a1d27', borderColor: '#2e3347',
      textStyle: { color: '#e8eaf0', fontSize: 12 },
    },
    xAxis: {
      type: 'category' as const, data: timestamps.value,
      axisLabel: {
        color: '#8b90a0', fontSize: 10,
        formatter: (val: string) => { const d = new Date(val); return `${d.getHours()}:${String(d.getMinutes()).padStart(2, '0')}` },
      },
      axisLine: { lineStyle: { color: '#2e3347' } },
      splitLine: { show: false },
    },
    yAxis: {
      type: 'value' as const, name: yLabel,
      nameTextStyle: { color: '#8b90a0', fontSize: 11 },
      axisLabel: { color: '#8b90a0', fontSize: 10 },
      splitLine: { lineStyle: { color: '#2e3347' } },
    },
    series: [{
      type: 'line', data: series.value.map(s => s[dataKey]),
      smooth: true, showSymbol: false,
      lineStyle: { color, width: 2 },
      areaStyle: { color: `${color}18` },
      itemStyle: { color },
    }],
  }))
}

const tempChart = makeLineChart('device_temp_c', '#f59e0b', '°C')
const sdChart = makeLineChart('sd_free_mib', '#3b82f6', 'MiB')

const tsdbPairs = computed(() => {
  if (!tsdb.value) return []
  const t = tsdb.value
  const totalRows = (t.position_rows ?? 0) + (t.obd_rows ?? 0) + (t.boost_rows ?? 0) +
    (t.imu_rows ?? 0) + (t.status_rows ?? 0) + (t.transition_rows ?? 0) + (t.gap_rows ?? 0)
  return [
    { label: 'Trips', value: String(t.trips ?? 0) },
    { label: 'Total Rows', value: totalRows.toLocaleString() },
    { label: 'Position', value: (t.position_rows ?? 0).toLocaleString() },
    { label: 'OBD', value: (t.obd_rows ?? 0).toLocaleString() },
    { label: 'Boost', value: (t.boost_rows ?? 0).toLocaleString() },
    { label: 'IMU', value: (t.imu_rows ?? 0).toLocaleString() },
    { label: 'Bundles', value: String(t.bundle_rows ?? 0) },
    { label: 'Tables', value: String(t.tables ?? 0) },
  ]
})
</script>

<template>
  <div>
    <LayoutPageHeader title="Device & System" subtitle="Hardware health and data pipeline" />

    <!-- Health charts -->
    <div class="grid grid-cols-1 lg:grid-cols-2 gap-4 mb-6">
      <template v-if="healthPending">
        <div v-for="i in 2" :key="i" class="rounded-lg animate-pulse" style="height: 248px"
          :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }" />
      </template>
      <template v-else-if="series.length > 0">
        <div class="rounded-lg p-4" :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }">
          <h3 class="text-sm font-semibold mb-2">Temperature</h3>
          <v-chart :option="tempChart" style="height: 200px; width: 100%" autoresize />
        </div>
        <div class="rounded-lg p-4" :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }">
          <h3 class="text-sm font-semibold mb-2">SD Free</h3>
          <v-chart :option="sdChart" style="height: 200px; width: 100%" autoresize />
        </div>
      </template>
      <template v-else>
        <div class="lg:col-span-2 rounded-lg p-6"
          :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }">
          <DataEmptyState title="No health data" message="Device health metrics will appear once the dongle reports in." />
        </div>
      </template>
    </div>

    <!-- Data Bundles -->
    <div class="rounded-lg p-6 mb-6"
      :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }">
      <h2 class="text-lg font-semibold mb-4">Data Bundles</h2>

      <template v-if="bundlesPending">
        <p class="text-sm" style="color: var(--color-text-secondary)">Loading...</p>
      </template>

      <template v-else-if="bundles.length > 0">
        <div class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead>
              <tr class="text-xs uppercase tracking-wider text-left"
                :style="{ color: 'var(--color-text-secondary)', borderBottom: '1px solid var(--color-border)' }">
                <th class="pb-3 pr-4 font-medium">Boot ID</th>
                <th class="pb-3 pr-4 font-medium">Origin</th>
                <th class="pb-3 pr-4 font-medium text-right">Position</th>
                <th class="pb-3 pr-4 font-medium text-right">OBD</th>
                <th class="pb-3 pr-4 font-medium text-right">Boost</th>
                <th class="pb-3 pr-4 font-medium text-right">IMU</th>
                <th class="pb-3 font-medium">Reproduced</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="b in bundles" :key="b.bundle_id"
                :style="{ borderBottom: '1px solid var(--color-border)' }">
                <td class="py-3 pr-4 font-mono text-xs">{{ b.boot_id.slice(0, 12) }}…</td>
                <td class="py-3 pr-4 text-xs">{{ b.origin }}</td>
                <td class="py-3 pr-4 font-mono text-xs text-right">{{ b.n_position }}</td>
                <td class="py-3 pr-4 font-mono text-xs text-right">{{ b.n_obd }}</td>
                <td class="py-3 pr-4 font-mono text-xs text-right">{{ b.n_boost }}</td>
                <td class="py-3 pr-4 font-mono text-xs text-right">{{ b.n_imu }}</td>
                <td class="py-3">
                  <DataStatusBadge
                    :status="b.reproduced ? 'success' : 'warning'"
                    :label="b.reproduced ? 'Yes' : 'No'"
                  />
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </template>

      <DataEmptyState v-else title="No bundles" message="Data bundles will appear here as the device uploads telemetry." />
    </div>

    <!-- TSDB Status -->
    <div class="rounded-lg p-6"
      :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }">
      <h2 class="text-lg font-semibold mb-4">TSDB Status</h2>

      <template v-if="tsdbPending">
        <p class="text-sm" style="color: var(--color-text-secondary)">Loading...</p>
      </template>

      <template v-else-if="tsdb">
        <div class="grid grid-cols-2 sm:grid-cols-4 gap-6">
          <div v-for="pair in tsdbPairs" :key="pair.label">
            <p class="text-xs font-medium uppercase tracking-wider font-sans mb-1" style="color: var(--color-text-secondary)">
              {{ pair.label }}
            </p>
            <p class="text-base font-mono font-medium">{{ pair.value }}</p>
          </div>
        </div>
      </template>

      <DataEmptyState v-else title="TSDB unavailable" message="Time-series database status will appear once the service is reachable." />
    </div>
  </div>
</template>
