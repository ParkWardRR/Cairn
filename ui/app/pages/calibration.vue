<script setup lang="ts">
import 'echarts'
import VChart from 'vue-echarts'

definePageMeta({ layout: 'default' })

// ---------------------------------------------------------------------------
// Data fetching
// ---------------------------------------------------------------------------

const { data, pending } = useFetch<{
  points: Array<{ obd_kph: number; gnss_kph: number }>
  median_ratio: number
  point_count: number
}>('/api/analytics/speed-agreement')

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function ratioColor(ratio: number | undefined): 'success' | 'warning' | 'danger' {
  if (ratio == null) return 'danger'
  const dev = Math.abs(ratio - 1)
  if (dev <= 0.03) return 'success'
  if (dev <= 0.05) return 'warning'
  return 'danger'
}

// ---------------------------------------------------------------------------
// Chart
// ---------------------------------------------------------------------------

const chartOption = computed(() => {
  const points = data.value?.points ?? []
  const scatterData = points.map((p) => [p.obd_kph, p.gnss_kph])

  return {
    backgroundColor: 'transparent',
    grid: {
      left: 60,
      right: 30,
      top: 30,
      bottom: 50,
    },
    tooltip: {
      trigger: 'item',
      backgroundColor: '#1a1d27',
      borderColor: '#2e3347',
      textStyle: { color: '#e8eaf0', fontSize: 12 },
      formatter: (params: any) => {
        const [obd, gnss] = params.value
        return `OBD: ${obd.toFixed(1)} kph<br/>GNSS: ${gnss.toFixed(1)} kph`
      },
    },
    xAxis: {
      name: 'OBD Speed (kph)',
      nameLocation: 'middle',
      nameGap: 35,
      nameTextStyle: { color: '#8b90a0', fontSize: 12 },
      min: 0,
      max: 250,
      axisLabel: { color: '#8b90a0' },
      axisLine: { lineStyle: { color: '#2e3347' } },
      splitLine: { lineStyle: { color: '#2e3347' } },
    },
    yAxis: {
      name: 'GNSS Speed (kph)',
      nameLocation: 'middle',
      nameGap: 45,
      nameTextStyle: { color: '#8b90a0', fontSize: 12 },
      min: 0,
      max: 250,
      axisLabel: { color: '#8b90a0' },
      axisLine: { lineStyle: { color: '#2e3347' } },
      splitLine: { lineStyle: { color: '#2e3347' } },
    },
    series: [
      {
        type: 'scatter',
        data: scatterData,
        symbolSize: 4,
        itemStyle: {
          color: '#3b82f6',
          opacity: 0.4,
        },
        markLine: {
          silent: true,
          symbol: 'none',
          lineStyle: {
            color: '#22c55e',
            type: 'dashed',
            width: 1.5,
          },
          label: { show: false },
          data: [
            [
              { coord: [0, 0] },
              { coord: [250, 250] },
            ],
          ],
        },
      },
    ],
  }
})
</script>

<template>
  <div>
    <LayoutPageHeader
      title="Speedometer Calibration"
      subtitle="OBD vs GNSS speed agreement"
    />

    <!-- Loading state -->
    <template v-if="pending">
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-4 mb-8">
        <div
          v-for="i in 2"
          :key="i"
          class="rounded-lg p-5 animate-pulse h-28"
          :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }"
        />
      </div>
    </template>

    <!-- Data loaded -->
    <template v-else-if="data">
      <!-- Stat cards -->
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-4 mb-8">
        <DataStatCard
          label="Median Ratio"
          :value="data.median_ratio.toFixed(3)"
          subtitle="OBD / GNSS ratio"
          :color="ratioColor(data.median_ratio)"
        />
        <DataStatCard
          label="Data Points"
          :value="String(data.point_count)"
          subtitle="speed samples"
        />
      </div>

      <!-- Scatter chart -->
      <div
        class="rounded-lg p-6 mb-6"
        :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }"
      >
        <h2 class="text-sm font-semibold mb-4">Speed Agreement -- OBD vs GNSS</h2>

        <template v-if="data.points.length > 0">
          <VChart
            :option="chartOption"
            autoresize
            style="height: 500px; width: 100%"
          />
        </template>
        <DataEmptyState
          v-else
          title="No speed data"
          message="Speed agreement data will appear once trips with both OBD and GNSS readings are recorded."
        />
      </div>

      <!-- Explanation card -->
      <div
        class="rounded-lg p-6"
        :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }"
      >
        <h3
          class="text-xs font-semibold uppercase tracking-wider mb-2"
          style="color: var(--color-text-secondary)"
        >
          How to read this
        </h3>
        <p class="text-sm leading-relaxed" style="color: var(--color-text-secondary)">
          Points along the green diagonal indicate perfect agreement between OBD and GNSS
          speeds. Points above the line mean OBD reads higher than actual speed. A median
          ratio of 1.00 means perfect calibration.
        </p>
      </div>
    </template>

    <!-- Empty / error state -->
    <DataEmptyState
      v-else
      title="No calibration data"
      message="Speed agreement data will appear once trips with both OBD and GNSS readings are recorded."
    />
  </div>
</template>
