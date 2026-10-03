<script setup lang="ts">
import 'echarts'
import VChart from 'vue-echarts'

interface OBDSample {
  mono_ms: number
  speed_kph: number
  rpm: number
  coolant_c: number | null
  intake_c: number | null
  throttle_pct: number | null
  load_pct: number | null
}

interface BoostSample {
  mono_ms: number
  boost_psi: number | null
  lambda_ratio: number | null
  stft_pct: number | null
  ltft_pct: number | null
}

interface GPSSample {
  mono_ms: number
  lat: number
  lon: number
  speed_mps: number | null
  sats_used: number | null
  fix_type: number | null
  hdop: number | null
  h_acc_m: number | null
  alt_m: number | null
}

const props = defineProps<{
  bootId: string
  firstObdMs?: number | null
  lastObdMs?: number | null
  firstFixMs?: number | null
}>()

const emit = defineEmits<{
  hover: [pos: { lat: number; lon: number; mono_ms: number } | null]
  select: [data: {
    mono_ms: number
    lat: number
    lon: number
    speed_mph: number | null
    rpm: number | null
    boost_psi: number | null
    coolant_f: number | null
    intake_f: number | null
    throttle_pct: number | null
    lambda: number | null
    sats: number | null
    alt_ft: number | null
  } | null]
}>()

const { data: timeline } = useFetch<{ obd: OBDSample[]; boost: BoostSample[]; gps: GPSSample[] }>(
  () => `/api/trips/${props.bootId}/timeline`,
)

const obd = computed(() => timeline.value?.obd ?? [])
const boost = computed(() => timeline.value?.boost ?? [])
const gps = computed(() => timeline.value?.gps ?? [])

const gpsWithFix = computed(() => gps.value.filter(g => g.lat !== 0 && g.lon !== 0))

const selectedPoint = ref<{
  mono_ms: number
  lat: number; lon: number
  speed_mph: number | null; rpm: number | null
  boost_psi: number | null; coolant_f: number | null; intake_f: number | null
  throttle_pct: number | null; lambda: number | null
  sats: number | null; alt_ft: number | null
  pctX: number
} | null>(null)

function findNearestObd(ms: number): OBDSample | null {
  if (!obd.value.length) return null
  let best = obd.value[0], bestDist = Math.abs(best.mono_ms - ms)
  for (const s of obd.value) {
    const d = Math.abs(s.mono_ms - ms)
    if (d < bestDist) { best = s; bestDist = d }
  }
  return bestDist < 10000 ? best : null
}

function findNearestGps(ms: number): GPSSample | null {
  if (!gpsWithFix.value.length) return null
  let best = gpsWithFix.value[0], bestDist = Math.abs(best.mono_ms - ms)
  for (const s of gpsWithFix.value) {
    const d = Math.abs(s.mono_ms - ms)
    if (d < bestDist) { best = s; bestDist = d }
  }
  return bestDist < 10000 ? best : null
}

function findNearestBoost(ms: number): BoostSample | null {
  if (!boost.value.length) return null
  let best = boost.value[0], bestDist = Math.abs(best.mono_ms - ms)
  for (const s of boost.value) {
    const d = Math.abs(s.mono_ms - ms)
    if (d < bestDist) { best = s; bestDist = d }
  }
  return bestDist < 10000 ? best : null
}

function buildPoint(ms: number, pctX: number) {
  const o = findNearestObd(ms)
  const b = findNearestBoost(ms)
  const g = findNearestGps(ms)
  if (!g) return null
  return {
    mono_ms: ms,
    lat: g.lat, lon: g.lon,
    speed_mph: o ? Math.round(o.speed_kph / 1.60934) : (g.speed_mps != null ? Math.round(g.speed_mps * 2.23694) : null),
    rpm: o?.rpm ?? null,
    boost_psi: b?.boost_psi != null ? Math.round(b.boost_psi * 10) / 10 : null,
    coolant_f: o?.coolant_c != null ? Math.round(o.coolant_c * 9/5 + 32) : null,
    intake_f: o?.intake_c != null ? Math.round(o.intake_c * 9/5 + 32) : null,
    throttle_pct: o?.throttle_pct ?? null,
    lambda: b?.lambda_ratio != null ? Math.round(b.lambda_ratio * 1000) / 1000 : null,
    sats: g.sats_used,
    alt_ft: g.alt_m != null ? Math.round(g.alt_m * 3.28084) : null,
    pctX,
  }
}

