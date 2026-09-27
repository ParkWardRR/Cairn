<script lang="ts">
  import { toasts } from '$lib/stores/toast';
  import { fly, fade } from 'svelte/transition';
</script>

<div class="toast-container">
  {#each $toasts as toast (toast.id)}
    <div
      class="toast"
      in:fly={{ y: -30, duration: 350, easing: t => 1 - Math.pow(1 - t, 3) }}
      out:fade={{ duration: 200 }}
    >
      {toast.text}
    </div>
  {/each}
</div>

<style>
  .toast-container {
    position: fixed;
    top: 12px;
    left: 50%;
    transform: translateX(-50%);
    z-index: 400;
    display: flex;
    flex-direction: column;
    gap: 8px;
    align-items: center;
    pointer-events: none;
    width: 100%;
    max-width: 350px;
    padding: 0 16px;
  }
  .toast {
    background: var(--material-thick);
    backdrop-filter: blur(25px) saturate(1.8);
    -webkit-backdrop-filter: blur(25px) saturate(1.8);
    border-radius: var(--radius-card);
    padding: 14px 20px;
    font-size: 15px;
    font-weight: 500;
    color: var(--label-primary);
    box-shadow: var(--shadow-float);
    pointer-events: auto;
    text-align: center;
    width: 100%;
    letter-spacing: -0.01em;
  }
</style>
