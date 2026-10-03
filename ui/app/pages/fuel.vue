<script setup lang="ts">
import 'echarts'
import VChart from 'vue-echarts'

definePageMeta({ layout: 'default' })

interface TrimRow { rpm_bin: number; load_bin: number; avg_stft: number; avg_ltft: number; samples: number | string }
interface TripTrim { boot_id: string; first_seen: string; avg_ltft: string; avg_stft: string; avg_lambda: string; peak_maf_cgps: number | null; samples: number }
interface FlowSample { mono_ms: number; rpm: number; maf_cgps: number; lambda_ratio: number; boost_psi: number | null; intake_c: number | null; load_pct: number | null; ltft_pct: number | null; stft_pct: number | null }
interface LambdaLoadPoint { load_pct: number; lambda_ratio: number; rpm: number; boost_psi: number | null }

const { data: trimData, status: trimStatus } = useFetch<{ trimMap: TrimRow[] }>('/api/analytics/trim-map')
const { data: fuelData, status: fuelStatus } = useFetch<{
  perTrip: TripTrim[]
  flowSamples: FlowSample[]
  lambdaLoad: LambdaLoadPoint[]
}>('/api/analytics/fuel-health')

const rows = computed(() => trimData.value?.trimMap ?? [])
const perTrip = computed(() => fuelData.value?.perTrip ?? [])
const flowSamples = computed(() => fuelData.value?.flowSamples ?? [])
const lambdaLoad = computed(() => fuelData.value?.lambdaLoad ?? [])

const rpmBins = computed(() => [...new Set(rows.value.map(r => r.rpm_bin))].sort((a, b) => a - b))
const loadBins = computed(() => [...new Set(rows.value.map(r => r.load_bin))].sort((a, b) => a - b))

const overallLtft = computed(() => {
  if (!perTrip.value.length) return null
  const total = perTrip.value.reduce((s, t) => s + parseFloat(t.avg_ltft) * t.samples, 0)
  const count = perTrip.value.reduce((s, t) => s + t.samples, 0)
  return count ? total / count : null
})

const overallStft = computed(() => {
  const valid = perTrip.value.filter(t => t.avg_stft != null)
  if (!valid.length) return null
  const total = valid.reduce((s, t) => s + parseFloat(t.avg_stft) * t.samples, 0)
  const count = valid.reduce((s, t) => s + t.samples, 0)
  return count ? total / count : null
})

// Ethanol estimation: stoich_gas=14.7, stoich_eth=9.0
// LTFT ≈ (14.7 / blend_stoich - 1) * 100
// Solving: ethanol% = (14.7 * LTFT / (100 + LTFT)) * 100 / 5.7
function estimateEthanolFromLtft(ltft: number): number {
  if (ltft <= 0) return 0
  return Math.round((14.7 * ltft / (100 + ltft)) * 100 / 5.7)
}

const estimatedBlend = computed(() => {
  if (overallLtft.value == null) return null
  return estimateEthanolFromLtft(overallLtft.value)
})

const BLEND_KEY = 'cairn-ethanol-blend'
const userBlend = ref<number | null>(null)

onMounted(() => {
  const stored = localStorage.getItem(BLEND_KEY)
  if (stored != null) userBlend.value = parseInt(stored, 10)
})

function setBlend(val: number) {
  userBlend.value = val
  localStorage.setItem(BLEND_KEY, String(val))
}

function clearBlend() {
  userBlend.value = null
  localStorage.removeItem(BLEND_KEY)
}

const blendInput = ref('')
function applyBlendInput() {
  const v = parseInt(blendInput.value, 10)
  if (!isNaN(v) && v >= 0 && v <= 85) {
    setBlend(v)
    blendInput.value = ''
  }
}

const activeBlend = computed(() => userBlend.value ?? estimatedBlend.value ?? 0)

function stoichForBlend(ethPct: number): number {
  return 14.7 - (ethPct / 100) * 5.7
}

