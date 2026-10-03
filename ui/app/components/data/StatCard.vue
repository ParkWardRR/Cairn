<script setup lang="ts">
const props = defineProps<{
  label: string
  value: string
  subtitle?: string
  trend?: 'up' | 'down' | 'flat'
  color?: 'default' | 'success' | 'warning' | 'danger'
}>()

const accentBorder = computed(() => {
  const map: Record<string, string> = {
    success: 'var(--color-success)',
    warning: 'var(--color-warning)',
    danger: 'var(--color-danger)',
  }
  return map[props.color ?? ''] ?? 'transparent'
})
</script>

<template>
  <div
    class="group relative rounded-xl p-5 transition-all duration-200 hover:-translate-y-0.5"
    :style="{
      backgroundColor: 'var(--color-surface)',
      border: '1px solid var(--color-border)',
      borderTopColor: accentBorder !== 'transparent' ? accentBorder : undefined,
      borderTopWidth: accentBorder !== 'transparent' ? '2px' : undefined,
    }"
  >
    <p class="text-[11px] font-semibold uppercase tracking-wider" style="color: var(--color-text-secondary)">
      {{ label }}
    </p>
    <p
      class="text-2xl font-bold mt-2 font-mono tracking-tight"
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
        {{ trend === 'up' ? '↑' : trend === 'down' ? '↓' : '—' }}
      </span>
      <span v-if="subtitle" class="text-xs" style="color: var(--color-text-secondary)">{{ subtitle }}</span>
    </div>
  </div>
</template>
