<script setup lang="ts">
import 'echarts'
import VChart from 'vue-echarts'

definePageMeta({ layout: 'default' })

interface EconSample {
  speed_kph: number
  maf_cgps: number
  lambda_ratio: number
  rpm: number
  timing_advance_deg: number | null
  load_pct: number | null
  boost_psi: number | null
  ltft_pct: number | null
  boot_id: string
  mono_ms: number
}

interface TripSummary {
  boot_id: string
  first_seen: string
  samples: number
  avg_speed_kph: string
  max_speed_kph: number
  avg_maf: string
  avg_lambda: string
  avg_rpm: string
  avg_timing: string | null
  avg_ltft: string | null
  avg_load: string | null
  duration_s: number
}

interface LoadZone {
  load_zone: string
  avg_timing: string
  avg_lambda: string
  avg_maf: string
  avg_speed: string
  avg_rpm: string
  samples: number
}

const { data } = useFetch<{
  samples: EconSample[]
  perTrip: TripSummary[]
  timingByLoad: LoadZone[]
}>('/api/analytics/fuel-economy')

const samples = computed(() => data.value?.samples ?? [])
const perTrip = computed(() => data.value?.perTrip ?? [])
const timingByLoad = computed(() => data.value?.timingByLoad ?? [])

const BLEND_KEY = 'cairn-ethanol-blend'
const userBlend = ref<number | null>(null)
onMounted(() => {
  const stored = localStorage.getItem(BLEND_KEY)
  if (stored != null) userBlend.value = parseInt(stored, 10)
})

const activeBlend = computed(() => userBlend.value ?? 37)

function stoichForBlend(ethPct: number): number {
  return 14.7 - (ethPct / 100) * 5.7
}

function fuelDensityGPerGal(ethPct: number): number {
  return 2834 + (ethPct / 100) * 154
}

function energyBtuPerGal(ethPct: number): number {
  return 114000 - (ethPct / 100) * 38000
}

function calcMpg(speedKph: number, mafCgps: number, lambda: number, ethPct: number): number {
  const stoich = stoichForBlend(ethPct)
  const fuelGs = (mafCgps / 100) / (stoich * lambda)
  const fuelGalPerS = fuelGs / fuelDensityGPerGal(ethPct)
  const fuelGalPerHr = fuelGalPerS * 3600
  const speedMph = speedKph / 1.60934
  if (fuelGalPerHr <= 0) return 0
  return speedMph / fuelGalPerHr
}

const mpgData = computed(() =>
  samples.value
    .map(s => ({
      ...s,
      speed_mph: s.speed_kph / 1.60934,
      mpg: calcMpg(s.speed_kph, s.maf_cgps, s.lambda_ratio, activeBlend.value),
    }))
    .filter(s => s.mpg > 0 && s.mpg < 80)
)

const avgMpg = computed(() => {
  if (!mpgData.value.length) return null
  return mpgData.value.reduce((s, p) => s + p.mpg, 0) / mpgData.value.length
})

const bestMpg = computed(() => {
  if (!mpgData.value.length) return null
  return mpgData.value.reduce((m, p) => Math.max(m, p.mpg), 0)
})

const avgTiming = computed(() => {
  const pts = mpgData.value.filter(p => p.timing_advance_deg != null)
  if (!pts.length) return null
  return pts.reduce((s, p) => s + (p.timing_advance_deg ?? 0), 0) / pts.length
})

const ethPenalty = computed(() => {
  const e0Energy = energyBtuPerGal(0)
  const blendEnergy = energyBtuPerGal(activeBlend.value)
  return Math.round((1 - blendEnergy / e0Energy) * 100)
})

const perTripMpg = computed(() =>
  perTrip.value.map(t => {
    const tripSamples = mpgData.value.filter(s => s.boot_id === t.boot_id)
    const avgTripMpg = tripSamples.length
      ? tripSamples.reduce((s, p) => s + p.mpg, 0) / tripSamples.length
      : null
    return { ...t, avgMpg: avgTripMpg }
  })
)

const mpgSpeedChartOption = computed(() => {
  const pts = mpgData.value
  if (!pts.length) return {}
  return {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'item',
      backgroundColor: '#1a1d27', borderColor: '#2e3347',
      textStyle: { color: '#e8eaf0', fontSize: 12 },
      formatter: (p: any) => {
        const d = p.data
        return `Speed: <b>${d[0].toFixed(0)} mph</b><br>MPG: <b>${d[1].toFixed(1)}</b><br>RPM: <b>${d[2]}</b>`
      },
    },
    grid: { top: 24, right: 24, bottom: 48, left: 56 },
    xAxis: {
      name: 'Speed (mph)', nameLocation: 'center', nameGap: 32,
      nameTextStyle: { color: '#8b90a0', fontSize: 12 },
      type: 'value',
      axisLabel: { color: '#8b90a0', fontSize: 11 },
      axisLine: { lineStyle: { color: '#2e3347' } },
      splitLine: { lineStyle: { color: '#2e3347', opacity: 0.3 } },
    },
    yAxis: {
      name: 'MPG', nameTextStyle: { color: '#8b90a0', fontSize: 12 },
      type: 'value', min: 0,
      axisLabel: { color: '#8b90a0', fontSize: 11 },
      axisLine: { lineStyle: { color: '#2e3347' } },
      splitLine: { lineStyle: { color: '#2e3347', opacity: 0.3 } },
    },
    series: [{
      type: 'scatter', symbolSize: 8,
      data: pts.map(p => [p.speed_mph, p.mpg, p.rpm]),
      itemStyle: { color: '#22c55e', opacity: 0.8 },
    }],
  }
})

