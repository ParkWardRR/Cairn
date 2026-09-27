<script lang="ts">
  import type { Snippet } from 'svelte';

  let { title, description, icon }: {
    title: string;
    description: string;
    icon?: Snippet;
  } = $props();
</script>

<div class="empty">
  {#if icon}
    <div class="empty-icon">{@render icon()}</div>
  {/if}
  <div class="empty-title">{title}</div>
  <div class="empty-desc">{description}</div>
</div>

<style>
  .empty {
    text-align: center;
    padding: 48px 24px;
    color: var(--label-tertiary);
  }
  .empty-icon {
    margin-bottom: 16px;
    opacity: 0.25;
  }
  .empty-title {
    font-size: 20px;
    font-weight: 600;
    color: var(--label-secondary);
    margin-bottom: 6px;
    letter-spacing: -0.02em;
  }
  .empty-desc {
    font-size: 15px;
    line-height: 1.45;
    max-width: 280px;
    margin: 0 auto;
  }
</style>
