<script lang="ts">
  import type { Snippet } from 'svelte';

  let { clickable = false, href, children, class: className = '' }: {
    clickable?: boolean;
    href?: string;
    children: Snippet;
    class?: string;
  } = $props();
</script>

{#if href}
  <a {href} class="card clickable {className}" data-sveltekit-preload-data>
    {@render children()}
  </a>
{:else}
  <div class="card {className}" class:clickable>
    {@render children()}
  </div>
{/if}

<style>
  .card {
    background: var(--system-bg-grouped-secondary);
    border-radius: var(--radius-card);
    padding: 16px;
    transition: background var(--duration-fast) var(--ease-default),
                transform var(--duration-fast) var(--ease-default);
  }
  .clickable {
    cursor: pointer;
    text-decoration: none;
    color: inherit;
    display: block;
  }
  .clickable:hover {
    background: var(--system-bg-grouped-tertiary);
  }
  .clickable:active {
    transform: scale(0.98);
  }
</style>