// Fuel flow: MAF(g/s) / (stoich * lambda) = fuel flow g/s
// Convert to lb/hr: * 7.9366
const fuelFlowData = computed(() => {
  const stoich = stoichForBlend(activeBlend.value)
  return flowSamples.value.map(s => {
    const mafGs = s.maf_cgps / 100
    const fuelGs = mafGs / (stoich * s.lambda_ratio)
    return {
      rpm: s.rpm,
      fuel_lbhr: fuelGs * 7.9366,
      fuel_gs: fuelGs,
      maf_gs: mafGs,
      boost_psi: s.boost_psi,
      intake_c: s.intake_c,
      lambda: s.lambda_ratio,
    }
  })
})

const peakFlow = computed(() => {
  if (!fuelFlowData.value.length) return null
  return fuelFlowData.value.reduce((m, p) => Math.max(m, p.fuel_lbhr), 0)
})

// Charts
const trimHeatmapOption = computed(() => {
  if (!rows.value.length) return {}
  const heatData: [number, number, number][] = rows.value.map(b => {
    const xi = rpmBins.value.indexOf(b.rpm_bin)
    const yi = loadBins.value.indexOf(b.load_bin)
    return [xi, yi, b.avg_ltft]
  })
  return {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'item', backgroundColor: '#1a1d27', borderColor: '#2e3347',
      textStyle: { color: '#e8eaf0', fontSize: 12 },
      formatter: (p: any) => {
        const d = p.data
        return `RPM: <b>${rpmBins.value[d[0]]}</b><br>Load: <b>${loadBins.value[d[1]]}%</b><br>LTFT: <b>${d[2].toFixed(1)}%</b>`
      },
    },
    grid: { top: 24, right: 100, bottom: 48, left: 64 },
    xAxis: {
      type: 'category', data: rpmBins.value.map(String),
      name: 'RPM', nameLocation: 'center', nameGap: 32,
      nameTextStyle: { color: '#8b90a0', fontSize: 12 },
      axisLabel: { color: '#8b90a0', fontSize: 11 },
      axisLine: { lineStyle: { color: '#2e3347' } },
    },
    yAxis: {
      type: 'category', data: loadBins.value.map(String),
      name: 'Load %', nameTextStyle: { color: '#8b90a0', fontSize: 12 },
      axisLabel: { color: '#8b90a0', fontSize: 11 },
      axisLine: { lineStyle: { color: '#2e3347' } },
    },
    visualMap: {
      min: -15, max: 25, show: true, orient: 'vertical', right: 10, top: 'center',
      inRange: { color: ['#22c55e', '#4ade80', '#bbf7d0', '#e8eaf0', '#fecaca', '#f87171', '#ef4444'] },
      textStyle: { color: '#8b90a0', fontSize: 11 },
    },
    series: [{ type: 'heatmap', data: heatData, label: { show: true, color: '#e8eaf0', fontSize: 10, formatter: (p: any) => p.data[2].toFixed(1) }, emphasis: { itemStyle: { borderColor: '#e8eaf0', borderWidth: 1 } } }],
  }
})

