<script setup lang="ts">
import 'echarts'
import VChart from 'vue-echarts'

interface DriveSummary {
  boot_id: string
  duration_s: number
  max_speed_kph: number
  max_rpm: number
  obd_samples: number
}

const selectedBootId = ref<string>('')

const { data: drivesData, status: drivesStatus } = useFetch<{ driveSummary: DriveSummary[] }>('/api/analytics/drive-summary')
const drives = computed(() => drivesData.value?.driveSummary ?? [])

const { data: telemetryData, status: telemetryStatus } = useFetch<{ telemetry: any[] }>(
  () => `/api/analytics/telemetry?boot_id=${selectedBootId.value}`,
  { watch: [selectedBootId], immediate: false },
)

watch(selectedBootId, (val) => {
  if (!val) telemetryData.value = null
})

const telemetryRows = computed(() => telemetryData.value?.telemetry ?? [])

function formatDriveLabel(d: DriveSummary): string {
  return `${d.boot_id.slice(0, 8)}… — ${Math.round(d.duration_s / 60)}m, ${d.max_speed_kph} kph`
}

function formatTimestamp(monoMs: number, baseMs: number): string {
  const totalSeconds = Math.round((monoMs - baseMs) / 1000)
  const m = Math.floor(totalSeconds / 60)
  const s = totalSeconds % 60
  return `${m}:${String(s).padStart(2, '0')}`
}

const baseMs = computed(() => telemetryRows.value[0]?.mono_ms ?? 0)

const timeLabels = computed(() =>
  telemetryRows.value.map((r: any) => formatTimestamp(r.mono_ms, baseMs.value)),
)

const tooltipStyle = {
  backgroundColor: '#1a1d27',
  borderColor: '#2e3347',
  textStyle: { color: '#e8eaf0' },
  trigger: 'axis' as const,
}

const axisLabelStyle = { color: '#8b90a0' }
const splitLineStyle = { lineStyle: { color: '#2e3347' } }

function baseGrid() {
  return { top: 40, right: 60, bottom: 60, left: 60, containLabel: false }
}

function baseXAxis() {
  return {
    type: 'category' as const,
    data: timeLabels.value,
    axisLabel: axisLabelStyle,
    splitLine: splitLineStyle,
  }
}

function baseDataZoom() {
  return [
    {
      type: 'slider' as const,
      xAxisIndex: 0,
      height: 20,
      bottom: 8,
      borderColor: '#2e3347',
      backgroundColor: '#1a1d27',
      fillerColor: 'rgba(59,130,246,0.15)',
      handleStyle: { color: '#3b82f6' },
      textStyle: { color: '#8b90a0' },
      dataBackground: {
        lineStyle: { color: '#2e3347' },
        areaStyle: { color: '#2e3347' },
      },
    },
    { type: 'inside' as const, xAxisIndex: 0 },
  ]
}

function pluck(key: string) {
  return telemetryRows.value.map((r: any) => r[key] ?? null)
}

const speedOptions = computed(() => ({
  backgroundColor: 'transparent',
  animation: false,
  tooltip: tooltipStyle,
  grid: baseGrid(),
  xAxis: baseXAxis(),
  yAxis: { type: 'value', name: 'km/h', nameTextStyle: axisLabelStyle, axisLabel: axisLabelStyle, splitLine: splitLineStyle },
  dataZoom: baseDataZoom(),
  series: [
    { name: 'Speed', type: 'line', data: pluck('speed_kph'), showSymbol: false, lineStyle: { color: '#3b82f6', width: 1.5 }, itemStyle: { color: '#3b82f6' } },
  ],
}))

const rpmThrottleOptions = computed(() => ({
  backgroundColor: 'transparent',
  animation: false,
  tooltip: tooltipStyle,
  grid: baseGrid(),
  xAxis: baseXAxis(),
  yAxis: [
    { type: 'value', name: 'RPM', nameTextStyle: axisLabelStyle, axisLabel: axisLabelStyle, splitLine: splitLineStyle },
    { type: 'value', name: 'Throttle %', nameTextStyle: axisLabelStyle, axisLabel: axisLabelStyle, splitLine: { show: false } },
  ],
  dataZoom: baseDataZoom(),
  series: [
    { name: 'RPM', type: 'line', yAxisIndex: 0, data: pluck('rpm'), showSymbol: false, lineStyle: { color: '#f59e0b', width: 1.5 }, itemStyle: { color: '#f59e0b' } },
    { name: 'Throttle', type: 'line', yAxisIndex: 1, data: pluck('throttle_pct'), showSymbol: false, lineStyle: { color: '#22c55e', width: 1.5 }, itemStyle: { color: '#22c55e' } },
  ],
}))

const boostOptions = computed(() => ({
  backgroundColor: 'transparent',
  animation: false,
  tooltip: tooltipStyle,
  grid: baseGrid(),
  xAxis: baseXAxis(),
  yAxis: { type: 'value', name: 'PSI', nameTextStyle: axisLabelStyle, axisLabel: axisLabelStyle, splitLine: splitLineStyle },
  dataZoom: baseDataZoom(),
  series: [
    {
      name: 'Boost', type: 'line', data: pluck('boost_psi'), showSymbol: false,
      lineStyle: { color: '#8b5cf6', width: 1.5 }, itemStyle: { color: '#8b5cf6' },
      markLine: {
        silent: true, symbol: 'none',
        lineStyle: { color: '#8b90a0', type: 'dashed' as const, width: 1 },
        data: [{ yAxis: 0, label: { show: true, formatter: '0 PSI', color: '#8b90a0', position: 'end' as const } }],
      },
    },
  ],
}))

