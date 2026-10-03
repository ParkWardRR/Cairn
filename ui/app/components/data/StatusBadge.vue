<script setup lang="ts">
defineProps<{
  status: 'success' | 'warning' | 'danger' | 'info' | 'neutral'
  label: string
}>()

const colors = {
  success: 'var(--color-success)',
  warning: 'var(--color-warning)',
  danger: 'var(--color-danger)',
  info: 'var(--color-info)',
  neutral: 'var(--color-text-secondary)',
}
</script>

<template>
  <span class="inline-flex items-center gap-1.5 text-xs font-medium px-2 py-0.5 rounded-full"
    :style="{ backgroundColor: colors[status] + '18', color: colors[status] }">
    <span class="w-1.5 h-1.5 rounded-full" :style="{ backgroundColor: colors[status] }" />
    {{ label }}
  </span>
</template>
