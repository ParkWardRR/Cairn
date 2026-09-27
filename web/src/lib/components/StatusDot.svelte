<script lang="ts">
  let { color, pulse = false }: {
    color: 'green' | 'amber' | 'red';
    pulse?: boolean;
  } = $props();
</script>

<span class="dot {color}" class:pulse>
  {#if pulse}<span class="ring"></span>{/if}
</span>

<style>
  .dot {
    position: relative;
    display: inline-block;
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex-shrink: 0;
  }
  .green  { background: var(--system-green); }
  .amber  { background: var(--system-orange); }
  .red    { background: var(--system-red); }

  .ring {
    position: absolute;
    inset: -3px;
    border-radius: 50%;
    animation: pulse-ring 2s ease-out infinite;
  }
  .green .ring  { border: 2px solid var(--system-green); }
  .amber .ring  { border: 2px solid var(--system-orange); }
  .red .ring    { border: 2px solid var(--system-red); }
</style>
