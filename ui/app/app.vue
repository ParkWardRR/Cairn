<script setup lang="ts">
const ui = useUiStore()
const route = useRoute()

useHead({
  htmlAttrs: { class: () => ui.resolvedTheme === 'light' ? 'light' : '' },
})

const mobileNav = [
  { path: '/', label: 'Home', icon: 'M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-4 0a1 1 0 01-1-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 01-1 1' },
  { path: '/trips', label: 'Trips', icon: 'M9 20l-5.447-2.724A1 1 0 013 16.382V5.618a1 1 0 011.447-.894L9 7m0 13l6-3m-6 3V7m6 10l4.553 2.276A1 1 0 0021 18.382V7.618a1 1 0 00-.553-.894L15 4m0 13V4m0 0L9 7' },
  { path: '/analytics', label: 'Engine', icon: 'M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z' },
  { path: '/boost', label: 'Boost', icon: 'M13 10V3L4 14h7v7l9-11h-7z' },
  { path: '/system', label: 'Device', icon: 'M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z' },
]

function isActive(path: string) {
  if (path === '/') return route.path === '/'
  return route.path.startsWith(path)
}
</script>

<template>
  <div class="min-h-screen" style="background-color: var(--color-bg)">
    <!-- Desktop sidebar -->
    <LayoutSidebarNav class="hidden md:flex" />

    <!-- Main content -->
    <main
      class="min-h-screen transition-all duration-200 pb-20 md:pb-0"
      :class="ui.sidebarCollapsed ? 'md:ml-16' : 'md:ml-56'"
    >
      <div class="p-4 md:p-6 lg:p-8 max-w-[1400px]">
        <NuxtPage />
      </div>
    </main>

    <!-- Mobile bottom nav -->
    <nav
      class="fixed bottom-0 left-0 right-0 z-50 md:hidden flex items-center justify-around h-16 safe-bottom"
      :style="{
        backgroundColor: 'var(--color-surface)',
        borderTop: '1px solid var(--color-border)',
        backdropFilter: 'blur(12px)',
      }"
    >
      <NuxtLink
        v-for="item in mobileNav"
        :key="item.path"
        :to="item.path"
        class="flex flex-col items-center justify-center gap-0.5 w-16 py-1 transition-colors"
        :class="isActive(item.path)
          ? 'text-[var(--color-accent)]'
          : 'text-[var(--color-text-secondary)]'"
      >
        <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.75">
          <path stroke-linecap="round" stroke-linejoin="round" :d="item.icon" />
        </svg>
        <span class="text-[10px] font-medium">{{ item.label }}</span>
      </NuxtLink>
    </nav>
  </div>
</template>

<style scoped>
.safe-bottom {
  padding-bottom: env(safe-area-inset-bottom, 0px);
}
</style>
