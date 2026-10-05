<script setup lang="ts">
defineProps<{
  title: string
  subtitle?: string
}>()
</script>

<template>
  <div class="flex items-center justify-between mb-8">
    <div>
      <h1 class="text-xl font-semibold tracking-tight">{{ title }}</h1>
      <p v-if="subtitle" class="text-[13px] mt-1" style="color: var(--color-text-secondary)">{{ subtitle }}</p>
    </div>
    <div class="flex items-center gap-3">
      <LayoutVehicleSelector />
      <slot name="actions" />
    </div>
  </div>
</template>
