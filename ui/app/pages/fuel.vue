<script setup lang="ts">
import 'echarts'
import VChart from 'vue-echarts'

definePageMeta({ layout: 'default' })

interface TrimRow {
  rpm_bin: number
  load_bin: number
  avg_stft: number
  avg_ltft: number
  samples: number | string
}

const { data: trimData, status } = useFetch<{ trimMap: TrimRow[] }>('/api/analytics/trim-map')

const rows = computed(() => trimData.value?.trimMap ?? [])

const rpmBins = computed(() => [...new Set(rows.value.map(r => r.rpm_bin))].sort((a, b) => a - b))
const loadBins = computed(() => [...new Set(rows.value.map(r => r.load_bin))].sort((a, b) => a - b))

const ltftAvg = computed(() => {
  if (!rows.value.length) return null
  const sum = rows.value.reduce((acc, b) => acc + b.avg_ltft, 0)
  return sum / rows.value.length
})

const stftAvg = computed(() => {
  if (!rows.value.length) return null
  const sum = rows.value.reduce((acc, b) => acc + b.avg_stft, 0)
  return sum / rows.value.length
})

const ltftColor = computed<'success' | 'warning' | 'danger'>(() => {
  if (ltftAvg.value == null) return 'success'
  const abs = Math.abs(ltftAvg.value)
  if (abs < 5) return 'success'
  if (abs < 10) return 'warning'
  return 'danger'
})

const chartOption = computed(() => {
  if (!rows.value.length) return {}

  const heatData: [number, number, number][] = rows.value.map(b => {
    const xi = rpmBins.value.indexOf(b.rpm_bin)
    const yi = loadBins.value.indexOf(b.load_bin)
    return [xi, yi, b.avg_ltft]
  })

  return {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'item',
      backgroundColor: '#1a1d27',
      borderColor: '#2e3347',
      textStyle: { color: '#e8eaf0', fontSize: 13 },
      formatter: (params: any) => {
        const d = params.data
        const rpm = rpmBins.value[d[0]] ?? d[0]
        const load = loadBins.value[d[1]] ?? d[1]
        return `RPM: <b>${rpm}</b><br/>Load: <b>${load}%</b><br/>LTFT: <b>${d[2].toFixed(1)}%</b>`
      },
    },
    grid: { top: 24, right: 100, bottom: 48, left: 64 },
    xAxis: {
      type: 'category',
      data: rpmBins.value.map(String),
      name: 'RPM', nameLocation: 'center', nameGap: 32,
      nameTextStyle: { color: '#8b90a0', fontSize: 12 },
      axisLabel: { color: '#8b90a0', fontSize: 11 },
      axisLine: { lineStyle: { color: '#2e3347' } },
      splitArea: { show: false },
    },
    yAxis: {
      type: 'category',
      data: loadBins.value.map(String),
      name: 'Load %',
      nameTextStyle: { color: '#8b90a0', fontSize: 12 },
      axisLabel: { color: '#8b90a0', fontSize: 11 },
      axisLine: { lineStyle: { color: '#2e3347' } },
      splitArea: { show: false },
    },
    visualMap: {
      min: -15, max: 15, show: true, orient: 'vertical', right: 10, top: 'center',
      inRange: { color: ['#22c55e', '#4ade80', '#bbf7d0', '#e8eaf0', '#fecaca', '#f87171', '#ef4444'] },
      textStyle: { color: '#8b90a0', fontSize: 11 },
    },
    series: [{
      type: 'heatmap',
      data: heatData,
      label: {
        show: true, color: '#e8eaf0', fontSize: 10,
        formatter: (params: any) => params.data[2].toFixed(1),
      },
      emphasis: { itemStyle: { borderColor: '#e8eaf0', borderWidth: 1 } },
    }],
  }
})

const loading = computed(() => status.value === 'pending')
</script>

<template>
  <div>
    <LayoutPageHeader title="Fuel & Tune Health" subtitle="Fuel trim and lambda analysis" />

    <div class="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-8">
      <template v-if="loading">
        <div v-for="i in 3" :key="i" class="rounded-lg p-5 animate-pulse h-28"
          :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }" />
      </template>
      <template v-else>
        <DataStatCard label="STFT Average" :value="stftAvg != null ? `${stftAvg.toFixed(1)}%` : '--'" subtitle="Short-term fuel trim" />
        <DataStatCard label="LTFT Average" :value="ltftAvg != null ? `${ltftAvg.toFixed(1)}%` : '--'" subtitle="Long-term fuel trim" :color="ltftColor" />
        <DataStatCard label="Samples" :value="String(rows.length)" subtitle="RPM/load bins with data" />
      </template>
    </div>

    <div class="rounded-lg p-6 mb-6"
      :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }">
      <h2 class="text-lg font-semibold mb-4">Fuel Trim Map — RPM vs Load</h2>

      <template v-if="loading">
        <div class="flex items-center justify-center" style="height: 450px">
          <div class="w-6 h-6 rounded-full border-2 animate-spin"
            :style="{ borderColor: 'var(--color-border)', borderTopColor: 'var(--color-accent)' }" />
        </div>
      </template>

      <DataEmptyState v-else-if="!rows.length" title="No fuel trim data"
        message="Fuel trim map data will appear here once enough OBD samples are collected." />

      <VChart v-else :option="chartOption" autoresize style="height: 450px; width: 100%" />
    </div>

    <div class="rounded-lg px-5 py-4"
      :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }">
      <p class="text-sm leading-relaxed" style="color: var(--color-text-secondary)">
        Green = rich (negative LTFT), Red = lean (positive LTFT). On this car's ~E41 ethanol blend, expect positive LTFT — the ECU adds fuel beyond the base map. Values over ±15% may indicate a tune issue.
      </p>
    </div>
  </div>
</template>
