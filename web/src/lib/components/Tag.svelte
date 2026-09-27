<script lang="ts">
  let { label, removable = false, onremove }: {
    label: string;
    removable?: boolean;
    onremove?: () => void;
  } = $props();
</script>

<span class="tag">
  {label}
  {#if removable}
    <button class="tag-x" onclick={onremove} aria-label="Remove tag {label}">&times;</button>
  {/if}
</span>

<style>
  .tag {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 3px 10px;
    border-radius: var(--radius-full);
    font-size: 13px;
    font-weight: 500;
    background: var(--tint-dim);
    color: var(--tint);
  }
  .tag-x {
    cursor: pointer;
    opacity: 0.5;
    font-size: 15px;
    line-height: 1;
    padding: 0;
    color: var(--tint);
    background: none;
    border: none;
  }
  .tag-x:hover { opacity: 1; }
</style>
