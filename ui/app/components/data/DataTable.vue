<script setup lang="ts">
interface Column {
  key: string
  label: string
  align?: 'left' | 'right' | 'center'
  mono?: boolean
}

const props = defineProps<{
  columns: Column[]
  rows: Record<string, any>[]
  clickable?: boolean
}>()

const emit = defineEmits<{
  rowClick: [row: Record<string, any>]
}>()
</script>

<template>
  <div class="rounded-lg overflow-hidden" :style="{ border: '1px solid var(--color-border)' }">
    <table class="w-full text-sm">
      <thead>
        <tr :style="{ backgroundColor: 'var(--color-surface-elevated)' }">
          <th
            v-for="col in columns"
            :key="col.key"
            class="px-4 py-3 font-medium text-xs uppercase tracking-wider"
            :class="col.align === 'right' ? 'text-right' : col.align === 'center' ? 'text-center' : 'text-left'"
            :style="{ color: 'var(--color-text-secondary)', borderBottom: '1px solid var(--color-border)' }"
          >
            {{ col.label }}
          </th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="(row, i) in rows"
          :key="i"
          class="transition-colors"
          :class="clickable ? 'cursor-pointer hover:bg-[var(--color-surface-elevated)]' : ''"
          :style="{ backgroundColor: 'var(--color-surface)', borderBottom: i < rows.length - 1 ? '1px solid var(--color-border)' : 'none' }"
          @click="clickable && emit('rowClick', row)"
        >
          <td
            v-for="col in columns"
            :key="col.key"
            class="px-4 py-3"
            :class="[
              col.align === 'right' ? 'text-right' : col.align === 'center' ? 'text-center' : 'text-left',
              col.mono ? 'font-mono' : '',
            ]"
          >
            <slot :name="col.key" :row="row" :value="row[col.key]">
              {{ row[col.key] ?? '—' }}
            </slot>
          </td>
        </tr>
        <tr v-if="rows.length === 0">
          <td :colspan="columns.length" class="px-4 py-8 text-center" style="color: var(--color-text-secondary)">
            No data
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
