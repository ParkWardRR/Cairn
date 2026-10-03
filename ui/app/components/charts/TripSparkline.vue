<script setup lang="ts">
import 'echarts'
import VChart from 'vue-echarts'

const props = defineProps<{
  coordinates: [number, number, number][]
  speeds: number[]
}>()

const chartOption = computed(() => {
  const validIdxs: number[] = []
  props.coordinates.forEach((c, i) => {
    if (c[0] !== 0 && c[1] !== 0) validIdxs.push(i)
  })

  if (validIdxs.length < 2) return null

  const speedKph = validIdxs.map(i => Math.round((props.speeds[i] ?? 0) * 3.6))
  const altitudes = validIdxs.map(i => props.coordinates[i][2] ?? 0)
  const labels = validIdxs.map((_, i) => String(i))

  return {
    backgroundColor: 'transparent',
    animation: false,
    grid: { top: 8, right: 8, bottom: 20, left: 40 },
    tooltip: {
      trigger: 'axis' as const,
      backgroundColor: '#1a1d27',
      borderColor: '#262b3d',
      textStyle: { color: '#e8eaf0', fontSize: 11 },
      formatter: (params: any) => {
        const s = params[0]?.value ?? 0
        const a = params[1]?.value ?? 0
        return `Speed: <b>${s} kph</b><br>Altitude: <b>${a.toFixed(0)} m</b>`
      },
    },
    xAxis: {
      type: 'category' as const,
      data: labels,
      show: false,
    },
    yAxis: [
      {
        type: 'value' as const,
        name: 'kph',
        nameTextStyle: { color: '#7c8298', fontSize: 10 },
        axisLabel: { color: '#7c8298', fontSize: 10 },
        splitLine: { lineStyle: { color: '#262b3d' } },
      },
      {
        type: 'value' as const,
        name: 'm',
        nameTextStyle: { color: '#7c8298', fontSize: 10 },
        axisLabel: { color: '#7c8298', fontSize: 10 },
        splitLine: { show: false },
      },
    ],
    series: [
      {
        name: 'Speed',
        type: 'line',
        data: speedKph,
        yAxisIndex: 0,
        showSymbol: false,
        lineStyle: { color: '#3b82f6', width: 1.5 },
        areaStyle: { color: 'rgba(59,130,246,0.08)' },
        itemStyle: { color: '#3b82f6' },
      },
      {
        name: 'Altitude',
        type: 'line',
        data: altitudes,
        yAxisIndex: 1,
        showSymbol: false,
        lineStyle: { color: '#8b5cf6', width: 1 },
        areaStyle: { color: 'rgba(139,92,246,0.06)' },
        itemStyle: { color: '#8b5cf6' },
      },
    ],
  }
})
</script>

<template>
  <div class="w-full">
    <VChart v-if="chartOption" :option="chartOption" autoresize style="height: 140px; width: 100%" />
    <div v-else class="flex items-center justify-center" style="height: 140px; color: var(--color-text-secondary)">
      <p class="text-[13px]">No GPS data for speed/elevation chart</p>
    </div>
  </div>
</template>
