<script setup lang="ts">
defineProps<{
  label: string
  value: string
  subtitle?: string
  trend?: 'up' | 'down' | 'flat'
  color?: 'default' | 'success' | 'warning' | 'danger'
}>()
</script>

<template>
  <div
    class="rounded-lg p-5 transition-colors"
    :style="{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }"
  >
    <p class="text-xs font-medium uppercase tracking-wider" style="color: var(--color-text-secondary)">
      {{ label }}
    </p>
    <p
      class="text-3xl font-bold mt-2 font-mono"
      :class="{
        'text-[var(--color-success)]': color === 'success',
        'text-[var(--color-warning)]': color === 'warning',
        'text-[var(--color-danger)]': color === 'danger',
      }"
    >
      {{ value }}
    </p>
    <div v-if="subtitle || trend" class="flex items-center gap-2 mt-2">
      <span v-if="trend" class="text-xs font-medium" :class="{
        'text-[var(--color-success)]': trend === 'up',
        'text-[var(--color-danger)]': trend === 'down',
        'text-[var(--color-text-secondary)]': trend === 'flat',
      }">
        {{ trend === 'up' ? '▲' : trend === 'down' ? '▼' : '—' }}
      </span>
      <span v-if="subtitle" class="text-sm" style="color: var(--color-text-secondary)">{{ subtitle }}</span>
    </div>
  </div>
</template>
