<script lang="ts">
  import { page } from '$app/state';

  const links = [
    { href: '/', label: 'Today', icon: 'today' },
    { href: '/trips', label: 'Trips', icon: 'map' },
    { href: '/places', label: 'Places', icon: 'pin' },
    { href: '/devices', label: 'Devices', icon: 'cpu' },
    { href: '/settings', label: 'Privacy', icon: 'shield' },
  ] as const;

  function isActive(href: string, path: string): boolean {
    if (href === '/') return path === '/';
    return path.startsWith(href);
  }
</script>

<!-- Mobile: iOS tab bar at bottom -->
<nav class="tab-bar" aria-label="Main navigation">
  {#each links as link}
    {@const active = isActive(link.href, page.url.pathname)}
    <a
      href={link.href}
      class="tab-item"
      class:active
      aria-current={active ? 'page' : undefined}
      data-sveltekit-preload-data
    >
      <svg class="tab-icon" viewBox="0 0 24 24" fill={active ? 'currentColor' : 'none'} stroke="currentColor" stroke-width={active ? 0 : 1.5} stroke-linecap="round" stroke-linejoin="round">
        {#if link.icon === 'today'}
          {#if active}
            <path d="M3 12l2-2 7-7 7 7 2 2" fill="none" stroke="currentColor" stroke-width="1.8"/>
            <path d="M5 10v10a1 1 0 001 1h3a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1h3a1 1 0 001-1V10"/>
          {:else}
            <path d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-4 0a1 1 0 01-1-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 01-1 1"/>
          {/if}
        {:else if link.icon === 'map'}
          {#if active}
            <path d="M9 20l-5.447-2.724A1 1 0 013 16.382V5.618a1 1 0 011.447-.894L9 7l6-3 5.553 2.776A1 1 0 0121 7.618v10.764a1 1 0 01-1.447.894L15 17l-6 3z"/>
          {:else}
            <path d="M9 20l-5.447-2.724A1 1 0 013 16.382V5.618a1 1 0 011.447-.894L9 7m0 13l6-3m-6 3V7m6 10l5.447 2.724A1 1 0 0021 18.382V7.618a1 1 0 00-.553-.894L15 4m0 13V4m0 0L9 7"/>
          {/if}
        {:else if link.icon === 'pin'}
          {#if active}
            <path d="M17.657 16.657L13.414 20.9a2 2 0 01-2.828 0l-4.243-4.243a8 8 0 1111.314 0z"/>
            <circle cx="12" cy="11" r="3" fill="var(--system-bg-secondary)"/>
          {:else}
            <path d="M17.657 16.657L13.414 20.9a1.998 1.998 0 01-2.827 0l-4.244-4.243a8 8 0 1111.314 0z"/>
            <path d="M15 11a3 3 0 11-6 0 3 3 0 016 0z"/>
          {/if}
        {:else if link.icon === 'cpu'}
          {#if active}
            <rect x="5" y="5" width="14" height="14" rx="2" fill="currentColor"/>
            <rect x="9" y="9" width="6" height="6" fill="var(--system-bg-secondary)"/>
            <path d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2" fill="none" stroke="currentColor" stroke-width="1.5"/>
          {:else}
            <path d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z"/>
          {/if}
        {:else if link.icon === 'shield'}
          {#if active}
            <rect x="4" y="11" width="16" height="10" rx="2" fill="currentColor"/>
            <path d="M8 11V7a4 4 0 018 0v4" fill="none" stroke="currentColor" stroke-width="1.5"/>
            <circle cx="12" cy="16" r="1.5" fill="var(--system-bg-secondary)"/>
          {:else}
            <path d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"/>
          {/if}
        {/if}
      </svg>
      <span class="tab-label">{link.label}</span>
    </a>
  {/each}
</nav>

<!-- Desktop: sidebar -->
<aside class="sidebar">
  <div class="sidebar-brand">
    <img src="/icons/cairn.svg" alt="" class="sidebar-logo" width="30" height="30" />
    <span class="sidebar-title">Cairn</span>
  </div>
  <div class="sidebar-links">
    {#each links as link}
      {@const active = isActive(link.href, page.url.pathname)}
      <a
        href={link.href}
        class="sidebar-link"
        class:active
        data-sveltekit-preload-data
      >
        <svg class="sidebar-icon" viewBox="0 0 24 24" fill={active ? 'currentColor' : 'none'} stroke="currentColor" stroke-width={active ? 0 : 1.5} stroke-linecap="round" stroke-linejoin="round">
          {#if link.icon === 'today'}
            {#if active}
              <path d="M3 12l2-2 7-7 7 7 2 2" fill="none" stroke="currentColor" stroke-width="1.8"/>
              <path d="M5 10v10a1 1 0 001 1h3a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1h3a1 1 0 001-1V10"/>
            {:else}
              <path d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-4 0a1 1 0 01-1-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 01-1 1"/>
            {/if}
          {:else if link.icon === 'map'}
            {#if active}
              <path d="M9 20l-5.447-2.724A1 1 0 013 16.382V5.618a1 1 0 011.447-.894L9 7l6-3 5.553 2.776A1 1 0 0121 7.618v10.764a1 1 0 01-1.447.894L15 17l-6 3z"/>
            {:else}
              <path d="M9 20l-5.447-2.724A1 1 0 013 16.382V5.618a1 1 0 011.447-.894L9 7m0 13l6-3m-6 3V7m6 10l5.447 2.724A1 1 0 0021 18.382V7.618a1 1 0 00-.553-.894L15 4m0 13V4m0 0L9 7"/>
            {/if}
          {:else if link.icon === 'pin'}
            {#if active}
              <path d="M17.657 16.657L13.414 20.9a2 2 0 01-2.828 0l-4.243-4.243a8 8 0 1111.314 0z"/>
              <circle cx="12" cy="11" r="3" fill="var(--system-bg-secondary)"/>
            {:else}
              <path d="M17.657 16.657L13.414 20.9a1.998 1.998 0 01-2.827 0l-4.244-4.243a8 8 0 1111.314 0z"/>
              <path d="M15 11a3 3 0 11-6 0 3 3 0 016 0z"/>
            {/if}
          {:else if link.icon === 'cpu'}
            {#if active}
              <rect x="5" y="5" width="14" height="14" rx="2" fill="currentColor"/>
              <rect x="9" y="9" width="6" height="6" fill="var(--system-bg-secondary)"/>
              <path d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2" fill="none" stroke="currentColor" stroke-width="1.5"/>
            {:else}
              <path d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z"/>
            {/if}
          {:else if link.icon === 'shield'}
            {#if active}
              <rect x="4" y="11" width="16" height="10" rx="2" fill="currentColor"/>
              <path d="M8 11V7a4 4 0 018 0v4" fill="none" stroke="currentColor" stroke-width="1.5"/>
              <circle cx="12" cy="16" r="1.5" fill="var(--system-bg-secondary)"/>
            {:else}
              <path d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"/>
            {/if}
          {/if}
        </svg>
        <span>{link.label}</span>
      </a>
    {/each}
  </div>
</aside>

<style>
  /* ---- iOS Tab Bar (mobile) ---- */
  .tab-bar {
    position: fixed;
    bottom: 0;
    left: 0;
    right: 0;
    height: var(--tab-bar-total);
    background: var(--material-chrome);
    backdrop-filter: blur(25px) saturate(1.8);
    -webkit-backdrop-filter: blur(25px) saturate(1.8);
    border-top: 0.5px solid var(--separator);
    z-index: 100;
    display: flex;
    justify-content: space-around;
    align-items: flex-start;
    padding-top: 5px;
    padding-bottom: env(safe-area-inset-bottom, 0px);
  }

  .tab-item {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 1px;
    padding: 2px 10px;
    color: var(--label-tertiary);
    text-decoration: none;
    -webkit-tap-highlight-color: transparent;
    transition: color var(--duration-fast) var(--ease-default);
    min-width: 50px;
  }
  .tab-item:active { transform: scale(0.92); }
  .tab-item.active { color: var(--tint); }

  .tab-icon { width: 25px; height: 25px; }
  .tab-label {
    font-size: 10px;
    font-weight: 500;
    letter-spacing: 0.01em;
  }

  /* ---- Sidebar (desktop) ---- */
  .sidebar { display: none; }

  @media (min-width: 768px) {
    .tab-bar { display: none; }
    .sidebar {
      display: flex;
      flex-direction: column;
      position: fixed;
      top: 0;
      left: 0;
      bottom: 0;
      width: var(--sidebar-width);
      background: var(--material-chrome);
      backdrop-filter: blur(30px) saturate(1.8);
      -webkit-backdrop-filter: blur(30px) saturate(1.8);
      border-right: 0.5px solid var(--separator);
      z-index: 100;
      padding: 0;
    }
  }

  .sidebar-brand {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 20px 20px 8px;
  }
  .sidebar-logo { border-radius: 7px; }
  .sidebar-title {
    font-size: 18px;
    font-weight: 700;
    letter-spacing: -0.02em;
    color: var(--label-primary);
  }

  .sidebar-links {
    display: flex;
    flex-direction: column;
    padding: 8px 12px;
    gap: 1px;
  }

  .sidebar-link {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 12px;
    border-radius: var(--radius-sm);
    color: var(--label-secondary);
    font-size: 15px;
    font-weight: 400;
    text-decoration: none;
    transition: background var(--duration-fast) var(--ease-default),
                color var(--duration-fast) var(--ease-default);
  }
  .sidebar-link:hover {
    background: var(--fill-quaternary);
    color: var(--label-primary);
  }
  .sidebar-link.active {
    background: var(--tint-dim);
    color: var(--tint);
    font-weight: 500;
  }
  .sidebar-icon { width: 20px; height: 20px; flex-shrink: 0; }
</style>
