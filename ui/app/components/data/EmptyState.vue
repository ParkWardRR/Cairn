<script setup lang="ts">
defineProps<{
  title: string
  message?: string
}>()
</script>

<template>
  <div class="flex flex-col items-center justify-center py-16 text-center">
    <div
      class="w-14 h-14 rounded-2xl flex items-center justify-center mb-5"
      style="background: var(--color-surface-elevated); border: 1px solid var(--color-border)"
    >
      <slot name="icon">
        <svg class="w-6 h-6" style="color: var(--color-text-secondary)" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
          <path stroke-linecap="round" stroke-linejoin="round" d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4" />
        </svg>
      </slot>
    </div>
    <h3 class="text-sm font-semibold">{{ title }}</h3>
    <p v-if="message" class="text-[13px] mt-1.5 max-w-sm leading-relaxed" style="color: var(--color-text-secondary)">
      {{ message }}
    </p>
  </div>
</template>