const mpgRpmChartOption = computed(() => {
  const pts = mpgData.value
  if (!pts.length) return {}
  return {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'item',
      backgroundColor: '#1a1d27', borderColor: '#2e3347',
      textStyle: { color: '#e8eaf0', fontSize: 12 },
      formatter: (p: any) => {
        const d = p.data
        return `RPM: <b>${d[0]}</b><br>MPG: <b>${d[1].toFixed(1)}</b><br>Load: <b>${d[2] != null ? d[2] + '%' : '--'}</b>`
      },
    },
    grid: { top: 24, right: 24, bottom: 48, left: 56 },
    xAxis: {
      name: 'RPM', nameLocation: 'center', nameGap: 32,
      nameTextStyle: { color: '#8b90a0', fontSize: 12 },
      type: 'value', min: 500,
      axisLabel: { color: '#8b90a0', fontSize: 11 },
      axisLine: { lineStyle: { color: '#2e3347' } },
      splitLine: { lineStyle: { color: '#2e3347', opacity: 0.3 } },
    },
    yAxis: {
      name: 'MPG', nameTextStyle: { color: '#8b90a0', fontSize: 12 },
      type: 'value', min: 0,
      axisLabel: { color: '#8b90a0', fontSize: 11 },
      axisLine: { lineStyle: { color: '#2e3347' } },
      splitLine: { lineStyle: { color: '#2e3347', opacity: 0.3 } },
    },
    visualMap: {
      show: true, orient: 'vertical', right: 0, top: 'center',
      min: 0, max: 80, text: ['Heavy', 'Light'],
      inRange: { color: ['#22c55e', '#eab308', '#ef4444'] },
      textStyle: { color: '#8b90a0', fontSize: 10 },
      dimension: 2,
    },
    series: [{
      type: 'scatter', symbolSize: 8,
      data: pts.map(p => [p.rpm, p.mpg, p.load_pct ?? 30]),
      itemStyle: { opacity: 0.8 },
    }],
  }
})

const loadZoneOrder = ['light', 'medium', 'heavy']
const loadZoneLabel: Record<string, string> = { light: '< 20%', medium: '20–50%', heavy: '> 50%' }
</script>

