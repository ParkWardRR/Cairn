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
    month: 'short', day: 'numeric', year: 'numeric',
    hour: 'numeric', minute: '2-digit', hour12: true,
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
    <LayoutPageHeader title="Trips" subtitle="Browse all recorded trips">
      <template #actions>
        <span v-if="trips.length" class="text-xs font-medium px-2.5 py-1 rounded-full" style="background: var(--color-accent-soft); color: var(--color-accent)">
          {{ trips.length }} trip{{ trips.length === 1 ? '' : 's' }}
        </span>
      </template>
    </LayoutPageHeader>

    <div v-if="status === 'pending'" class="space-y-2">
      <div v-for="i in 5" :key="i" class="skeleton h-14" />
    </div>

    <DataEmptyState
      v-else-if="trips.length === 0"
      title="No trips recorded"
      message="Trips will appear here once the device records driving data."
    />

    <div
      v-else
      class="rounded-xl overflow-hidden"
      :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }"
    >
      <table class="w-full text-sm">
        <thead>
          <tr :style="{ borderBottom: '1px solid var(--color-border)' }">
            <th class="text-left px-5 py-3 text-[11px] font-semibold uppercase tracking-wider" style="color: var(--color-text-secondary)">Date</th>
            <th class="text-left px-5 py-3 text-[11px] font-semibold uppercase tracking-wider" style="color: var(--color-text-secondary)">Duration</th>
            <th class="text-right px-5 py-3 text-[11px] font-semibold uppercase tracking-wider" style="color: var(--color-text-secondary)">Max Speed</th>
            <th class="text-right px-5 py-3 text-[11px] font-semibold uppercase tracking-wider hidden sm:table-cell" style="color: var(--color-text-secondary)">Max RPM</th>
            <th class="text-right px-5 py-3 text-[11px] font-semibold uppercase tracking-wider hidden md:table-cell" style="color: var(--color-text-secondary)">OBD</th>
            <th class="text-right px-5 py-3 text-[11px] font-semibold uppercase tracking-wider hidden md:table-cell" style="color: var(--color-text-secondary)">Gaps</th>
            <th class="w-10 px-3" />
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="trip in trips"
            :key="trip.boot_id"
            class="group cursor-pointer transition-colors hover:bg-[var(--color-surface-elevated)]"
            :style="{ borderBottom: '1px solid var(--color-border)' }"
            @click="navigateTo(`/trips/${trip.boot_id}`)"
          >
            <td class="px-5 py-3.5 text-[13px]">{{ formatDate(trip.start_time) }}</td>
            <td class="px-5 py-3.5 font-mono text-[13px]">{{ formatDuration(trip.duration_s) }}</td>
            <td class="px-5 py-3.5 font-mono text-[13px] text-right">{{ Math.round(trip.max_speed_kph / 1.60934) }} <span class="text-[11px]" style="color: var(--color-text-secondary)">mph</span></td>
            <td class="px-5 py-3.5 font-mono text-[13px] text-right hidden sm:table-cell">{{ trip.max_rpm.toLocaleString() }}</td>
            <td class="px-5 py-3.5 font-mono text-[13px] text-right hidden md:table-cell">{{ trip.obd_samples.toLocaleString() }}</td>
            <td class="px-5 py-3.5 font-mono text-[13px] text-right hidden md:table-cell">{{ trip.gap_count }}</td>
            <td class="px-3 py-3.5 text-right">
              <svg class="w-4 h-4 inline-block opacity-0 group-hover:opacity-50 transition-opacity" style="color: var(--color-text-secondary)" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7" />
              </svg>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
