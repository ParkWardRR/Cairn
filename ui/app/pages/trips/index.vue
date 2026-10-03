<script setup lang="ts">
interface Trip {
  boot_id: string
  start_time: string
  duration_s: number
  max_speed_kph: number
  max_rpm: number
  obd_samples: number
  gap_count: number
}

const { data, status } = useFetch<{ trips: Trip[]; total: number }>('/api/trips')

const trips = computed(() => data.value?.trips ?? [])

function formatDate(iso: string | null): string {
  if (!iso) return '--'
  const d = new Date(iso)
  if (isNaN(d.getTime()) || d.getFullYear() < 2000) return '--'
  return d.toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
    hour12: true,
  })
}

function formatDuration(seconds: number | null): string {
  if (!seconds || seconds <= 0) return '--'
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = Math.floor(seconds % 60)
  if (h > 0) return `${h}h ${m}m`
  return `${m}m ${s}s`
}
</script>

<template>
  <div>
    <LayoutPageHeader title="Trips" subtitle="Browse all recorded trips" />

    <div v-if="status === 'pending'" class="flex items-center justify-center py-16">
      <div
        class="w-6 h-6 rounded-full border-2 animate-spin"
        :style="{
          borderColor: 'var(--color-border)',
          borderTopColor: 'var(--color-accent)',
        }"
      />
    </div>

    <DataEmptyState
      v-else-if="trips.length === 0"
      title="No trips recorded"
      message="Trips will appear here once the device records driving data."
    />

    <div
      v-else
      class="rounded-lg overflow-hidden"
      :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }"
    >
      <table class="w-full text-sm">
        <thead>
          <tr :style="{ borderBottom: '1px solid var(--color-border)' }">
            <th class="text-left px-4 py-3 text-xs font-medium uppercase tracking-wider" style="color: var(--color-text-secondary)">Date</th>
            <th class="text-left px-4 py-3 text-xs font-medium uppercase tracking-wider" style="color: var(--color-text-secondary)">Duration</th>
            <th class="text-right px-4 py-3 text-xs font-medium uppercase tracking-wider" style="color: var(--color-text-secondary)">Max Speed</th>
            <th class="text-right px-4 py-3 text-xs font-medium uppercase tracking-wider" style="color: var(--color-text-secondary)">Max RPM</th>
            <th class="text-right px-4 py-3 text-xs font-medium uppercase tracking-wider" style="color: var(--color-text-secondary)">OBD Samples</th>
            <th class="text-right px-4 py-3 text-xs font-medium uppercase tracking-wider" style="color: var(--color-text-secondary)">Gaps</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="trip in trips"
            :key="trip.boot_id"
            class="cursor-pointer transition-colors hover:bg-[var(--color-surface-elevated)]"
            :style="{ borderBottom: '1px solid var(--color-border)' }"
            @click="navigateTo(`/trips/${trip.boot_id}`)"
          >
            <td class="px-4 py-3">{{ formatDate(trip.start_time) }}</td>
            <td class="px-4 py-3 font-mono">{{ formatDuration(trip.duration_s) }}</td>
            <td class="px-4 py-3 font-mono text-right">{{ trip.max_speed_kph }} kph</td>
            <td class="px-4 py-3 font-mono text-right">{{ trip.max_rpm }}</td>
            <td class="px-4 py-3 font-mono text-right">{{ trip.obd_samples }}</td>
            <td class="px-4 py-3 font-mono text-right">{{ trip.gap_count }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