function onBarHover(e: MouseEvent) {
  const rect = (e.currentTarget as HTMLElement).getBoundingClientRect()
  const pct = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width))
  const startMs = props.firstObdMs ?? obd.value[0]?.mono_ms ?? 0
  const endMs = props.lastObdMs ?? obd.value[obd.value.length - 1]?.mono_ms ?? 1
  const ms = startMs + pct * (endMs - startMs)

  const g = findNearestGps(ms)
  if (g) {
    emit('hover', { lat: g.lat, lon: g.lon, mono_ms: ms })
  } else {
    emit('hover', null)
  }
}

function onBarLeave() {
  emit('hover', null)
}

function onBarClick(e: MouseEvent) {
  const rect = (e.currentTarget as HTMLElement).getBoundingClientRect()
  const pct = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width))
  const startMs = props.firstObdMs ?? obd.value[0]?.mono_ms ?? 0
  const endMs = props.lastObdMs ?? obd.value[obd.value.length - 1]?.mono_ms ?? 1
  const ms = startMs + pct * (endMs - startMs)
  const pt = buildPoint(ms, pct * 100)
  if (pt) {
    selectedPoint.value = selectedPoint.value?.mono_ms === pt.mono_ms ? null : pt
    emit('select', selectedPoint.value)
  }
}

function fmtMs(ms: number): string {
  const sec = ms / 1000
  if (sec < 60) return `${Math.round(sec)}s`
  return `${Math.floor(sec / 60)}m ${Math.round(sec % 60)}s`
}

function dismissSelection() {
  selectedPoint.value = null
  emit('select', null)
}

// GPS health chart
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
        areaStyle: {
          color: 'rgba(34, 197, 94, 0.15)',
          origin: 'start',
        },
        yAxisIndex: 0,
        silent: true,
      },
    ],
  }
})
</script>

