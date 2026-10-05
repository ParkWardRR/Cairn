<script setup lang="ts">
// A video-style scrubber for a trip: drag or click to jump anywhere in it, press
// play to watch the car drive the route. The track shows speed over time, the
// stretch before the first GPS fix, and every stop; the map follows the playhead.

interface OBDSample {
  mono_ms: number
  speed_kph: number
  rpm: number
  coolant_c: number | null
  throttle_pct: number | null
}

interface BoostSample {
  mono_ms: number
  boost_psi: number | null
  lambda_ratio: number | null
}

interface GPSSample {
  mono_ms: number
  lat: number
  lon: number
  speed_mps: number | null
  alt_m: number | null
}

interface ScrubStop {
  start_mono_ms: number
  end_mono_ms: number
  category: StopCategory
  inferred: boolean
  place?: { name: string | null } | null
}

const props = defineProps<{
  bootId: string
  stops?: ScrubStop[]
  selectedStop?: number | null
  // The playhead, in the trip's monotonic milliseconds.
  modelValue: number | null
  // Wall-clock ms at mono_ms = 0, when the trip has a trustworthy UTC basis.
  wallBaseMs?: number | null
  firstFixMs?: number | null
}>()

const emit = defineEmits<{
  'update:modelValue': [ms: number]
  position: [pos: { lat: number; lon: number; mono_ms: number } | null]
  hover: [pos: { lat: number; lon: number; mono_ms: number } | null]
  'select-stop': [index: number]
  playing: [playing: boolean]
}>()

const { data: timeline } = useFetch<{ obd: OBDSample[]; boost: BoostSample[]; gps: GPSSample[] }>(
  () => `/api/trips/${props.bootId}/timeline`,
)

const obd = computed(() => timeline.value?.obd ?? [])
const boost = computed(() => timeline.value?.boost ?? [])
const fixes = computed(() => (timeline.value?.gps ?? []).filter(g => g.lat !== 0 && g.lon !== 0))

// ---- time range ----

const startMs = computed(() => {
  const xs = [obd.value[0]?.mono_ms, boost.value[0]?.mono_ms, timeline.value?.gps?.[0]?.mono_ms].filter((x): x is number => x != null)
  return xs.length ? Math.min(...xs) : 0
})
const endMs = computed(() => {
  const xs = [obd.value.at(-1)?.mono_ms, boost.value.at(-1)?.mono_ms, timeline.value?.gps?.at(-1)?.mono_ms].filter((x): x is number => x != null)
  return xs.length ? Math.max(...xs) : 1
})
const duration = computed(() => Math.max(1, endMs.value - startMs.value))

const pct = (ms: number) => Math.max(0, Math.min(100, ((ms - startMs.value) / duration.value) * 100))

// ---- lookups ----

// Index of the last element at or before ms, or -1.
function floorIndex<T extends { mono_ms: number }>(arr: T[], ms: number): number {
  let lo = 0
  let hi = arr.length - 1
  let ans = -1
  while (lo <= hi) {
    const mid = (lo + hi) >> 1
    if (arr[mid].mono_ms <= ms) { ans = mid; lo = mid + 1 } else hi = mid - 1
  }
  return ans
}

function nearest<T extends { mono_ms: number }>(arr: T[], ms: number, within = 10_000): T | null {
  if (!arr.length) return null
  const i = floorIndex(arr, ms)
  const cands = [arr[i], arr[i + 1]].filter(Boolean) as T[]
  let best: T | null = null
  for (const c of cands) if (!best || Math.abs(c.mono_ms - ms) < Math.abs(best.mono_ms - ms)) best = c
  return best && Math.abs(best.mono_ms - ms) <= within ? best : null
}