<template>
  <div>
    <LayoutPageHeader title="Fuel Economy" subtitle="Estimated MPG, efficiency by speed and RPM, tuning context" />

    <!-- Stat cards -->
    <div class="grid grid-cols-2 lg:grid-cols-4 gap-3 md:gap-4 mb-4 sticky top-0 z-10 py-3 -mt-3" style="background: var(--color-background)">
      <DataStatCard label="Avg MPG" :value="avgMpg != null ? avgMpg.toFixed(1) : '--'" :subtitle="`est. at E${activeBlend}`" />
      <DataStatCard label="Best MPG" :value="bestMpg != null ? bestMpg.toFixed(1) : '--'" subtitle="peak instantaneous" />
      <DataStatCard label="Avg Timing" :value="avgTiming != null ? `${avgTiming.toFixed(1)}°` : '--'" subtitle="advance under load" />
      <DataStatCard label="E-Penalty" :value="`-${ethPenalty}%`" :subtitle="`vs E0 at E${activeBlend}`"
        :color="ethPenalty > 15 ? 'warning' : undefined" />
    </div>

    <!-- MPG vs Speed -->
    <div class="rounded-xl p-6 mb-4" :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }">
      <div class="flex items-center justify-between mb-4">
        <h2 class="text-base font-semibold">MPG vs Speed</h2>
        <span class="text-[11px]" style="color: var(--color-text-secondary)">Estimated from MAF at E{{ activeBlend }}</span>
      </div>
      <DataEmptyState v-if="!mpgData.length" title="No economy data" message="Need MAF + speed + lambda samples while driving." />
      <VChart v-else :option="mpgSpeedChartOption" autoresize style="height: 340px; width: 100%" />
    </div>

    <!-- MPG vs RPM + Tuning by Load side by side -->
    <div class="grid grid-cols-1 lg:grid-cols-2 gap-4 mb-4">
      <div class="rounded-xl p-6" :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }">
        <div class="flex items-center justify-between mb-4">
          <h2 class="text-base font-semibold">MPG vs RPM</h2>
          <span class="text-[11px]" style="color: var(--color-text-secondary)">Colored by engine load</span>
        </div>
        <DataEmptyState v-if="!mpgData.length" title="No data" message="Need MAF + RPM samples." />
        <VChart v-else :option="mpgRpmChartOption" autoresize style="height: 280px; width: 100%" />
      </div>

      <!-- Tuning efficiency by load zone -->
      <div class="rounded-xl p-6" :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }">
        <h2 class="text-base font-semibold mb-4">Tuning Efficiency by Load</h2>
        <DataEmptyState v-if="!timingByLoad.length" title="No data" message="Need timing + load data." />
        <div v-else class="space-y-3">
          <div v-for="zone in loadZoneOrder" :key="zone">
            <template v-for="z in timingByLoad.filter(t => t.load_zone === zone)" :key="z.load_zone">
              <div class="rounded-lg px-4 py-3" :style="{ backgroundColor: 'var(--color-surface-elevated)' }">
                <div class="flex items-center justify-between mb-2">
                  <div class="flex items-center gap-2">
                    <div class="w-2 h-2 rounded-full" :style="{ background: zone === 'light' ? '#22c55e' : zone === 'medium' ? '#f59e0b' : '#ef4444' }" />
                    <span class="text-[12px] font-semibold capitalize">{{ zone }} load</span>
                    <span class="text-[10px] font-mono" style="color: var(--color-text-secondary)">{{ loadZoneLabel[zone] }}</span>
                  </div>
                  <span class="text-[10px] font-mono" style="color: var(--color-text-secondary)">{{ z.samples }} samples</span>
                </div>
                <div class="grid grid-cols-4 gap-3 text-[11px] font-mono">
                  <div>
                    <div style="color: var(--color-text-secondary)">Timing</div>
                    <div class="font-semibold">{{ z.avg_timing }}°</div>
                  </div>
                  <div>
                    <div style="color: var(--color-text-secondary)">Lambda</div>
                    <div class="font-semibold">{{ z.avg_lambda }}</div>
                  </div>
                  <div>
                    <div style="color: var(--color-text-secondary)">Avg RPM</div>
                    <div class="font-semibold">{{ z.avg_rpm }}</div>
                  </div>
                  <div>
                    <div style="color: var(--color-text-secondary)">Avg Speed</div>
                    <div class="font-semibold">{{ (parseFloat(z.avg_speed) / 1.60934).toFixed(0) }} mph</div>
                  </div>
                </div>
              </div>
            </template>
          </div>
        </div>
      </div>
    </div>

    <!-- Per-trip comparison -->
    <div class="rounded-xl p-6 mb-4" :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }">
      <h2 class="text-base font-semibold mb-4">Economy by Trip</h2>
      <DataEmptyState v-if="!perTripMpg.length" title="No trips" message="Need multiple trips with MAF data." />
      <div v-else class="space-y-2">
        <div v-for="t in perTripMpg" :key="t.boot_id"
          class="flex items-center justify-between py-3 px-4 rounded-lg"
          :style="{ backgroundColor: 'var(--color-surface-elevated)' }"
        >
          <div class="flex items-center gap-3">
            <div class="w-2 h-2 rounded-full" :style="{ background: (t.avgMpg ?? 0) > 20 ? '#22c55e' : (t.avgMpg ?? 0) > 10 ? '#f59e0b' : '#ef4444' }" />
            <span class="text-[11px] font-mono" style="color: var(--color-text-secondary)">{{ t.boot_id.slice(0, 8) }}</span>
          </div>
          <div class="flex items-center gap-5 text-[12px] font-mono">
            <span>
              <b :style="{ color: (t.avgMpg ?? 0) > 20 ? '#22c55e' : 'inherit' }">
                {{ t.avgMpg != null ? t.avgMpg.toFixed(1) : '--' }} MPG
              </b>
            </span>
            <span style="color: var(--color-text-secondary)">{{ (parseFloat(t.avg_speed_kph) / 1.60934).toFixed(0) }} mph avg</span>
            <span style="color: var(--color-text-secondary)">{{ t.avg_rpm }} rpm</span>
            <span v-if="t.avg_timing" style="color: var(--color-text-secondary)">{{ t.avg_timing }}° timing</span>
            <span style="color: var(--color-text-secondary)">{{ t.samples }} pts</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Ethanol economy context -->
    <div class="rounded-xl px-5 py-4" :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }">
      <p class="text-[12px] leading-relaxed" style="color: var(--color-text-secondary)">
        <b>Ethanol and fuel economy:</b> E{{ activeBlend }} has ~{{ ethPenalty }}% less energy per gallon than pure gasoline
        ({{ energyBtuPerGal(activeBlend).toLocaleString() }} vs {{ energyBtuPerGal(0).toLocaleString() }} BTU/gal).
        This means you'll use roughly {{ ethPenalty }}% more fuel by volume for the same work — but ethanol's higher octane
        allows more aggressive timing advance, which can partially offset the penalty under load.
        MPG estimates here are calculated from MAF airflow, lambda, and the stoichiometric AFR for your blend ({{ stoichForBlend(activeBlend).toFixed(1) }}:1).
      </p>
    </div>
  </div>
</template>