const fuelFlowChartOption = computed(() => {
  const pts = fuelFlowData.value
  if (!pts.length) return {}
  return {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'item', backgroundColor: '#1a1d27', borderColor: '#2e3347',
      textStyle: { color: '#e8eaf0', fontSize: 12 },
      formatter: (p: any) => {
        const d = p.data
        return `RPM: <b>${d[0]}</b><br>Fuel: <b>${d[1].toFixed(1)} lb/hr</b><br>MAF: <b>${d[2].toFixed(1)} g/s</b>`
      },
    },
    grid: { top: 24, right: 24, bottom: 48, left: 64 },
    xAxis: {
      name: 'RPM', nameLocation: 'center', nameGap: 32,
      nameTextStyle: { color: '#8b90a0', fontSize: 12 },
      type: 'value', min: 500, max: 7000,
      axisLabel: { color: '#8b90a0', fontSize: 11 },
      axisLine: { lineStyle: { color: '#2e3347' } },
      splitLine: { lineStyle: { color: '#2e3347', opacity: 0.3 } },
    },
    yAxis: {
      name: 'lb/hr', nameTextStyle: { color: '#8b90a0', fontSize: 12 },
      type: 'value',
      axisLabel: { color: '#8b90a0', fontSize: 11 },
      axisLine: { lineStyle: { color: '#2e3347' } },
      splitLine: { lineStyle: { color: '#2e3347', opacity: 0.3 } },
    },
    series: [{
      type: 'scatter', symbolSize: 6,
      data: pts.map(p => [p.rpm, p.fuel_lbhr, p.maf_gs]),
      itemStyle: { color: '#f59e0b', opacity: 0.7 },
    }],
  }
})

const lambdaLoadChartOption = computed(() => {
  const pts = lambdaLoad.value
  if (!pts.length) return {}
  return {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'item', backgroundColor: '#1a1d27', borderColor: '#2e3347',
      textStyle: { color: '#e8eaf0', fontSize: 12 },
      formatter: (p: any) => {
        const d = p.data
        let html = `Load: <b>${d[0]}%</b><br>Lambda: <b>${d[1].toFixed(3)}</b>`
        if (d[2]) html += `<br>Boost: <b>${d[2].toFixed(1)} psi</b>`
        return html
      },
    },
    grid: { top: 24, right: 24, bottom: 48, left: 56 },
    xAxis: {
      name: 'Load %', nameLocation: 'center', nameGap: 32,
      nameTextStyle: { color: '#8b90a0', fontSize: 12 },
      type: 'value',
      axisLabel: { color: '#8b90a0', fontSize: 11 },
      axisLine: { lineStyle: { color: '#2e3347' } },
      splitLine: { lineStyle: { color: '#2e3347', opacity: 0.3 } },
    },
    yAxis: {
      name: 'Lambda', nameTextStyle: { color: '#8b90a0', fontSize: 12 },
      type: 'value', min: 0.7, max: 1.15,
      axisLabel: { color: '#8b90a0', fontSize: 11 },
      axisLine: { lineStyle: { color: '#2e3347' } },
      splitLine: { lineStyle: { color: '#2e3347', opacity: 0.3 } },
    },
    visualMap: {
      show: false,
      min: 0.8, max: 1.05, dimension: 1,
      inRange: { color: ['#22c55e', '#84cc16', '#e8eaf0', '#fecaca', '#ef4444'] },
    },
    series: [{
      type: 'scatter', symbolSize: 6,
      data: pts.map(p => [p.load_pct, p.lambda_ratio, p.boost_psi]),
      itemStyle: { opacity: 0.7 },
      markLine: {
        data: [
          { yAxis: 1.0, label: { show: true, formatter: 'Stoich', color: '#8b90a0', fontSize: 10 }, lineStyle: { color: '#8b90a0', type: 'dashed', opacity: 0.4 } },
          { yAxis: 0.82, label: { show: true, formatter: 'WOT target', color: '#22c55e', fontSize: 10 }, lineStyle: { color: '#22c55e', type: 'dashed', opacity: 0.4 } },
        ],
      },
    }],
  }
})

const loading = computed(() => trimStatus.value === 'pending' || fuelStatus.value === 'pending')
</script>