// Where the car was at ms. Fixes close together are interpolated; across a long
// silence the car is held at its last fix (it was parked or the receiver was
// out). Before the first fix there is no position.
function positionAt(ms: number): { lat: number; lon: number } | null {
  const f = fixes.value
  if (!f.length || ms < f[0].mono_ms) return null
  const i = floorIndex(f, ms)
  const a = f[i]
  const b = f[i + 1]
  if (!b || b.mono_ms - a.mono_ms > 30_000) return { lat: a.lat, lon: a.lon }
  const k = (ms - a.mono_ms) / (b.mono_ms - a.mono_ms)
  return { lat: a.lat + (b.lat - a.lat) * k, lon: a.lon + (b.lon - a.lon) * k }
}

function speedMphAt(ms: number): number | null {
  const o = nearest(obd.value, ms)
  if (o) return Math.round(o.speed_kph / 1.60934)
  const g = nearest(fixes.value, ms)
  return g?.speed_mps != null ? Math.round(g.speed_mps * 2.23694) : null
}

const readout = computed(() => {
  if (t.value == null) return null
  const ms = t.value
  const o = nearest(obd.value, ms)
  const b = nearest(boost.value, ms)
  const g = nearest(fixes.value, ms, 30_000)
  return {
    mph: speedMphAt(ms),
    rpm: o?.rpm ?? null,
    boost: b?.boost_psi != null ? Math.round(b.boost_psi * 10) / 10 : null,
    coolantF: o?.coolant_c != null ? Math.round(o.coolant_c * 9 / 5 + 32) : null,
    throttle: o?.throttle_pct ?? null,
    lambda: b?.lambda_ratio != null ? Math.round(b.lambda_ratio * 1000) / 1000 : null,
    altFt: g?.alt_m != null ? Math.round(g.alt_m * 3.28084) : null,
  }
})

// ---- speed trace ----

const BUCKETS = 400

const trace = computed(() => {
  const peaks: Array<number | null> = new Array(BUCKETS).fill(null)
  for (const g of fixes.value) {
    if (g.speed_mps == null) continue
    const i = Math.min(BUCKETS - 1, Math.floor(((g.mono_ms - startMs.value) / duration.value) * BUCKETS))
    peaks[i] = Math.max(peaks[i] ?? 0, g.speed_mps)
  }
  const max = Math.max(5, ...peaks.filter((x): x is number => x != null))
  const y = (v: number) => 100 - (v / max) * 88

  // Join buckets that have data; break the line across long silences so a
  // parked stretch reads as empty rather than as zero speed.
  let line = ''
  let area = ''
  let run: Array<[number, number]> = []
  let misses = 0
  const flush = () => {
    if (run.length > 1) {
      line += 'M' + run.map(([x, v]) => `${x} ${y(v)}`).join(' L')
      area += `M${run[0][0]} 100 L` + run.map(([x, v]) => `${x} ${y(v)}`).join(' L') + ` L${run.at(-1)![0]} 100 Z`
    }
    run = []
  }
  peaks.forEach((v, i) => {
    if (v == null) { if (++misses > 6) flush(); return }
    misses = 0
    run.push([i + 0.5, v])
  })
  flush()
  return { line, area }
})

// ---- playhead ----

const t = ref<number | null>(props.modelValue)
watch(() => props.modelValue, (v) => { if (v !== t.value) t.value = v })

function setT(ms: number, silent = false) {
  const c = Math.max(startMs.value, Math.min(endMs.value, ms))
  t.value = c
  if (!silent) emit('update:modelValue', c)
}

watch(t, (ms) => {
  if (ms == null) { emit('position', null); return }
  const p = positionAt(ms)
  emit('position', p ? { ...p, mono_ms: ms } : null)
})

// ---- playback ----

const RATES = [10, 30, 60, 120]
const rate = ref(30)
const playing = ref(false)
let raf = 0
let last = 0

function tick(now: number) {
  if (!playing.value) return
  const dt = now - last
  last = now
  const next = (t.value ?? startMs.value) + dt * rate.value
  if (next >= endMs.value) {
    setT(endMs.value)
    pause()
    return
  }
  setT(next)
  raf = requestAnimationFrame(tick)
}

