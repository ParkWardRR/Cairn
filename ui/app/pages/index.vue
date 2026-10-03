<script setup lang="ts">
function formatDuration(seconds: number | null | undefined): string {
  if (!seconds || seconds <= 0) return '--'
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = Math.floor(seconds % 60)
  if (h > 0) return `${h}h ${m}m`
  return `${m}m ${s}s`
}

function formatDistance(meters: number | null | undefined): string {
  if (!meters || meters <= 0) return '--'
  return `${(meters / 1000).toFixed(1)} km`
}

function timeAgo(dateString: string | null | undefined): string {
  if (!dateString) return 'unknown'
  const now = Date.now()
  const then = new Date(dateString).getTime()
  const diffSeconds = Math.floor((now - then) / 1000)
  if (diffSeconds < 60) return 'just now'
  const diffMinutes = Math.floor(diffSeconds / 60)
  if (diffMinutes < 60) return `${diffMinutes}m ago`
  const diffHours = Math.floor(diffMinutes / 60)
  if (diffHours < 24) return `${diffHours}h ago`
  const diffDays = Math.floor(diffHours / 24)
  return `${diffDays}d ago`
}

function batteryStatus(mv: number | null | undefined): { label: string; status: string } {
  if (mv == null) return { label: 'N/A', status: 'info' }
  const v = mv / 1000
  if (v > 3.5) return { label: 'Good', status: 'success' }
  if (v > 3.2) return { label: 'Low', status: 'warning' }
  return { label: 'Critical', status: 'danger' }
}

function tempStatus(c: number | null | undefined): { label: string; status: string } {
  if (c == null) return { label: 'N/A', status: 'info' }
  if (c < 50) return { label: 'Normal', status: 'success' }
  if (c < 70) return { label: 'Warm', status: 'warning' }
  return { label: 'Hot', status: 'danger' }
}

function rssiStatus(dbm: number | null | undefined): { label: string; status: string } {
  if (dbm == null) return { label: 'N/A', status: 'info' }
  if (dbm > -70) return { label: 'Strong', status: 'success' }
  if (dbm > -85) return { label: 'Fair', status: 'warning' }
  return { label: 'Weak', status: 'danger' }
}

const { data: stats, pending: statsPending } = useFetch('/api/dashboard/stats')
const { data: recent, pending: recentPending } = useFetch<{
  trips: Array<{
    boot_id: string
    duration_s: number
    max_speed_kph: number
    start_lat: number
    start_lon: number
    end_lat: number
    end_lon: number
    start_time: string
  }>
}>('/api/dashboard/recent')
const { data: device, pending: devicePending } = useFetch<{
  battery_mv: number | null
  device_temp_c: number | null
  sd_free_mib: number | null
  rssi_dbm: number | null
  health_state: number
  observed_at: string
}>('/api/dashboard/device')

const lastTrip = computed(() => recent.value?.trips?.[0] ?? null)
</script>

