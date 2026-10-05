<script setup lang="ts">
const { vehicles, selected, scope, effectiveId } = useVehicle()

// With one car there is nothing to choose, and trip detail is already one car.
const visible = computed(() => scope.value !== 'none' && vehicles.value.length > 1)

// Analysis pages never offer "All vehicles": they show the car they are using.
const value = computed({
  get: () => (scope.value === 'single' ? (effectiveId.value ?? '') : selected.value),
  set: (id: string) => { selected.value = id },
})

function label(v: { name: string; engine_code: string | null; archived: boolean }) {
  const engine = v.engine_code && !v.name.includes(v.engine_code) ? ` (${v.engine_code})` : ''
  return `${v.name}${engine}${v.archived ? ' - archived' : ''}`
}
</script>

<template>
  <select
    v-if="visible"
    v-model="value"
    aria-label="Vehicle"
    class="max-w-[16rem] truncate px-3 py-1.5 rounded-lg border text-[13px] font-medium cursor-pointer focus:ring-2 focus:ring-[var(--color-accent)] focus:outline-none"
    :style="{ backgroundColor: 'var(--color-surface)', borderColor: 'var(--color-border)', color: 'var(--color-text)' }"
  >
    <option v-if="scope === 'all'" value="">All vehicles</option>
    <option v-for="v in vehicles" :key="v.id" :value="v.id">{{ label(v) }}</option>
  </select>
</template>
