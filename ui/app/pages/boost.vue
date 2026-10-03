<script setup lang="ts">
import 'echarts'
import VChart from 'vue-echarts'

definePageMeta({ layout: 'default' })

interface BoostPoint {
  rpm: number
  boost_psi: number
}

interface Pull {
  boot_id: string
  start_ms: number
  end_ms: number
  duration_s: number
  min_rpm: number
  max_rpm: number
  max_speed_kph: number
  peak_boost_psi: number | null
  avg_lambda: number | null
  avg_stft: number | null
  avg_ltft: number | null
}

const { data: curveData, status: curveStatus } = useFetch<{ boostCurve: BoostPoint[] }>('/api/analytics/boost-curve')
const { data: pullsData, status: pullsStatus } = useFetch<{ pulls: Pull[] }>('/api/analytics/pulls')

const points = computed(() => curveData.value?.boostCurve ?? [])
const pulls = computed(() => pullsData.value?.pulls ?? [])

const peakBoost = computed(() => {
  if (!points.value.length) return null
  return points.value.reduce((m, p) => (p.boost_psi != null && p.boost_psi > m ? p.boost_psi : m), -Infinity)
})

const maxRpm = computed(() => {
  if (!pulls.value.length) return null
  return pulls.value.reduce((m, p) => Math.max(m, p.max_rpm), -Infinity)
})

const maxSpeed = computed(() => {
  if (!pulls.value.length) return null
  return pulls.value.reduce((m, p) => Math.max(m, p.max_speed_kph), -Infinity)
})

const chartOption = computed(() => {
  const pts = points.value.filter(p => p.boost_psi != null)
  return {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'item',
      backgroundColor: '#1a1d27',
      borderColor: '#2e3347',
      textStyle: { color: '#e8eaf0', fontSize: 13 },
      formatter: (params: any) => {
        const d = params.data
        return `RPM: <b>${d[0]}</b><br/>Boost: <b>${d[1].toFixed(1)} PSI</b>`
      },
    },
    grid: { top: 24, right: 24, bottom: 40, left: 56 },
    xAxis: {
      name: 'RPM', nameLocation: 'center', nameGap: 28,
      nameTextStyle: { color: '#8b90a0', fontSize: 12 },
      type: 'value', min: 1000, max: 7000,
      axisLabel: { color: '#8b90a0', fontSize: 11 },
      axisLine: { lineStyle: { color: '#2e3347' } },
      splitLine: { lineStyle: { color: '#2e3347' } },
    },
    yAxis: {
      name: 'PSI', nameTextStyle: { color: '#8b90a0', fontSize: 12 },
      type: 'value',
      axisLabel: { color: '#8b90a0', fontSize: 11 },
      axisLine: { lineStyle: { color: '#2e3347' } },
      splitLine: { lineStyle: { color: '#2e3347' } },
    },
    series: [{
      type: 'scatter',
      data: pts.map(p => [p.rpm, p.boost_psi]),
      symbolSize: 6,
      itemStyle: { color: '#8b5cf6', opacity: 0.7 },
    }],
  }
})

const loading = computed(() => curveStatus.value === 'pending' || pullsStatus.value === 'pending')
</script>

<template>
  <div>
    <LayoutPageHeader title="Boost & Power" subtitle="Turbo performance analysis" />

    <div class="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-8">
      <template v-if="loading">
        <div v-for="i in 3" :key="i" class="rounded-xl p-5 skeleton h-[100px]"
          :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }" />
      </template>
      <template v-else>
        <DataStatCard label="Peak Boost" :value="peakBoost != null ? `${peakBoost.toFixed(1)} PSI` : '--'" />
        <DataStatCard label="Max RPM" :value="maxRpm != null ? maxRpm.toLocaleString() : '--'" />
        <DataStatCard label="Max Speed" :value="maxSpeed != null ? `${Math.round(maxSpeed)} kph` : '--'" />
      </template>
    </div>

    <div class="rounded-xl p-6 mb-6"
      :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }">
      <h2 class="text-base font-semibold mb-4">Boost Curve — RPM vs Boost PSI</h2>

      <template v-if="curveStatus === 'pending'">
        <div class="flex items-center justify-center" style="height: 400px">
          <div class="spinner" />
        </div>
      </template>

      <DataEmptyState v-else-if="!points.length" title="No boost data"
        message="Boost curve data will appear here once WOT pulls are recorded." />

      <VChart v-else :option="chartOption" autoresize style="height: 400px; width: 100%" />
    </div>

    <div class="rounded-xl p-6"
      :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }">
      <h2 class="text-base font-semibold mb-4">WOT Pulls</h2>

      <template v-if="pullsStatus === 'pending'">
        <div class="flex items-center justify-center py-12">
          <div class="spinner" />
        </div>
      </template>

      <DataEmptyState v-else-if="!pulls.length" title="No pulls recorded"
        message="WOT pull data will appear here once full-throttle events are detected." />

      <div v-else class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr :style="{ borderBottom: '1px solid var(--color-border)' }">
              <th class="text-left px-5 py-3 text-[11px] font-semibold uppercase tracking-wider" style="color: var(--color-text-secondary)">RPM Range</th>
              <th class="text-right px-5 py-3 text-[11px] font-semibold uppercase tracking-wider" style="color: var(--color-text-secondary)">Peak Boost</th>
              <th class="text-right px-5 py-3 text-[11px] font-semibold uppercase tracking-wider" style="color: var(--color-text-secondary)">Max Speed</th>
              <th class="text-right px-5 py-3 text-[11px] font-semibold uppercase tracking-wider" style="color: var(--color-text-secondary)">Duration</th>
              <th class="text-right px-5 py-3 text-[11px] font-semibold uppercase tracking-wider hidden sm:table-cell" style="color: var(--color-text-secondary)">Lambda</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="pull in pulls" :key="pull.boot_id + pull.start_ms"
              :style="{ borderBottom: '1px solid var(--color-border)' }">
              <td class="px-5 py-3.5 font-mono">{{ pull.min_rpm }} → {{ pull.max_rpm }}</td>
              <td class="px-5 py-3.5 font-mono text-right">{{ pull.peak_boost_psi != null ? pull.peak_boost_psi.toFixed(1) + ' PSI' : '--' }}</td>
              <td class="px-5 py-3.5 font-mono text-right">{{ Math.round(pull.max_speed_kph) }} kph</td>
              <td class="px-5 py-3.5 font-mono text-right">{{ pull.duration_s.toFixed(1) }}s</td>
              <td class="px-5 py-3.5 font-mono text-right hidden sm:table-cell">{{ pull.avg_lambda != null ? pull.avg_lambda.toFixed(2) : '--' }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>
