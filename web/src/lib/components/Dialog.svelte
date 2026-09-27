<script lang="ts">
  import { fade, scale } from 'svelte/transition';

  let { title, text, confirmLabel = 'Delete', open = $bindable(false), onconfirm }: {
    title: string;
    text: string;
    confirmLabel?: string;
    open: boolean;
    onconfirm: () => void;
  } = $props();

  function cancel() { open = false; }
  function confirm() { onconfirm(); open = false; }
</script>

{#if open}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="overlay"
    transition:fade={{ duration: 200 }}
    onclick={(e) => { if (e.target === e.currentTarget) cancel(); }}
    onkeydown={(e) => { if (e.key === 'Escape') cancel(); }}
  >
    <div class="alert" transition:scale={{ start: 1.05, duration: 250, opacity: 0 }}>
      <div class="alert-content">
        <div class="alert-title">{title}</div>
        <div class="alert-message">{text}</div>
      </div>
      <div class="alert-actions">
        <button class="alert-btn" onclick={cancel}>Cancel</button>
        <button class="alert-btn destructive" onclick={confirm}>{confirmLabel}</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.4);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 500;
  }
  .alert {
    background: var(--system-bg-elevated-secondary);
    border-radius: 14px;
    width: 270px;
    overflow: hidden;
    box-shadow: var(--shadow-float);
  }
  .alert-content {
    padding: 20px 16px 16px;
    text-align: center;
  }
  .alert-title {
    font-size: 17px;
    font-weight: 600;
    letter-spacing: -0.02em;
    margin-bottom: 4px;
  }
  .alert-message {
    font-size: 13px;
    color: var(--label-secondary);
    line-height: 1.45;
  }
  .alert-actions {
    display: flex;
    border-top: 0.5px solid var(--separator);
  }
  .alert-btn {
    flex: 1;
    padding: 12px;
    font-size: 17px;
    font-weight: 400;
    color: var(--system-blue);
    background: none;
    border: none;
    cursor: pointer;
    transition: background var(--duration-fast);
  }
  .alert-btn:first-child {
    border-right: 0.5px solid var(--separator);
    font-weight: 600;
  }
  .alert-btn:active { background: var(--fill-tertiary); }
  .alert-btn.destructive {
    color: var(--system-red);
    font-weight: 400;
  }
</style>