<template>
  <div>
    <LayoutPageHeader title="Fuel & Tune" subtitle="Ethanol blend estimation, fuel flow, and trim health" />

    <!-- Blend Estimator -->
    <div class="rounded-xl p-5 mb-4" :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }">
      <div class="flex flex-col sm:flex-row sm:items-center gap-4">
        <div class="flex-1">
          <div class="flex items-center gap-2 mb-2">
            <h2 class="text-base font-semibold">Ethanol Blend</h2>
            <span v-if="userBlend != null" class="text-[10px] font-semibold px-2 py-0.5 rounded-full" style="background: var(--color-accent-soft); color: var(--color-accent)">User set</span>
            <span v-else class="text-[10px] font-semibold px-2 py-0.5 rounded-full" style="background: rgba(245, 158, 11, 0.15); color: #f59e0b">Estimated from LTFT</span>
          </div>
          <p class="text-[12px] leading-relaxed" style="color: var(--color-text-secondary)">
            <template v-if="estimatedBlend != null">
              LTFT of {{ overallLtft?.toFixed(1) }}% suggests ~E{{ estimatedBlend }} blend.
              {{ userBlend != null ? `Using your override: E${userBlend}.` : 'Enter your actual blend below for accurate fuel flow.' }}
            </template>
            <template v-else>Not enough trim data to estimate blend.</template>
          </p>
        </div>

        <div class="flex items-center gap-2">
          <div class="font-mono text-3xl font-bold" :style="{ color: activeBlend > 50 ? '#22c55e' : activeBlend > 20 ? '#f59e0b' : 'var(--color-text-primary)' }">
            E{{ activeBlend }}
          </div>
        </div>
      </div>

      <div class="flex items-center gap-2 mt-3 pt-3" :style="{ borderTop: '1px solid var(--color-border)' }">
        <div class="flex items-center gap-1">
          <button v-for="preset in [0, 10, 30, 41, 50, 60, 85]" :key="preset"
            class="px-2.5 py-1 rounded-md text-[11px] font-semibold transition-colors"
            :style="{
              background: activeBlend === preset ? 'var(--color-accent)' : 'var(--color-surface-elevated)',
              color: activeBlend === preset ? '#fff' : 'var(--color-text-secondary)',
            }"
            @click="setBlend(preset)"
          >E{{ preset }}</button>
        </div>
        <div class="flex items-center gap-1 ml-2">
          <input
            v-model="blendInput"
            type="text"
            inputmode="numeric"
            placeholder="E?"
            class="w-12 px-2 py-1 rounded-md text-[11px] font-mono text-center"
            :style="{ background: 'var(--color-surface-elevated)', color: 'var(--color-text-primary)', border: '1px solid var(--color-border)' }"
            @keydown.enter="applyBlendInput"
          >
          <button
            class="px-2 py-1 rounded-md text-[11px] font-semibold"
            :style="{ background: 'var(--color-surface-elevated)', color: 'var(--color-text-secondary)' }"
            @click="applyBlendInput"
          >Set</button>
        </div>
        <button v-if="userBlend != null"
          class="ml-auto text-[11px] font-medium px-2 py-1 rounded-md transition-colors hover:bg-[var(--color-surface-elevated)]"
          style="color: var(--color-text-secondary)"
          @click="clearBlend"
        >Reset to estimate</button>
      </div>
    </div>

    <!-- Key stats -->
    <div class="grid grid-cols-2 lg:grid-cols-4 gap-3 md:gap-4 mb-4">
      <DataStatCard label="LTFT" :value="overallLtft != null ? `${overallLtft > 0 ? '+' : ''}${overallLtft.toFixed(1)}%` : '--'" subtitle="long-term fuel trim"
        :color="(overallLtft ?? 0) > 20 ? 'warning' : undefined" />
      <DataStatCard label="STFT" :value="overallStft != null ? `${overallStft > 0 ? '+' : ''}${overallStft.toFixed(1)}%` : '--'" subtitle="short-term fuel trim" />
      <DataStatCard label="Peak Flow" :value="peakFlow != null ? `${peakFlow.toFixed(1)}` : '--'" subtitle="lb/hr estimated" />
      <DataStatCard label="Stoich AFR" :value="stoichForBlend(activeBlend).toFixed(1)" :subtitle="`at E${activeBlend}`" />
    </div>

    <!-- Fuel Flow Chart -->
    <div class="rounded-xl p-6 mb-4" :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }">
      <div class="flex items-center justify-between mb-4">
        <h2 class="text-base font-semibold">Fuel Flow vs RPM</h2>
        <span class="text-[11px]" style="color: var(--color-text-secondary)">Estimated from MAF at E{{ activeBlend }} (stoich {{ stoichForBlend(activeBlend).toFixed(1) }})</span>
      </div>
      <DataEmptyState v-if="!flowSamples.length" title="No MAF data" message="Need MAF + lambda samples for fuel flow estimation." />
      <VChart v-else :option="fuelFlowChartOption" autoresize style="height: 320px; width: 100%" />
    </div>

    <!-- Lambda under Load + Trim Map side by side on large screens -->
    <div class="grid grid-cols-1 lg:grid-cols-2 gap-4 mb-4">
      <div class="rounded-xl p-6" :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }">
        <div class="flex items-center justify-between mb-4">
          <h2 class="text-base font-semibold">Lambda vs Load</h2>
          <span class="text-[11px]" style="color: var(--color-text-secondary)">Rich under load = safe</span>
        </div>
        <DataEmptyState v-if="!lambdaLoad.length" title="No lambda data" message="Need lambda + load samples." />
        <VChart v-else :option="lambdaLoadChartOption" autoresize style="height: 320px; width: 100%" />
      </div>

      <!-- Per-trip LTFT trend -->
      <div class="rounded-xl p-6" :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }">
        <h2 class="text-base font-semibold mb-4">LTFT by Trip</h2>
        <DataEmptyState v-if="!perTrip.length" title="No trim data" message="Need multiple trips with fuel trim data." />
        <div v-else class="space-y-2">
          <div v-for="t in perTrip" :key="t.boot_id"
            class="flex items-center justify-between py-2 px-3 rounded-lg"
            :style="{ backgroundColor: 'var(--color-surface-elevated)' }"
          >
            <div class="flex items-center gap-3">
              <div class="w-2 h-2 rounded-full" :style="{ background: parseFloat(t.avg_ltft) > 20 ? '#ef4444' : parseFloat(t.avg_ltft) > 15 ? '#f59e0b' : '#22c55e' }" />
              <span class="text-[11px] font-mono" style="color: var(--color-text-secondary)">{{ t.boot_id.slice(0, 8) }}</span>
            </div>
            <div class="flex items-center gap-4 text-[12px] font-mono">
              <span>LTFT <b :style="{ color: parseFloat(t.avg_ltft) > 20 ? '#ef4444' : 'inherit' }">{{ parseFloat(t.avg_ltft) > 0 ? '+' : '' }}{{ t.avg_ltft }}%</b></span>
              <span style="color: var(--color-text-secondary)">λ {{ t.avg_lambda }}</span>
              <span style="color: var(--color-text-secondary)">{{ t.samples }} samples</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Trim Heatmap -->
    <div class="rounded-xl p-6 mb-4" :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }">
      <h2 class="text-base font-semibold mb-4">LTFT Map — RPM vs Load</h2>
      <DataEmptyState v-if="!rows.length" title="No trim data" message="Fuel trim map data will appear once enough OBD samples are collected." />
      <VChart v-else :option="trimHeatmapOption" autoresize style="height: 400px; width: 100%" />
    </div>

    <div class="rounded-xl px-5 py-4" :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }">
      <p class="text-[12px] leading-relaxed" style="color: var(--color-text-secondary)">
        <b>Reading LTFT on ethanol:</b> Positive LTFT means the ECU is adding fuel beyond its base map — expected on E{{ activeBlend }} since ethanol has a lower stoichiometric AFR ({{ stoichForBlend(activeBlend).toFixed(1) }} vs 14.7 for gasoline). LTFT over ±25% may indicate the ECU is running out of trim authority. Fuel flow is the limiting factor — if the injectors can't deliver enough at high RPM, the tune will pull timing or lean out.
      </p>
    </div>
  </div>
</template>