<template>
  <div class="space-y-3">
    <!-- GPS Health sparkline -->
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

    <!-- Interactive timeline bar -->
    <div class="rounded-xl p-4" :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }">
      <div class="flex items-center justify-between mb-2">
        <span class="text-[11px] font-semibold uppercase tracking-wider" style="color: var(--color-text-secondary)">Trip Timeline</span>
        <span class="text-[10px]" style="color: var(--color-text-secondary)">Hover to locate · Click for details</span>
      </div>

      <div
        class="relative h-8 rounded-full overflow-hidden cursor-crosshair select-none"
        style="background: var(--color-surface-elevated)"
        @mousemove="onBarHover"
        @mouseleave="onBarLeave"
        @click="onBarClick"
      >
        <!-- Blind segment -->
        <div
          v-if="firstObdMs != null && firstFixMs != null && firstFixMs > firstObdMs && lastObdMs != null"
          class="absolute inset-y-0 left-0 rounded-l-full"
          style="background: linear-gradient(90deg, #f59e0b, #ef4444); opacity: 0.6"
          :style="{ width: ((firstFixMs - firstObdMs) / (lastObdMs - firstObdMs) * 100) + '%' }"
        />
        <!-- Tracked segment -->
        <div
          v-if="firstObdMs != null && firstFixMs != null && firstFixMs > firstObdMs && lastObdMs != null"
          class="absolute inset-y-0 rounded-r-full"
          style="background: linear-gradient(90deg, #22c55e, #3b82f6); opacity: 0.7"
          :style="{
            left: ((firstFixMs - firstObdMs) / (lastObdMs - firstObdMs) * 100) + '%',
            width: (100 - (firstFixMs - firstObdMs) / (lastObdMs - firstObdMs) * 100) + '%',
          }"
        />
        <!-- Full tracked if instant fix -->
        <div
          v-else-if="firstObdMs != null && lastObdMs != null"
          class="absolute inset-0 rounded-full"
          style="background: linear-gradient(90deg, #22c55e, #3b82f6); opacity: 0.7"
        />

        <!-- GPS sample ticks -->
        <template v-if="firstObdMs != null && lastObdMs != null">
          <div
            v-for="(g, i) in gpsWithFix.filter((_, idx) => idx % 5 === 0)"
            :key="i"
            class="absolute top-0 bottom-0 w-px"
            style="background: rgba(255,255,255,0.12)"
            :style="{ left: ((g.mono_ms - firstObdMs) / (lastObdMs - firstObdMs) * 100) + '%' }"
          />
        </template>

        <!-- Selected point indicator -->
        <div
          v-if="selectedPoint"
          class="absolute top-0 bottom-0 w-0.5 z-10"
          style="background: white; box-shadow: 0 0 6px rgba(255,255,255,0.6)"
          :style="{ left: selectedPoint.pctX + '%' }"
        />
      </div>

      <div v-if="firstObdMs != null && lastObdMs != null" class="flex justify-between mt-1 text-[9px] font-mono" style="color: var(--color-text-secondary)">
        <span>0s</span>
        <span>{{ fmtMs((lastObdMs - firstObdMs) / 2) }}</span>
        <span>{{ fmtMs(lastObdMs - firstObdMs) }}</span>
      </div>
    </div>

    <!-- Selected point detail card -->
    <Transition name="fade">
      <div
        v-if="selectedPoint"
        class="rounded-xl p-4 relative"
        :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-accent)', boxShadow: '0 0 12px rgba(59, 130, 246, 0.1)' }"
      >
        <button
          class="absolute top-3 right-3 w-5 h-5 rounded-full flex items-center justify-center text-[11px] transition-colors hover:bg-[var(--color-surface-elevated)]"
          style="color: var(--color-text-secondary)"
          @click="dismissSelection"
        >
          &times;
        </button>

        <div class="flex items-center gap-2 mb-3">
          <span class="text-[11px] font-semibold uppercase tracking-wider" style="color: var(--color-accent)">
            {{ fmtMs(selectedPoint.mono_ms - (firstObdMs ?? 0)) }} into trip
          </span>
        </div>

        <div class="grid grid-cols-3 sm:grid-cols-4 lg:grid-cols-6 gap-3">
          <div v-if="selectedPoint.speed_mph != null">
            <p class="text-[10px] font-semibold uppercase" style="color: var(--color-text-secondary)">Speed</p>
            <p class="font-mono text-sm font-semibold">{{ selectedPoint.speed_mph }} <span class="text-[10px] font-normal" style="color: var(--color-text-secondary)">mph</span></p>
          </div>
          <div v-if="selectedPoint.rpm != null">
            <p class="text-[10px] font-semibold uppercase" style="color: var(--color-text-secondary)">RPM</p>
            <p class="font-mono text-sm font-semibold">{{ selectedPoint.rpm.toLocaleString() }}</p>
          </div>
          <div v-if="selectedPoint.boost_psi != null">
            <p class="text-[10px] font-semibold uppercase" style="color: var(--color-text-secondary)">Boost</p>
            <p class="font-mono text-sm font-semibold" :style="{ color: selectedPoint.boost_psi > 15 ? '#f59e0b' : 'inherit' }">{{ selectedPoint.boost_psi }} <span class="text-[10px] font-normal" style="color: var(--color-text-secondary)">psi</span></p>
          </div>
          <div v-if="selectedPoint.coolant_f != null">
            <p class="text-[10px] font-semibold uppercase" style="color: var(--color-text-secondary)">Coolant</p>
            <p class="font-mono text-sm font-semibold" :style="{ color: selectedPoint.coolant_f > 220 ? '#ef4444' : 'inherit' }">{{ selectedPoint.coolant_f }}°F</p>
          </div>
          <div v-if="selectedPoint.intake_f != null">
            <p class="text-[10px] font-semibold uppercase" style="color: var(--color-text-secondary)">IAT</p>
            <p class="font-mono text-sm font-semibold">{{ selectedPoint.intake_f }}°F</p>
          </div>
          <div v-if="selectedPoint.throttle_pct != null">
            <p class="text-[10px] font-semibold uppercase" style="color: var(--color-text-secondary)">Throttle</p>
            <p class="font-mono text-sm font-semibold">{{ selectedPoint.throttle_pct }}%</p>
          </div>
          <div v-if="selectedPoint.lambda != null">
            <p class="text-[10px] font-semibold uppercase" style="color: var(--color-text-secondary)">Lambda</p>
            <p class="font-mono text-sm font-semibold">{{ selectedPoint.lambda }}</p>
          </div>
          <div v-if="selectedPoint.sats != null">
            <p class="text-[10px] font-semibold uppercase" style="color: var(--color-text-secondary)">Sats</p>
            <p class="font-mono text-sm font-semibold">{{ selectedPoint.sats }}</p>
          </div>
          <div v-if="selectedPoint.alt_ft != null">
            <p class="text-[10px] font-semibold uppercase" style="color: var(--color-text-secondary)">Elevation</p>
            <p class="font-mono text-sm font-semibold">{{ selectedPoint.alt_ft }} <span class="text-[10px] font-normal" style="color: var(--color-text-secondary)">ft</span></p>
          </div>
        </div>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.fade-enter-active, .fade-leave-active { transition: opacity 0.15s ease; }
.fade-enter-from, .fade-leave-to { opacity: 0; }
</style>
