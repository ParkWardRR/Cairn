<script setup lang="ts">
import 'echarts'
import VChart from 'vue-echarts'

definePageMeta({ layout: 'default' })

interface Trip {
  boot_id: string
  duration_s: number
  max_speed_kph: number
  max_rpm: number
  obd_samples: number
  start_time?: string
}

interface ImuSample {
  boot_id: string
  mono_ms: number
  observed_at: string
  accel_peak_x_mg: number
  accel_peak_y_mg: number
  accel_peak_z_mg: number
  accel_rms_mg: number
  gyro_peak_dps: number
  event_flags: number
}

const { data: tripsData, pending: tripsPending } = useFetch<{ trips: Trip[]; total: number }>('/api/trips')
const trips = computed(() => tripsData.value?.trips ?? [])

const latestBootId = computed(() => trips.value[0]?.boot_id ?? null)

const { data: imuData, pending: imuPending } = useFetch<{ samples: ImuSample[] }>(
  () => latestBootId.value ? `/api/analytics/imu?boot_id=${latestBootId.value}` : '/api/analytics/imu',
  { watch: [latestBootId] },
)

const imuSamples = computed(() => imuData.value?.samples ?? [])

const stats = computed(() => {
  const samples = imuSamples.value
  if (!samples.length) return { peakLateral: 0, peakLong: 0, avgRms: 0, peakGyro: 0, total: 0 }

  let peakLat = 0, peakLong = 0, sumRms = 0, peakGyro = 0
  for (const s of samples) {
    const latG = Math.abs(s.accel_peak_y_mg) / 1000
    const longG = Math.abs(s.accel_peak_x_mg) / 1000
    if (latG > peakLat) peakLat = latG
    if (longG > peakLong) peakLong = longG
    sumRms += s.accel_rms_mg
    if (s.gyro_peak_dps > peakGyro) peakGyro = s.gyro_peak_dps
  }
  return {
    peakLateral: peakLat,
    peakLong,
    avgRms: sumRms / samples.length / 1000,
    peakGyro,
    total: samples.length,
  }
})

const gForceChartOption = computed(() => {
  if (!imuSamples.value.length) return {}

  const data = imuSamples.value.map(s => [
    s.accel_peak_y_mg / 1000,
    s.accel_peak_x_mg / 1000,
  ])

  const maxG = 1.2
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
      min: -maxG,
      max: maxG,
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
      min: -maxG,
      max: maxG,
      axisLabel: { color: '#8b90a0' },
      splitLine: { lineStyle: { color: '#2e3347' } },
      axisLine: { lineStyle: { color: '#2e3347' } },
    },
    series: [
      {
        type: 'scatter',
        data,
        symbolSize: 3,
        itemStyle: { color: '#3b82f6', opacity: 0.4 },
        markLine: {
          silent: true,
          symbol: 'none',
          lineStyle: { color: '#2e3347', type: 'solid', width: 1 },
          data: [{ xAxis: 0 }, { yAxis: 0 }],
          label: { show: false },
        },
      },
    ],
  }
})

const rmsHistogramOption = computed(() => {
  if (!imuSamples.value.length) return {}

  const bucketSize = 5
  const buckets = new Map<number, number>()
  for (const s of imuSamples.value) {
    const bucket = Math.floor(s.accel_rms_mg / bucketSize) * bucketSize
    buckets.set(bucket, (buckets.get(bucket) ?? 0) + 1)
  }
  const sorted = [...buckets.entries()].sort((a, b) => a[0] - b[0])

  return {
    backgroundColor: 'transparent',
    grid: { top: 16, right: 16, bottom: 40, left: 48 },
    tooltip: {
      trigger: 'axis',
      backgroundColor: '#1a1d27',
      borderColor: '#2e3347',
      textStyle: { color: '#e8eaf0', fontSize: 12 },
      formatter: (p: any) => `${p[0].name} mg: ${p[0].value} samples`,
    },
    xAxis: {
      type: 'category',
      data: sorted.map(([b]) => String(b)),
      name: 'RMS (mg)',
      nameLocation: 'center',
      nameGap: 24,
      nameTextStyle: { color: '#8b90a0', fontSize: 12 },
      axisLabel: { color: '#8b90a0', interval: 'auto' },
      axisLine: { lineStyle: { color: '#2e3347' } },
    },
    yAxis: {
      type: 'value',
      axisLabel: { color: '#8b90a0' },
      splitLine: { lineStyle: { color: '#2e3347' } },
      axisLine: { lineStyle: { color: '#2e3347' } },
    },
    series: [{
      type: 'bar',
      data: sorted.map(([, c]) => c),
      itemStyle: { color: '#8b5cf6', borderRadius: [2, 2, 0, 0] },
      barMaxWidth: 20,
    }],
  }
})

