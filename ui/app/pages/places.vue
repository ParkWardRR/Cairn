<script setup lang="ts">
definePageMeta({ layout: 'default' })
</script>

<template>
  <div>
    <LayoutPageHeader title="Places" subtitle="Saved locations and frequent stops" />

    <!-- Map placeholder -->
    <div
      class="w-full rounded-lg flex items-center justify-center mb-8"
      :style="{
        height: '400px',
        backgroundColor: 'var(--color-surface-elevated)',
        border: '1px solid var(--color-border)',
      }"
    >
      <p class="text-sm font-sans" style="color: var(--color-text-secondary)">
        Map loads here
      </p>
    </div>

    <!-- Coming soon card -->
    <div
      class="rounded-lg p-6"
      :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }"
    >
      <h2 class="text-lg font-semibold mb-2">Place management coming soon</h2>
      <p class="text-sm leading-relaxed" style="color: var(--color-text-secondary)">
        This feature requires a Postgres database for storing and managing saved places.
        The map will show your frequent stops, home/work locations, and custom waypoints.
      </p>
    </div>
  </div>
</template>
