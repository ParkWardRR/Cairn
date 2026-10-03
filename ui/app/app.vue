<script setup lang="ts">
const ui = useUiStore()

useHead({
  htmlAttrs: { class: () => ui.resolvedTheme === 'light' ? 'light' : '' },
})
</script>

<template>
  <div class="min-h-screen" style="background-color: var(--color-bg)">
    <LayoutSidebarNav />
    <main
      class="transition-all duration-200 min-h-screen"
      :class="ui.sidebarCollapsed ? 'ml-16' : 'ml-60'"
    >
      <div class="p-6">
        <NuxtPage />
      </div>
    </main>
  </div>
</template>
