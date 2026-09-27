<script lang="ts">
  let { variant = 'text', width = '100%' }: {
    variant?: 'text' | 'title' | 'stat' | 'map' | 'circle';
    width?: string;
  } = $props();
</script>

<div class="skeleton {variant}" style:width></div>

<style>
  .skeleton {
    background: linear-gradient(90deg, var(--system-bg-tertiary) 25%, var(--fill-quaternary) 50%, var(--system-bg-tertiary) 75%);
    background-size: 200% 100%;
    animation: shimmer 1.5s infinite;
    border-radius: var(--radius-sm);
  }
  .text   { height: 14px; margin-bottom: 8px; }
  .title  { height: 20px; margin-bottom: 12px; }
  .stat   { height: 28px; }
  .map    { height: 120px; border-radius: var(--radius-card); }
  .circle { width: 40px !important; height: 40px; border-radius: 50%; }
</style>
