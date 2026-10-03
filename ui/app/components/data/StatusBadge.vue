<script setup lang="ts">
defineProps<{
  status: 'success' | 'warning' | 'danger' | 'info' | 'neutral'
  label: string
}>()

const colors: Record<string, string> = {
  success: 'var(--color-success)',
  warning: 'var(--color-warning)',
  danger: 'var(--color-danger)',
  info: 'var(--color-info)',
  neutral: 'var(--color-text-secondary)',
}
</script>

<template>
  <span
    class="inline-flex items-center gap-1.5 text-[11px] font-semibold px-2.5 py-1 rounded-full tracking-wide uppercase"
    :style="{ backgroundColor: colors[status] + '14', color: colors[status] }"
  >
    <span class="w-1.5 h-1.5 rounded-full animate-pulse" :style="{ backgroundColor: colors[status] }" />
    {{ label }}
  </span>
</template>
