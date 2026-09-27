<script lang="ts">
  let { value, label, loading = false }: {
    value: string | number;
    label: string;
    loading?: boolean;
  } = $props();
</script>

<div class="stat">
  {#if loading}
    <div class="skeleton-value"></div>
  {:else}
    <div class="stat-value">{value}</div>
  {/if}
  <div class="stat-label">{label}</div>
</div>

<style>
  .stat {
    background: var(--system-bg-grouped-secondary);
    border-radius: var(--radius-card);
    padding: 16px 20px;
    text-align: center;
  }
  .stat-value {
    font-size: 28px;
    font-weight: 700;
    color: var(--label-primary);
    line-height: 1.15;
    letter-spacing: -0.03em;
    font-variant-numeric: tabular-nums;
  }
  .stat-label {
    font-size: 11px;
    color: var(--label-tertiary);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    font-weight: 600;
    margin-top: 4px;
  }
  .skeleton-value {
    height: 28px;
    width: 50%;
    margin: 0 auto 4px;
    background: linear-gradient(90deg, var(--system-bg-tertiary) 25%, var(--fill-quaternary) 50%, var(--system-bg-tertiary) 75%);
    background-size: 200% 100%;
    animation: shimmer 1.5s infinite;
    border-radius: var(--radius-sm);
  }
</style>
