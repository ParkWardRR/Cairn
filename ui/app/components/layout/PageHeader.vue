<script setup lang="ts">
defineProps<{
  title: string
  subtitle?: string
}>()
</script>

<template>
  <div class="flex items-center justify-between mb-6">
    <div>
      <h1 class="text-2xl font-semibold">{{ title }}</h1>
      <p v-if="subtitle" class="text-sm mt-1" style="color: var(--color-text-secondary)">{{ subtitle }}</p>
    </div>
    <slot name="actions" />
  </div>
</template>