function play() {
  if (!timeline.value) return
  if (t.value == null || t.value >= endMs.value - 50) setT(startMs.value)
  playing.value = true
  emit('playing', true)
  last = performance.now()
  raf = requestAnimationFrame(tick)
}

function pause() {
  playing.value = false
  emit('playing', false)
  cancelAnimationFrame(raf)
}

function toggle() { playing.value ? pause() : play() }

onUnmounted(() => cancelAnimationFrame(raf))

// ---- pointer scrubbing ----

const trackEl = ref<HTMLDivElement>()
const dragging = ref(false)
const hoverMs = ref<number | null>(null)
const hoverX = ref(0)

function msFromEvent(e: PointerEvent): number {
  const r = trackEl.value!.getBoundingClientRect()
  const k = Math.max(0, Math.min(1, (e.clientX - r.left) / r.width))
  return startMs.value + k * duration.value
}

function onDown(e: PointerEvent) {
  if (!timeline.value) return
  trackEl.value?.setPointerCapture(e.pointerId)
  dragging.value = true
  setT(msFromEvent(e))
}

function onMove(e: PointerEvent) {
  const ms = msFromEvent(e)
  const r = trackEl.value!.getBoundingClientRect()
  hoverX.value = e.clientX - r.left
  hoverMs.value = ms
  if (dragging.value) { setT(ms); return }
  const p = positionAt(ms)
  emit('hover', p ? { ...p, mono_ms: ms } : null)
}

function onUp(e: PointerEvent) {
  dragging.value = false
  trackEl.value?.releasePointerCapture(e.pointerId)
}

function onLeave() {
  if (!dragging.value) { hoverMs.value = null; emit('hover', null) }
}

function onKey(e: KeyboardEvent) {
  const step = e.shiftKey ? 60_000 : 5_000
  const cur = t.value ?? startMs.value
  if (e.key === ' ' || e.key === 'k') { e.preventDefault(); toggle() }
  else if (e.key === 'ArrowRight') { e.preventDefault(); setT(cur + step) }
  else if (e.key === 'ArrowLeft') { e.preventDefault(); setT(cur - step) }
  else if (e.key === 'Home') { e.preventDefault(); setT(startMs.value) }
  else if (e.key === 'End') { e.preventDefault(); setT(endMs.value) }
}

function seekStop(i: number) {
  const s = props.stops?.[i]
  if (!s) return
  emit('select-stop', i)
  setT(s.start_mono_ms)
}

// ---- formatting ----

function fmtElapsed(ms: number): string {
  const s = Math.max(0, Math.round((ms - startMs.value) / 1000))
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const sec = s % 60
  return h ? `${h}:${String(m).padStart(2, '0')}:${String(sec).padStart(2, '0')}` : `${m}:${String(sec).padStart(2, '0')}`
}

function fmtWall(ms: number): string | null {
  if (props.wallBaseMs == null) return null
  return new Date(props.wallBaseMs + ms).toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit', second: '2-digit' })
}

const blindPct = computed(() => {
  if (props.firstFixMs == null || props.firstFixMs <= startMs.value) return 0
  return pct(props.firstFixMs)
})

const ticks = computed(() => [0, 0.25, 0.5, 0.75, 1].map(k => ({ k, label: fmtElapsed(startMs.value + k * duration.value) })))

function stopLabel(s: ScrubStop): string {
  const mins = Math.round((s.end_mono_ms - s.start_mono_ms) / 60000)
  return `${STOP_LABEL[s.category]} stop · ${mins} min${s.place?.name ? ` · ${s.place.name}` : ''}`
}
</script>

