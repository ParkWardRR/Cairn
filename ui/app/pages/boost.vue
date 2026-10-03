<script setup lang="ts">
import 'echarts'
import VChart from 'vue-echarts'

definePageMeta({ layout: 'default' })

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

interface CurvePoint {
  rpm: number
  boost_psi: number
  intake_c: number | null
  timing_advance_deg: number | null
  lambda_ratio: number | null
}

interface TimingPoint { rpm: number; timing_advance_deg: number; boost_psi: number }
interface IATBoostPoint { boost_psi: number; intake_c: number; rpm: number }

const { data: detail } = useFetch<{
  curve: CurvePoint[]
  timing: TimingPoint[]
  iatBoost: IATBoostPoint[]
}>('/api/analytics/boost-detail')
const { data: pullsData } = useFetch<{ pulls: Pull[] }>('/api/analytics/pulls')

const curve = computed(() => detail.value?.curve ?? [])
const timing = computed(() => detail.value?.timing ?? [])
const iatBoost = computed(() => detail.value?.iatBoost ?? [])
const pulls = computed(() => pullsData.value?.pulls ?? [])

const peakBoost = computed(() => {
  if (!curve.value.length) return null
  return curve.value.reduce((m, p) => (p.boost_psi != null && p.boost_psi > m ? p.boost_psi : m), -Infinity)
})
const maxRpm = computed(() => {
  if (!pulls.value.length) return null
  return pulls.value.reduce((m, p) => Math.max(m, p.max_rpm), -Infinity)
})
const maxSpeed = computed(() => {
  if (!pulls.value.length) return null
  return Math.round(pulls.value.reduce((m, p) => Math.max(m, p.max_speed_kph), -Infinity) / 1.60934)
})
const peakIAT = computed(() => {
  const pts = iatBoost.value.filter(p => p.boost_psi > 5)
  if (!pts.length) return null
  return pts.reduce((m, p) => Math.max(m, p.intake_c), -Infinity)
})

function iatColor(c: number): string {
  if (c < 30) return '#22c55e'
  if (c < 35) return '#84cc16'
  if (c < 40) return '#eab308'
  if (c < 45) return '#f97316'
  return '#ef4444'
}

const boostChartOption = computed(() => {
  const pts = curve.value.filter(p => p.boost_psi != null && p.rpm != null)
  return {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'item',
      backgroundColor: '#1a1d27', borderColor: '#2e3347',
      textStyle: { color: '#e8eaf0', fontSize: 12 },
      formatter: (p: any) => {
        const d = p.data
        let html = `RPM: <b>${d[0]}</b><br>Boost: <b>${d[1].toFixed(1)} psi</b>`
        if (d[2] != null) html += `<br>IAT: <b>${Math.round(d[2] * 9/5 + 32)}°F</b> (${d[2]}°C)`
        return html
      },
    },
    grid: { top: 24, right: 24, bottom: 48, left: 56 },
    xAxis: {
      name: 'RPM', nameLocation: 'center', nameGap: 32,
      nameTextStyle: { color: '#8b90a0', fontSize: 12 },
      type: 'value', min: 1000, max: 7000,
      axisLabel: { color: '#8b90a0', fontSize: 11 },
      axisLine: { lineStyle: { color: '#2e3347' } },
      splitLine: { lineStyle: { color: '#2e3347', opacity: 0.3 } },
    },
    yAxis: {
      name: 'PSI', nameTextStyle: { color: '#8b90a0', fontSize: 12 },
      type: 'value',
      axisLabel: { color: '#8b90a0', fontSize: 11 },
      axisLine: { lineStyle: { color: '#2e3347' } },
      splitLine: { lineStyle: { color: '#2e3347', opacity: 0.3 } },
    },
    visualMap: {
      show: true, orient: 'vertical', right: 0, top: 'center',
      min: 25, max: 50, text: ['Hot', 'Cool'],
      inRange: { color: ['#22c55e', '#84cc16', '#eab308', '#f97316', '#ef4444'] },
      textStyle: { color: '#8b90a0', fontSize: 10 },
      dimension: 2,
    },
    series: [{
      type: 'scatter',
      data: pts.map(p => [p.rpm, p.boost_psi, p.intake_c ?? 30]),
      symbolSize: 7,
      itemStyle: { opacity: 0.8 },
    }],
  }
})