<template>
  <div>
    <LayoutPageHeader title="Dashboard" subtitle="Vehicle telemetry overview" />

    <!-- Stat cards -->
    <div class="grid grid-cols-2 lg:grid-cols-4 gap-3 md:gap-4 mb-8">
      <template v-if="statsPending">
        <div v-for="i in 4" :key="i" class="skeleton h-[100px]" />
      </template>
      <template v-else-if="stats">
        <DataStatCard
          label="Total Trips"
          :value="String(stats.allTime?.total_trips ?? 0)"
          subtitle="all time"
        />
        <DataStatCard
          label="Distance"
          :value="formatDistance(stats.allTime?.total_distance_m)"
          subtitle="all time"
        />
        <DataStatCard
          label="Drive Time"
          :value="formatDuration(stats.allTime?.total_duration_s)"
          subtitle="all time"
        />
        <DataStatCard
          label="Top Speed"
          :value="stats.allTime?.max_speed_kph ? `${Math.round(stats.allTime.max_speed_kph)} kph` : '--'"
          subtitle="all time peak"
        />
      </template>
    </div>

    <!-- Two-column layout: Last Trip + Device Health -->
    <div class="grid grid-cols-1 lg:grid-cols-[3fr_2fr] gap-4 md:gap-6">

      <!-- Last Trip -->
      <div
        class="rounded-xl p-5 md:p-6"
        :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }"
      >
        <div class="flex items-center justify-between mb-5">
          <h2 class="text-base font-semibold">Last Trip</h2>
          <span v-if="lastTrip" class="text-xs font-medium px-2 py-0.5 rounded-full" style="background: var(--color-accent-soft); color: var(--color-accent)">
            {{ timeAgo(lastTrip.start_time) }}
          </span>
        </div>

        <template v-if="recentPending">
          <div class="grid grid-cols-3 gap-4">
            <div v-for="i in 3" :key="i" class="skeleton h-14" />
          </div>
        </template>

        <template v-else-if="lastTrip">
          <div class="grid grid-cols-3 gap-4 mb-5">
            <div>
              <p class="text-[11px] font-semibold uppercase tracking-wider mb-1.5" style="color: var(--color-text-secondary)">Duration</p>
              <p class="font-mono text-lg font-semibold">{{ formatDuration(lastTrip.duration_s) }}</p>
            </div>
            <div>
              <p class="text-[11px] font-semibold uppercase tracking-wider mb-1.5" style="color: var(--color-text-secondary)">Max Speed</p>
              <p class="font-mono text-lg font-semibold">{{ lastTrip.max_speed_kph }} <span class="text-xs font-normal" style="color: var(--color-text-secondary)">kph</span></p>
            </div>
            <div>
              <p class="text-[11px] font-semibold uppercase tracking-wider mb-1.5" style="color: var(--color-text-secondary)">Started</p>
              <p class="text-sm font-medium" style="color: var(--color-text-secondary)">{{ timeAgo(lastTrip.start_time) }}</p>
            </div>
          </div>

          <NuxtLink
            :to="`/trips/${lastTrip.boot_id}`"
            class="inline-flex items-center gap-1.5 text-[13px] font-semibold transition-all hover:gap-2.5"
            style="color: var(--color-accent)"
          >
            View trip details
            <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
              <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7" />
            </svg>
          </NuxtLink>
        </template>

        <DataEmptyState v-else title="No trips yet" message="Trip data will appear here once your first drive is recorded." />
      </div>

      <!-- Device Health -->
      <div
        class="rounded-xl p-5 md:p-6"
        :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }"
      >
        <div class="flex items-center justify-between mb-5">
          <h2 class="text-base font-semibold">Device Health</h2>
          <DataStatusBadge
            v-if="device"
            :status="device.battery_mv != null ? 'success' : 'neutral'"
            :label="device.battery_mv != null ? 'Online' : 'Offline'"
          />
        </div>

        <template v-if="devicePending">
          <div class="space-y-4">
            <div v-for="i in 4" :key="i" class="skeleton h-12" />
          </div>
        </template>

        <template v-else-if="device">
          <div class="space-y-3.5">
            <div
              v-for="(item, idx) in [
                { label: 'Battery', value: device.battery_mv != null ? `${(device.battery_mv / 1000).toFixed(2)} V` : '--', statusFn: () => batteryStatus(device.battery_mv) },
                { label: 'Temperature', value: device.device_temp_c != null ? `${device.device_temp_c} °C` : '--', statusFn: () => tempStatus(device.device_temp_c) },
                { label: 'SD Free', value: device.sd_free_mib != null ? `${device.sd_free_mib} MiB` : '--', statusFn: () => ({ label: (device.sd_free_mib ?? 0) > 100 ? 'OK' : (device.sd_free_mib ?? 0) > 20 ? 'Low' : 'Full', status: (device.sd_free_mib ?? 0) > 100 ? 'success' : (device.sd_free_mib ?? 0) > 20 ? 'warning' : 'danger' }) },
                { label: 'RSSI', value: device.rssi_dbm != null ? `${device.rssi_dbm} dBm` : '--', statusFn: () => rssiStatus(device.rssi_dbm) },
              ]"
              :key="item.label"
              class="flex items-center justify-between py-1.5 pb-3.5"
              :style="idx < 3 ? { borderBottom: '1px solid var(--color-border)' } : {}"
            >
              <div>
                <p class="text-[11px] font-semibold uppercase tracking-wider" style="color: var(--color-text-secondary)">{{ item.label }}</p>
                <p class="font-mono text-sm font-medium mt-1">{{ item.value }}</p>
              </div>
              <DataStatusBadge :status="item.statusFn().status" :label="item.statusFn().label" />
            </div>
          </div>

          <p class="text-[11px] mt-4 pt-3" :style="{ borderTop: '1px solid var(--color-border)', color: 'var(--color-text-secondary)' }">
            Last seen {{ timeAgo(device.observed_at) }}
          </p>
        </template>

        <DataEmptyState v-else title="No device data" message="Device health information will appear once the dongle connects." />
      </div>
    </div>
  </div>
</template>
