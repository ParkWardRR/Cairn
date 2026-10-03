<script setup lang="ts">
definePageMeta({ layout: 'default' })
</script>

<template>
  <div>
    <LayoutPageHeader title="Places" subtitle="Saved locations and frequent stops" />

    <!-- Map placeholder -->
    <div
      class="w-full rounded-xl flex items-center justify-center mb-8"
      :style="{
        height: '400px',
        backgroundColor: 'var(--color-surface-elevated)',
        border: '1px solid var(--color-border)',
      }"
    >
      <div class="text-center">
        <svg class="w-8 h-8 mx-auto mb-2" style="color: var(--color-text-secondary)" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
          <path stroke-linecap="round" stroke-linejoin="round" d="M17.657 16.657L13.414 20.9a1.998 1.998 0 01-2.827 0l-4.244-4.243a8 8 0 1111.314 0z M15 11a3 3 0 11-6 0 3 3 0 016 0z" />
        </svg>
        <p class="text-[13px]" style="color: var(--color-text-secondary)">
          Map view
        </p>
      </div>
    </div>

    <!-- Coming soon card -->
    <div
      class="rounded-xl p-6"
      :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }"
    >
      <h2 class="text-base font-semibold mb-2">Place management coming soon</h2>
      <p class="text-[13px] leading-relaxed" style="color: var(--color-text-secondary)">
        This feature requires a Postgres database for storing and managing saved places.
        The map will show your frequent stops, home/work locations, and custom waypoints.
      </p>
    </div>
  </div>
</template>
