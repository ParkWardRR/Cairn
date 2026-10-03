<script setup lang="ts">
definePageMeta({ layout: 'default' })

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
  if (diffMinutes < 60) return `${diffMinutes} minute${diffMinutes === 1 ? '' : 's'} ago`
  const diffHours = Math.floor(diffMinutes / 60)
  if (diffHours < 24) return `${diffHours} hour${diffHours === 1 ? '' : 's'} ago`
  const diffDays = Math.floor(diffHours / 24)
  return `${diffDays} day${diffDays === 1 ? '' : 's'} ago`
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
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
      <template v-if="statsPending">
        <div v-for="i in 4" :key="i"
          class="rounded-lg p-5 animate-pulse h-28"
          :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }"
        />
      </template>
      <template v-else-if="stats">
        <DataStatCard
          label="Trips Today"
          :value="String(stats.today?.trips ?? 0)"
        />
        <DataStatCard
          label="Distance"
          :value="formatDistance(stats.allTime?.total_distance_m)"
        />
        <DataStatCard
          label="Drive Time"
          :value="formatDuration(stats.today?.duration_s)"
        />
        <DataStatCard
          label="Top Speed"
          :value="stats.allTime?.max_speed_kph ? `${Math.round(stats.allTime.max_speed_kph)} kph` : '--'"
        />
      </template>
    </div>

    <!-- Two-column layout: Last Trip + Device Health -->
    <div class="grid grid-cols-1 lg:grid-cols-[3fr_2fr] gap-6">

      <!-- Last Trip -->
      <div
        class="rounded-lg p-6"
        :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }"
      >
        <h2 class="text-lg font-semibold mb-4">Last Trip</h2>

        <template v-if="recentPending">
          <p class="text-sm" style="color: var(--color-text-secondary)">Loading...</p>
        </template>

        <template v-else-if="lastTrip">
          <div class="grid grid-cols-3 gap-4 mb-4">
            <div>
              <p class="text-xs uppercase tracking-wider mb-1" style="color: var(--color-text-secondary)">Duration</p>
              <p class="font-mono text-sm font-medium">{{ formatDuration(lastTrip.duration_s) }}</p>
            </div>
            <div>
              <p class="text-xs uppercase tracking-wider mb-1" style="color: var(--color-text-secondary)">Max Speed</p>
              <p class="font-mono text-sm font-medium">{{ lastTrip.max_speed_kph }} kph</p>
            </div>
            <div>
              <p class="text-xs uppercase tracking-wider mb-1" style="color: var(--color-text-secondary)">When</p>
              <p class="text-sm font-medium" style="color: var(--color-text-secondary)">{{ timeAgo(lastTrip.start_time) }}</p>
            </div>
          </div>

          <NuxtLink
            :to="`/trips/${lastTrip.boot_id}`"
            class="inline-flex items-center gap-1 text-sm font-medium transition-colors hover:underline"
            style="color: var(--color-accent)"
          >
            View trip details
            <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7" />
            </svg>
          </NuxtLink>
        </template>

        <DataEmptyState
          v-else
          title="No trips yet"
          message="Trip data will appear here once your first drive is recorded."
        />
      </div>

      <!-- Device Health -->
      <div
        class="rounded-lg p-6"
        :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }"
      >
        <h2 class="text-lg font-semibold mb-4">Device Health</h2>

        <template v-if="devicePending">
          <p class="text-sm" style="color: var(--color-text-secondary)">Loading...</p>
        </template>

        <template v-else-if="device">
          <div class="space-y-4">
            <!-- Battery -->
            <div class="flex items-center justify-between">
              <div>
                <p class="text-xs uppercase tracking-wider" style="color: var(--color-text-secondary)">Battery</p>
                <p class="font-mono text-sm font-medium mt-0.5">
                  {{ device.battery_mv != null ? `${(device.battery_mv / 1000).toFixed(2)} V` : '--' }}
                </p>
              </div>
              <DataStatusBadge
                :status="batteryStatus(device.battery_mv).status"
                :label="batteryStatus(device.battery_mv).label"
              />
            </div>

            <!-- Temperature -->
            <div class="flex items-center justify-between">
              <div>
                <p class="text-xs uppercase tracking-wider" style="color: var(--color-text-secondary)">Temperature</p>
                <p class="font-mono text-sm font-medium mt-0.5">
                  {{ device.device_temp_c != null ? `${device.device_temp_c} °C` : '--' }}
                </p>
              </div>
              <DataStatusBadge
                :status="tempStatus(device.device_temp_c).status"
                :label="tempStatus(device.device_temp_c).label"
              />
            </div>

            <!-- SD Free -->
            <div class="flex items-center justify-between">
              <div>
                <p class="text-xs uppercase tracking-wider" style="color: var(--color-text-secondary)">SD Free</p>
                <p class="font-mono text-sm font-medium mt-0.5">
                  {{ device.sd_free_mib != null ? `${device.sd_free_mib} MiB` : '--' }}
                </p>
              </div>
              <DataStatusBadge
                :status="(device.sd_free_mib ?? 0) > 100 ? 'success' : (device.sd_free_mib ?? 0) > 20 ? 'warning' : 'danger'"
                :label="(device.sd_free_mib ?? 0) > 100 ? 'OK' : (device.sd_free_mib ?? 0) > 20 ? 'Low' : 'Full'"
              />
            </div>

            <!-- RSSI -->
            <div class="flex items-center justify-between">
              <div>
                <p class="text-xs uppercase tracking-wider" style="color: var(--color-text-secondary)">RSSI</p>
                <p class="font-mono text-sm font-medium mt-0.5">
                  {{ device.rssi_dbm != null ? `${device.rssi_dbm} dBm` : '--' }}
                </p>
              </div>
              <DataStatusBadge
                :status="rssiStatus(device.rssi_dbm).status"
                :label="rssiStatus(device.rssi_dbm).label"
              />
            </div>
          </div>

          <p class="text-xs mt-5 pt-4" :style="{ borderTop: '1px solid var(--color-border)', color: 'var(--color-text-secondary)' }">
            Last seen {{ timeAgo(device.observed_at) }}
          </p>
        </template>

        <DataEmptyState
          v-else
          title="No device data"
          message="Device health information will appear once the dongle connects."
        />
      </div>
    </div>
  </div>
</template>
