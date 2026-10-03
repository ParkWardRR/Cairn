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
    <slot name="actions" />
  </div>
</template>