const timingChartOption = computed(() => {
  const pts = timing.value
  if (!pts.length) return {}
  return {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'item',
      backgroundColor: '#1a1d27', borderColor: '#2e3347',
      textStyle: { color: '#e8eaf0', fontSize: 12 },
      formatter: (p: any) => `RPM: <b>${p.data[0]}</b><br>Timing: <b>${p.data[1]}°</b><br>Boost: <b>${p.data[2].toFixed(1)} psi</b>`,
    },
    grid: { top: 24, right: 24, bottom: 48, left: 56 },
    xAxis: {
      name: 'RPM', nameLocation: 'center', nameGap: 32,
      nameTextStyle: { color: '#8b90a0', fontSize: 12 },
      type: 'value', min: 1000, max: 7000,
      axisLabel: { color: '#8b90a0', fontSize: 11 },
      axisLine: { lineStyle: { color: '#2e3347' } },
      splitLine: { lineStyle: { color: '#2e3347', opacity: 0.3 } },
    },
    yAxis: {
      name: 'Timing °', nameTextStyle: { color: '#8b90a0', fontSize: 12 },
      type: 'value',
      axisLabel: { color: '#8b90a0', fontSize: 11 },
      axisLine: { lineStyle: { color: '#2e3347' } },
      splitLine: { lineStyle: { color: '#2e3347', opacity: 0.3 } },
    },
    visualMap: {
      show: true, orient: 'vertical', right: 0, top: 'center',
      min: 0, max: 22, text: ['High', 'Low'],
      inRange: { color: ['#3b82f6', '#8b5cf6', '#f59e0b', '#ef4444'] },
      textStyle: { color: '#8b90a0', fontSize: 10 },
      dimension: 2,
    },
    series: [{
      type: 'scatter',
      data: pts.map(p => [p.rpm, p.timing_advance_deg, p.boost_psi]),
      symbolSize: 6,
      itemStyle: { opacity: 0.7 },
    }],
  }
})

const iatBoostChartOption = computed(() => {
  const pts = iatBoost.value
  if (!pts.length) return {}
  return {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'item',
      backgroundColor: '#1a1d27', borderColor: '#2e3347',
      textStyle: { color: '#e8eaf0', fontSize: 12 },
      formatter: (p: any) => `Boost: <b>${p.data[0].toFixed(1)} psi</b><br>IAT: <b>${Math.round(p.data[1] * 9/5 + 32)}°F</b> (${p.data[1]}°C)<br>RPM: <b>${p.data[2]}</b>`,
    },
    grid: { top: 24, right: 24, bottom: 48, left: 56 },
    xAxis: {
      name: 'Boost (PSI)', nameLocation: 'center', nameGap: 32,
      nameTextStyle: { color: '#8b90a0', fontSize: 12 },
      type: 'value',
      axisLabel: { color: '#8b90a0', fontSize: 11 },
      axisLine: { lineStyle: { color: '#2e3347' } },
      splitLine: { lineStyle: { color: '#2e3347', opacity: 0.3 } },
    },
    yAxis: {
      name: 'IAT °F', nameTextStyle: { color: '#8b90a0', fontSize: 12 },
      type: 'value',
      axisLabel: { color: '#8b90a0', fontSize: 11, formatter: (v: number) => `${Math.round(v * 9/5 + 32)}` },
      axisLine: { lineStyle: { color: '#2e3347' } },
      splitLine: { lineStyle: { color: '#2e3347', opacity: 0.3 } },
      min: 'dataMin', max: 'dataMax',
    },
    series: [{
      type: 'scatter',
      data: pts.map(p => [p.boost_psi, p.intake_c, p.rpm]),
      symbolSize: 8,
      itemStyle: {
        color: (p: any) => iatColor(p.data[1]),
        opacity: 0.8,
      },
    }],
    markLine: {
      data: [{ yAxis: 40, label: { show: true, formatter: '104°F limit', position: 'end', color: '#ef4444', fontSize: 10 }, lineStyle: { color: '#ef4444', type: 'dashed', opacity: 0.5 } }],
    },
  }
})
</script>