function formatDate(iso: string | null | undefined): string {
  if (!iso) return '--'
  const d = new Date(iso)
  if (isNaN(d.getTime()) || d.getFullYear() < 2000) return '--'
  return d.toLocaleDateString('en-US', {
    month: 'short', day: 'numeric',
    hour: 'numeric', minute: '2-digit', hour12: true,
  })
}
</script>

<template>
  <div>
    <LayoutPageHeader title="Drive Behavior" subtitle="Driving style and motion analysis">
      <template #actions>
        <span v-if="latestBootId" class="text-xs font-medium px-2.5 py-1 rounded-full" style="background: var(--color-accent-soft); color: var(--color-accent)">
          {{ stats.total.toLocaleString() }} IMU samples
        </span>
      </template>
    </LayoutPageHeader>

    <!-- Stat cards -->
    <div class="grid grid-cols-2 lg:grid-cols-4 gap-3 md:gap-4 mb-8">
      <template v-if="imuPending || tripsPending">
        <div v-for="i in 4" :key="i" class="skeleton h-[100px]" />
      </template>
      <template v-else>
        <DataStatCard
          label="Peak Lateral G"
          :value="stats.peakLateral.toFixed(2)"
          subtitle="max cornering force"
          :color="stats.peakLateral > 0.5 ? 'warning' : undefined"
        />
        <DataStatCard
          label="Peak Longitudinal G"
          :value="stats.peakLong.toFixed(2)"
          subtitle="max accel/brake force"
          :color="stats.peakLong > 0.5 ? 'warning' : undefined"
        />
        <DataStatCard
          label="Avg Vibration"
          :value="`${(stats.avgRms * 1000).toFixed(0)} mg`"
          subtitle="mean RMS acceleration"
        />
        <DataStatCard
          label="Peak Yaw Rate"
          :value="`${stats.peakGyro.toFixed(1)}°/s`"
          subtitle="max rotational speed"
        />
      </template>
    </div>

    <!-- G-Force scatter -->
    <div class="rounded-xl p-5 mb-6" :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }">
      <div class="flex items-center justify-between mb-4">
        <h2 class="text-[13px] font-semibold">G-Force Distribution</h2>
        <span v-if="latestBootId" class="text-[11px] font-mono" style="color: var(--color-text-secondary)">
          Latest trip · {{ formatDate(trips[0]?.start_time) }}
        </span>
      </div>

      <template v-if="imuPending">
        <div class="flex items-center justify-center" style="height: 400px">
          <div class="spinner" />
        </div>
      </template>

      <template v-else-if="imuSamples.length > 0">
        <VChart :option="gForceChartOption" style="height: 400px; width: 100%" autoresize />
      </template>

      <DataEmptyState v-else title="No IMU data" message="G-force data will appear once accelerometer telemetry is available." />
    </div>

    <!-- Vibration histogram -->
    <div class="rounded-xl p-5 mb-6" :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }">
      <h2 class="text-[13px] font-semibold mb-4">Vibration Distribution</h2>

      <template v-if="imuPending">
        <div class="flex items-center justify-center" style="height: 240px">
          <div class="spinner" />
        </div>
      </template>

      <template v-else-if="imuSamples.length > 0">
        <VChart :option="rmsHistogramOption" style="height: 240px; width: 100%" autoresize />
      </template>

      <DataEmptyState v-else title="No vibration data" message="Vibration histogram will appear once IMU data is available." />
    </div>

    <!-- Trip-level summary -->
    <div class="rounded-xl p-5" :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }">
      <h2 class="text-[13px] font-semibold mb-4">Trips</h2>
      <template v-if="tripsPending">
        <div class="space-y-2"><div v-for="i in 3" :key="i" class="skeleton h-12" /></div>
      </template>
      <template v-else-if="trips.length">
        <div class="space-y-2">
          <div
            v-for="t in trips"
            :key="t.boot_id"
            class="flex items-center justify-between py-2.5 cursor-pointer transition-colors hover:bg-[var(--color-surface-elevated)] rounded-lg px-3 -mx-3"
            @click="navigateTo(`/trips/${t.boot_id}`)"
          >
            <div>
              <p class="text-[13px] font-medium">{{ formatDate(t.start_time) }}</p>
              <p class="text-[11px] font-mono" style="color: var(--color-text-secondary)">
                {{ Math.round(t.max_speed_kph / 1.60934) }} mph peak · {{ t.max_rpm.toLocaleString() }} rpm
              </p>
            </div>
            <svg class="w-4 h-4 opacity-30" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7" />
            </svg>
          </div>
        </div>
      </template>
      <DataEmptyState v-else title="No trips recorded" message="Trips will appear here once driving data is logged." />
    </div>
  </div>
</template>
