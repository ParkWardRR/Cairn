<script setup lang="ts">
// The icon for a kind of place (home, work, gym...), drawn as an outline in the
// current text colour.
const props = withDefaults(defineProps<{ kind?: string | null; size?: number }>(), { kind: 'other', size: 16 })
const paths = computed(() => placeKind(props.kind).paths)
</script>

<template>
  <svg
    :width="size"
    :height="size"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="1.75"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <path v-for="(d, i) in paths" :key="i" :d="d" />
  </svg>
</template>