<template>
  <div class="rounded-xl px-4 pt-3 pb-2.5" :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }">
    <!-- Transport -->
    <div class="flex items-center gap-3 mb-2.5">
      <button
        type="button"
        class="w-9 h-9 rounded-full flex items-center justify-center shrink-0 transition-colors hover:brightness-110 disabled:opacity-40"
        style="background: var(--color-accent); color: #fff"
        :disabled="!timeline"
        :aria-label="playing ? 'Pause' : 'Play'"
        @click="toggle"
      >
        <svg v-if="!playing" class="w-4 h-4 ml-0.5" viewBox="0 0 24 24" fill="currentColor"><path d="M8 5v14l11-7z" /></svg>
        <svg v-else class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor"><path d="M6 5h4v14H6zM14 5h4v14h-4z" /></svg>
      </button>

      <div class="font-mono text-[13px] tabular-nums">
        <span class="font-semibold">{{ fmtElapsed(t ?? startMs) }}</span>
        <span style="color: var(--color-text-secondary)"> / {{ fmtElapsed(endMs) }}</span>
        <span v-if="t != null && fmtWall(t)" class="ml-2 text-[11px]" style="color: var(--color-text-secondary)">{{ fmtWall(t) }}</span>
      </div>

      <div class="ml-auto flex items-center gap-1">
        <span class="text-[10px] uppercase tracking-wider mr-1" style="color: var(--color-text-secondary)">Playback</span>
        <button
          v-for="r in RATES"
          :key="r"
          type="button"
          class="px-1.5 py-0.5 rounded text-[11px] font-mono transition-colors"
          :style="{ background: rate === r ? 'var(--color-accent-soft)' : 'transparent', color: rate === r ? 'var(--color-accent)' : 'var(--color-text-secondary)' }"
          @click="rate = r"
        >{{ r }}×</button>
      </div>
    </div>

    <!-- Track -->
    <div
      ref="trackEl"
      class="relative h-16 rounded-lg overflow-hidden cursor-pointer select-none touch-none outline-none focus-visible:ring-2"
      style="background: var(--color-surface-elevated); --tw-ring-color: var(--color-accent)"
      tabindex="0"
      role="slider"
      aria-label="Trip position"
      :aria-valuemin="startMs"
      :aria-valuemax="endMs"
      :aria-valuenow="t ?? startMs"
      :aria-valuetext="fmtElapsed(t ?? startMs)"
      @pointerdown="onDown"
      @pointermove="onMove"
      @pointerup="onUp"
      @pointercancel="onUp"
      @pointerleave="onLeave"
      @keydown="onKey"
    >
      <!-- Before the first GPS fix -->
      <div
        v-if="blindPct > 0"
        class="absolute inset-y-0 left-0"
        :style="{
          width: blindPct + '%',
          background: 'repeating-linear-gradient(45deg, rgba(245,158,11,.18) 0 6px, rgba(245,158,11,.06) 6px 12px)',
        }"
        title="No GPS fix yet"
      />

      <!-- Speed over time -->
      <svg class="absolute inset-0 w-full h-full" viewBox="0 0 400 100" preserveAspectRatio="none">
        <path :d="trace.area" fill="rgba(59,130,246,.22)" />
        <path :d="trace.line" fill="none" stroke="#3b82f6" stroke-width="1.5" vector-effect="non-scaling-stroke" stroke-linejoin="round" />
      </svg>

      <!-- Played so far -->
      <div
        v-if="t != null"
        class="absolute inset-y-0 left-0 pointer-events-none"
        style="background: rgba(255,255,255,.07); border-right: 1px solid rgba(255,255,255,.0)"
        :style="{ width: pct(t) + '%' }"
      />

      <!-- Stops -->
      <div class="absolute left-0 right-0 bottom-0 h-3.5 pointer-events-none">
        <button
          v-for="(s, i) in stops"
          :key="i"
          type="button"
          class="absolute bottom-0 h-3.5 rounded-t-[3px] pointer-events-auto transition-[filter]"
          :style="{
            left: pct(s.start_mono_ms) + '%',
            width: `max(6px, ${pct(s.end_mono_ms) - pct(s.start_mono_ms)}%)`,
            background: STOP_COLOR[s.category],
            opacity: s.inferred ? 0.65 : 1,
            outline: selectedStop === i ? '2px solid #fff' : 'none',
            zIndex: selectedStop === i ? 2 : 1,
          }"
          :title="stopLabel(s)"
          @pointerdown.stop
          @click.stop="seekStop(i)"
        />
      </div>

      <!-- Hover preview -->
      <div
        v-if="hoverMs != null && !dragging"
        class="absolute inset-y-0 w-px pointer-events-none"
        style="background: rgba(255,255,255,.35)"
        :style="{ left: hoverX + 'px' }"
      />

      <!-- Playhead -->
      <div
        v-if="t != null"
        class="absolute inset-y-0 w-0.5 pointer-events-none z-10"
        style="background: #fff; box-shadow: 0 0 6px rgba(255,255,255,.7)"
        :style="{ left: `calc(${pct(t)}% - 1px)` }"
      >
        <span class="absolute -top-0 -left-[5px] w-3 h-3 rounded-full" style="background: #fff; box-shadow: 0 0 6px rgba(0,0,0,.6)" />
      </div>
    </div>

    <!-- Hover tooltip, outside the track so it is not clipped -->
    <div class="relative h-0">
      <div
        v-if="hoverMs != null && !dragging"
        class="absolute -top-[88px] -translate-x-1/2 px-2 py-1 rounded-md text-[11px] font-mono pointer-events-none z-20 whitespace-nowrap"
        style="background: rgba(15,17,23,.95); border: 1px solid var(--color-border)"
        :style="{ left: Math.max(40, hoverX) + 'px' }"
      >
        {{ fmtElapsed(hoverMs) }}<template v-if="speedMphAt(hoverMs) != null"> · {{ speedMphAt(hoverMs) }} mph</template>
      </div>
    </div>

    <!-- Time ticks -->
    <div class="flex justify-between mt-1 text-[9px] font-mono" style="color: var(--color-text-secondary)">
      <span v-for="tk in ticks" :key="tk.k">{{ tk.label }}</span>
    </div>

    <!-- Telemetry at the playhead -->
    <div v-if="readout" class="flex flex-wrap items-center gap-x-5 gap-y-1 mt-2 pt-2 text-[12px]" style="border-top: 1px solid var(--color-border)">
      <span v-if="readout.mph != null"><span class="text-[10px] uppercase" style="color: var(--color-text-secondary)">Speed </span><span class="font-mono font-semibold">{{ readout.mph }}</span> <span class="text-[10px]" style="color: var(--color-text-secondary)">mph</span></span>
      <span v-if="readout.rpm != null"><span class="text-[10px] uppercase" style="color: var(--color-text-secondary)">RPM </span><span class="font-mono font-semibold">{{ readout.rpm.toLocaleString() }}</span></span>
      <span v-if="readout.boost != null"><span class="text-[10px] uppercase" style="color: var(--color-text-secondary)">Boost </span><span class="font-mono font-semibold">{{ readout.boost }}</span> <span class="text-[10px]" style="color: var(--color-text-secondary)">psi</span></span>
      <span v-if="readout.coolantF != null"><span class="text-[10px] uppercase" style="color: var(--color-text-secondary)">Coolant </span><span class="font-mono font-semibold">{{ readout.coolantF }}°F</span></span>
      <span v-if="readout.throttle != null"><span class="text-[10px] uppercase" style="color: var(--color-text-secondary)">Throttle </span><span class="font-mono font-semibold">{{ readout.throttle }}%</span></span>
      <span v-if="readout.lambda != null"><span class="text-[10px] uppercase" style="color: var(--color-text-secondary)">Lambda </span><span class="font-mono font-semibold">{{ readout.lambda }}</span></span>
      <span v-if="readout.altFt != null"><span class="text-[10px] uppercase" style="color: var(--color-text-secondary)">Elev </span><span class="font-mono font-semibold">{{ readout.altFt }}</span> <span class="text-[10px]" style="color: var(--color-text-secondary)">ft</span></span>
    </div>
    <p v-else class="mt-2 text-[11px]" style="color: var(--color-text-secondary)">
      Drag along the track or press play. Space plays or pauses, ← → step 5 s (Shift for 60 s).
    </p>
  </div>
</template>
