<script setup lang="ts">
import 'echarts'
import VChart from 'vue-echarts'

definePageMeta({ layout: 'default' })

interface SpeedPoint {
  boot_id: string
  mono_ms: number
  obd_speed_kph: number
  gnss_speed_kph: number
  ratio: number | null
  gnss_age_ms: number
}

const { data, pending } = useFetch<{ speedAgreement: SpeedPoint[] }>('/api/analytics/speed-agreement')

const points = computed(() => {
  return (data.value?.speedAgreement ?? []).filter(p => p.ratio != null && p.obd_speed_kph > 5)
})

const medianRatio = computed(() => {
  const ratios = points.value.map(p => p.ratio!).sort((a, b) => a - b)
  if (!ratios.length) return null
  const mid = Math.floor(ratios.length / 2)
  return ratios.length % 2 === 0 ? (ratios[mid - 1] + ratios[mid]) / 2 : ratios[mid]
})

function ratioColor(ratio: number | null): 'success' | 'warning' | 'danger' {
  if (ratio == null) return 'danger'
  const dev = Math.abs(ratio - 1)
  if (dev <= 0.03) return 'success'
  if (dev <= 0.05) return 'warning'
  return 'danger'
}

function ratioLabel(ratio: number | null): string {
  if (ratio == null) return '--'
  const pct = ((ratio - 1) * 100).toFixed(1)
  if (ratio > 1.01) return `OBD reads ${pct}% high`
  if (ratio < 0.99) return `OBD reads ${Math.abs(+pct).toFixed(1)}% low`
  return 'Well calibrated'
}

const chartOption = computed(() => {
  const scatterData = points.value.map(p => [
    Math.round(p.obd_speed_kph / 1.60934),
    Math.round(p.gnss_speed_kph / 1.60934),
  ])

  const maxMph = 100
  return {
    backgroundColor: 'transparent',
    grid: { left: 60, right: 30, top: 30, bottom: 50 },
    tooltip: {
      trigger: 'item',
      backgroundColor: '#1a1d27',
      borderColor: '#2e3347',
      textStyle: { color: '#e8eaf0', fontSize: 12 },
      formatter: (params: any) => {
        const [obd, gnss] = params.value
        const ratio = gnss > 0 ? (obd / gnss).toFixed(3) : '--'
        return `OBD: ${obd} mph<br>GNSS: ${gnss} mph<br>Ratio: ${ratio}`
      },
    },
    xAxis: {
      name: 'OBD Speed (mph)',
      nameLocation: 'middle',
      nameGap: 35,
      nameTextStyle: { color: '#8b90a0', fontSize: 12 },
      min: 0,
      max: maxMph,
      axisLabel: { color: '#8b90a0' },
      axisLine: { lineStyle: { color: '#2e3347' } },
      splitLine: { lineStyle: { color: '#2e3347' } },
    },
    yAxis: {
      name: 'GNSS Speed (mph)',
      nameLocation: 'middle',
      nameGap: 45,
      nameTextStyle: { color: '#8b90a0', fontSize: 12 },
      min: 0,
      max: maxMph,
      axisLabel: { color: '#8b90a0' },
      axisLine: { lineStyle: { color: '#2e3347' } },
      splitLine: { lineStyle: { color: '#2e3347' } },
    },
    series: [
      {
        type: 'scatter',
        data: scatterData,
        symbolSize: 5,
        itemStyle: { color: '#3b82f6', opacity: 0.5 },
        markLine: {
          silent: true,
          symbol: 'none',
          lineStyle: { color: '#22c55e', type: 'dashed', width: 1.5 },
          label: { show: false },
          data: [[{ coord: [0, 0] }, { coord: [maxMph, maxMph] }]],
        },
      },
    ],
  }
})
</script>

<template>
  <div>
    <LayoutPageHeader title="Speedometer Calibration" subtitle="OBD vs GNSS speed agreement">
      <template #actions>
        <span v-if="points.length" class="text-xs font-medium px-2.5 py-1 rounded-full" style="background: var(--color-accent-soft); color: var(--color-accent)">
          {{ points.length }} samples
        </span>
      </template>
    </LayoutPageHeader>

    <template v-if="pending">
      <div class="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-8">
        <div v-for="i in 3" :key="i" class="skeleton h-[100px]" />
      </div>
    </template>

    <template v-else>
      <!-- Stat cards -->
      <div class="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-8">
        <DataStatCard
          label="Median Ratio"
          :value="medianRatio != null ? medianRatio.toFixed(3) : '--'"
          subtitle="OBD / GNSS ratio"
          :color="ratioColor(medianRatio)"
        />
        <DataStatCard
          label="Agreement"
          :value="ratioLabel(medianRatio)"
          :subtitle="medianRatio != null ? `based on ${points.length} samples` : 'no data'"
        />
        <DataStatCard
          label="Data Points"
          :value="String(points.length)"
          subtitle="above 3 mph"
        />
      </div>

      <!-- Scatter chart -->
      <div class="rounded-xl p-5 mb-6" :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }">
        <h2 class="text-[13px] font-semibold mb-4">Speed Agreement — OBD vs GNSS</h2>

        <template v-if="points.length > 0">
          <VChart :option="chartOption" autoresize style="height: 500px; width: 100%" />
        </template>
        <DataEmptyState v-else title="No speed data" message="Speed agreement data will appear once trips with both OBD and GNSS readings are recorded." />
      </div>

      <!-- Explanation card -->
      <div class="rounded-xl p-5" :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }">
        <h3 class="text-[11px] font-semibold uppercase tracking-wider mb-2" style="color: var(--color-text-secondary)">How to read this</h3>
        <p class="text-[13px] leading-relaxed" style="color: var(--color-text-secondary)">
          Points along the green diagonal indicate perfect agreement between OBD and GNSS speeds.
          Points above the line mean OBD reads higher than actual speed.
          A median ratio of 1.000 means perfect calibration; values above 1.0 indicate the
          speedometer reads fast (common on modified cars with different tire sizes or ECU tunes).
        </p>
      </div>
    </template>
  </div>
</template>