<template>
  <div>
    <LayoutPageHeader title="Boost & Power" subtitle="Turbo performance and intake temperature analysis" />

    <div class="grid grid-cols-2 lg:grid-cols-4 gap-3 md:gap-4 mb-8 sticky top-0 z-10 py-3 -mt-3" style="background: var(--color-background)">
      <DataStatCard label="Peak Boost" :value="peakBoost != null && peakBoost > -Infinity ? `${peakBoost.toFixed(1)}` : '--'" subtitle="psi all time"
        :color="(peakBoost ?? 0) > 20 ? 'warning' : undefined" />
      <DataStatCard label="Max RPM" :value="maxRpm != null && maxRpm > -Infinity ? maxRpm.toLocaleString() : '--'" subtitle="under boost" />
      <DataStatCard label="Top Speed" :value="maxSpeed != null && maxSpeed > -Infinity ? `${maxSpeed}` : '--'" subtitle="mph in pulls" />
      <DataStatCard label="Peak IAT" :value="peakIAT != null ? `${Math.round(peakIAT * 9/5 + 32)}°F` : '--'" subtitle="under boost"
        :color="(peakIAT ?? 0) > 40 ? 'warning' : (peakIAT ?? 0) > 45 ? 'danger' : undefined" />
    </div>

    <!-- Boost Curve by IAT -->
    <div class="rounded-xl p-6 mb-4" :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }">
      <div class="flex items-center justify-between mb-4">
        <h2 class="text-base font-semibold">Boost Curve</h2>
        <span class="text-[11px]" style="color: var(--color-text-secondary)">Colored by intake air temp</span>
      </div>
      <DataEmptyState v-if="!curve.length" title="No boost data" message="Boost data will appear once WOT events are recorded." />
      <VChart v-else :option="boostChartOption" autoresize style="height: 380px; width: 100%" />
    </div>

    <!-- IAT under Boost + Timing side by side -->
    <div class="grid grid-cols-1 lg:grid-cols-2 gap-4 mb-4">
      <div class="rounded-xl p-6" :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }">
        <div class="flex items-center justify-between mb-4">
          <h2 class="text-base font-semibold">IAT vs Boost</h2>
          <span class="text-[11px]" style="color: var(--color-text-secondary)">Higher boost → hotter intake</span>
        </div>
        <DataEmptyState v-if="!iatBoost.length" title="No data" message="Need boost + IAT samples." />
        <VChart v-else :option="iatBoostChartOption" autoresize style="height: 280px; width: 100%" />
      </div>

      <div class="rounded-xl p-6" :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }">
        <div class="flex items-center justify-between mb-4">
          <h2 class="text-base font-semibold">Timing Under Boost</h2>
          <span class="text-[11px]" style="color: var(--color-text-secondary)">Colored by boost pressure</span>
        </div>
        <DataEmptyState v-if="!timing.length" title="No data" message="Need timing + boost samples." />
        <VChart v-else :option="timingChartOption" autoresize style="height: 280px; width: 100%" />
      </div>
    </div>

    <!-- WOT Pulls -->
    <div class="rounded-xl p-6" :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }">
      <h2 class="text-base font-semibold mb-4">WOT Pulls</h2>
      <DataEmptyState v-if="!pulls.length" title="No pulls recorded" message="WOT pulls will appear once full-throttle events are detected." />
      <div v-else class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr :style="{ borderBottom: '1px solid var(--color-border)' }">
              <th class="text-left px-4 py-3 text-[11px] font-semibold uppercase tracking-wider" style="color: var(--color-text-secondary)">RPM Range</th>
              <th class="text-right px-4 py-3 text-[11px] font-semibold uppercase tracking-wider" style="color: var(--color-text-secondary)">Peak Boost</th>
              <th class="text-right px-4 py-3 text-[11px] font-semibold uppercase tracking-wider" style="color: var(--color-text-secondary)">Speed</th>
              <th class="text-right px-4 py-3 text-[11px] font-semibold uppercase tracking-wider" style="color: var(--color-text-secondary)">Duration</th>
              <th class="text-right px-4 py-3 text-[11px] font-semibold uppercase tracking-wider hidden md:table-cell" style="color: var(--color-text-secondary)">Lambda</th>
              <th class="text-right px-4 py-3 text-[11px] font-semibold uppercase tracking-wider hidden md:table-cell" style="color: var(--color-text-secondary)">LTFT</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="pull in pulls" :key="pull.boot_id + pull.start_ms" :style="{ borderBottom: '1px solid var(--color-border)' }">
              <td class="px-4 py-3 font-mono">{{ pull.min_rpm.toLocaleString() }} → {{ pull.max_rpm.toLocaleString() }}</td>
              <td class="px-4 py-3 font-mono text-right" :style="{ color: (pull.peak_boost_psi ?? 0) > 18 ? '#f59e0b' : 'inherit' }">
                {{ pull.peak_boost_psi != null ? pull.peak_boost_psi.toFixed(1) + ' psi' : '--' }}
              </td>
              <td class="px-4 py-3 font-mono text-right">{{ Math.round(pull.max_speed_kph / 1.60934) }} mph</td>
              <td class="px-4 py-3 font-mono text-right">{{ pull.duration_s.toFixed(1) }}s</td>
              <td class="px-4 py-3 font-mono text-right hidden md:table-cell" :style="{ color: (pull.avg_lambda ?? 1) < 0.9 ? '#22c55e' : 'inherit' }">
                {{ pull.avg_lambda != null ? pull.avg_lambda.toFixed(3) : '--' }}
              </td>
              <td class="px-4 py-3 font-mono text-right hidden md:table-cell">{{ pull.avg_ltft != null ? `${pull.avg_ltft > 0 ? '+' : ''}${pull.avg_ltft.toFixed(0)}%` : '--' }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>