const lambdaTrimsOptions = computed(() => ({
  backgroundColor: 'transparent',
  animation: false,
  tooltip: tooltipStyle,
  grid: baseGrid(),
  xAxis: baseXAxis(),
  yAxis: { type: 'value', axisLabel: axisLabelStyle, splitLine: splitLineStyle },
  dataZoom: baseDataZoom(),
  series: [
    { name: 'Lambda', type: 'line', data: pluck('lambda_ratio'), showSymbol: false, lineStyle: { color: '#06b6d4', width: 1.5 }, itemStyle: { color: '#06b6d4' },
      markLine: { silent: true, symbol: 'none', lineStyle: { color: '#8b90a0', type: 'dashed' as const, width: 1 },
        data: [{ yAxis: 1.0, label: { show: true, formatter: 'Lambda 1.0', color: '#8b90a0', position: 'end' as const } }] } },
    { name: 'STFT', type: 'line', data: pluck('stft_pct'), showSymbol: false, lineStyle: { color: '#f59e0b', width: 1.5 }, itemStyle: { color: '#f59e0b' } },
    { name: 'LTFT', type: 'line', data: pluck('ltft_pct'), showSymbol: false, lineStyle: { color: '#ef4444', width: 1.5 }, itemStyle: { color: '#ef4444' } },
  ],
}))

const temperaturesOptions = computed(() => ({
  backgroundColor: 'transparent',
  animation: false,
  tooltip: tooltipStyle,
  grid: baseGrid(),
  xAxis: baseXAxis(),
  yAxis: { type: 'value', name: '°C', nameTextStyle: axisLabelStyle, axisLabel: axisLabelStyle, splitLine: splitLineStyle },
  dataZoom: baseDataZoom(),
  series: [
    { name: 'Coolant', type: 'line', data: pluck('coolant_c'), showSymbol: false, lineStyle: { color: '#ef4444', width: 1.5 }, itemStyle: { color: '#ef4444' } },
    { name: 'Intake', type: 'line', data: pluck('intake_c'), showSymbol: false, lineStyle: { color: '#3b82f6', width: 1.5 }, itemStyle: { color: '#3b82f6' } },
  ],
}))
</script>

<template>
  <div>
    <LayoutPageHeader title="Engine Analytics" subtitle="Deep-dive telemetry viewer" />

    <div class="mb-6">
      <select
        v-model="selectedBootId"
        class="w-full max-w-md px-3 py-2 rounded-md border font-sans text-sm appearance-none cursor-pointer"
        :style="{ backgroundColor: 'var(--color-surface)', borderColor: 'var(--color-border)', color: 'var(--color-text)' }"
      >
        <option value="" disabled>Select a trip...</option>
        <option v-for="d in drives" :key="d.boot_id" :value="d.boot_id">
          {{ formatDriveLabel(d) }}
        </option>
      </select>

      <div v-if="drivesStatus === 'pending'" class="text-sm mt-2" :style="{ color: 'var(--color-text-secondary)' }">
        Loading trips...
      </div>
    </div>

    <DataEmptyState v-if="!selectedBootId" title="No trip selected" message="Select a trip to view telemetry" />

    <div v-else-if="telemetryStatus === 'pending'" class="flex items-center justify-center py-16">
      <div class="w-6 h-6 rounded-full border-2 animate-spin"
        :style="{ borderColor: 'var(--color-border)', borderTopColor: 'var(--color-accent)' }" />
    </div>

    <div v-else-if="telemetryRows.length" class="flex flex-col gap-4">
      <div class="rounded-lg border p-4" :style="{ backgroundColor: 'var(--color-surface)', borderColor: 'var(--color-border)' }">
        <h3 class="text-sm font-medium mb-2" :style="{ color: 'var(--color-text)' }">Speed</h3>
        <v-chart :option="speedOptions" autoresize style="height: 250px; width: 100%" />
      </div>

      <div class="rounded-lg border p-4" :style="{ backgroundColor: 'var(--color-surface)', borderColor: 'var(--color-border)' }">
        <h3 class="text-sm font-medium mb-2" :style="{ color: 'var(--color-text)' }">RPM &amp; Throttle</h3>
        <v-chart :option="rpmThrottleOptions" autoresize style="height: 250px; width: 100%" />
      </div>

      <div class="rounded-lg border p-4" :style="{ backgroundColor: 'var(--color-surface)', borderColor: 'var(--color-border)' }">
        <h3 class="text-sm font-medium mb-2" :style="{ color: 'var(--color-text)' }">Boost</h3>
        <v-chart :option="boostOptions" autoresize style="height: 250px; width: 100%" />
      </div>

      <div class="rounded-lg border p-4" :style="{ backgroundColor: 'var(--color-surface)', borderColor: 'var(--color-border)' }">
        <h3 class="text-sm font-medium mb-2" :style="{ color: 'var(--color-text)' }">Lambda &amp; Trims</h3>
        <v-chart :option="lambdaTrimsOptions" autoresize style="height: 250px; width: 100%" />
      </div>

      <div class="rounded-lg border p-4" :style="{ backgroundColor: 'var(--color-surface)', borderColor: 'var(--color-border)' }">
        <h3 class="text-sm font-medium mb-2" :style="{ color: 'var(--color-text)' }">Temperatures</h3>
        <v-chart :option="temperaturesOptions" autoresize style="height: 250px; width: 100%" />
      </div>
    </div>
  </div>
</template>
