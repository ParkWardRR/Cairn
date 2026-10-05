<script setup lang="ts">
import 'echarts'
import VChart from 'vue-echarts'

// GPS health for a trip: satellites in use over time, shaded where there was a
// fix. The trip's scrubber lives in TripScrubber, directly under the map.

interface GPSSample {
  mono_ms: number
  lat: number
  lon: number
  sats_used: number | null
}

const props = defineProps<{
  bootId: string
  firstObdMs?: number | null
}>()

const { data: timeline } = useFetch<{ gps: GPSSample[] }>(
  () => `/api/trips/${props.bootId}/timeline`,
)

const gps = computed(() => timeline.value?.gps ?? [])
const gpsWithFix = computed(() => gps.value.filter(g => g.lat !== 0 && g.lon !== 0))

function fmtMs(ms: number): string {
  const sec = ms / 1000
  if (sec < 60) return `${Math.round(sec)}s`
  return `${Math.floor(sec / 60)}m ${Math.round(sec % 60)}s`
}

const satsChartOption = computed(() => {
  if (!gps.value.length) return {}
  const startMs = props.firstObdMs ?? gps.value[0]?.mono_ms ?? 0
  const data = gps.value.map(g => [
    Math.round((g.mono_ms - startMs) / 1000),
    g.sats_used ?? 0,
  ])
  const fixData = gps.value.map(g => [
    Math.round((g.mono_ms - startMs) / 1000),
    (g.lat !== 0 && g.lon !== 0) ? 1 : 0,
  ])

  return {
    backgroundColor: 'transparent',
    grid: { top: 8, right: 8, bottom: 24, left: 32 },
    tooltip: {
      trigger: 'axis',
      backgroundColor: '#1a1d27',
      borderColor: '#2e3347',
      textStyle: { color: '#e8eaf0', fontSize: 11 },
      formatter: (p: any) => {
        const sec = p[0]?.value?.[0] ?? 0
        const sats = p[0]?.value?.[1] ?? 0
        const fix = p[1]?.value?.[1] ?? 0
        return `${fmtMs(sec * 1000)} into trip<br>Satellites: ${sats}<br>${fix ? '3D fix' : 'No fix'}`
      },
    },
    xAxis: {
      type: 'value',
      min: 0,
      axisLabel: { color: '#8b90a0', fontSize: 10, formatter: (v: number) => `${Math.floor(v / 60)}m` },
      axisLine: { lineStyle: { color: '#2e3347' } },
      splitLine: { show: false },
    },
    yAxis: {
      type: 'value',
      min: 0,
      max: 20,
      axisLabel: { color: '#8b90a0', fontSize: 10 },
      axisLine: { lineStyle: { color: '#2e3347' } },
      splitLine: { lineStyle: { color: '#2e3347', opacity: 0.3 } },
    },
    series: [
      {
        type: 'line',
        data,
        symbol: 'none',
        lineStyle: { color: '#22c55e', width: 1.5 },
        areaStyle: { color: 'rgba(34, 197, 94, 0.08)' },
      },
      {
        type: 'line',
        data: fixData,
        symbol: 'none',
        lineStyle: { width: 0 },
        areaStyle: { color: 'rgba(34, 197, 94, 0.15)', origin: 'start' },
        yAxisIndex: 0,
        silent: true,
      },
    ],
  }
})
</script>

<template>
  <div class="rounded-xl p-4" :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }">
    <div class="flex items-center justify-between mb-2">
      <div class="flex items-center gap-2">
        <svg class="w-4 h-4" style="color: #22c55e" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.75">
          <path stroke-linecap="round" stroke-linejoin="round" d="M17.657 16.657L13.414 20.9a1.998 1.998 0 01-2.827 0l-4.244-4.243a8 8 0 1111.314 0z M15 11a3 3 0 11-6 0 3 3 0 016 0z" />
        </svg>
        <span class="text-[11px] font-semibold uppercase tracking-wider" style="color: var(--color-text-secondary)">GPS Health</span>
      </div>
      <span class="text-[11px] font-mono" style="color: var(--color-text-secondary)">
        {{ gpsWithFix.length }} fixes · {{ gps.length }} samples
      </span>
    </div>

    <template v-if="gps.length > 0">
      <VChart :option="satsChartOption" style="height: 80px; width: 100%" autoresize />
    </template>
    <div v-else class="flex items-center justify-center" style="height: 80px">
      <span class="text-[11px]" style="color: var(--color-text-secondary)">No GPS data</span>
    </div>
  </div>
</template>
