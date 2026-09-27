<script lang="ts">
  import 'maplibre-gl/dist/maplibre-gl.css';
  import '../app.css';
  import Nav from '$lib/components/Nav.svelte';
  import Toast from '$lib/components/Toast.svelte';
  import type { Snippet } from 'svelte';

  let { children }: { children: Snippet } = $props();
</script>

<Nav />
<main class="content-area">
  <div class="content-inner">
    {@render children()}
  </div>
</main>
<Toast />

<style>
  .content-area {
    min-height: 100dvh;
    padding-bottom: calc(var(--tab-bar-total, 83px) + 16px);
  }

  .content-inner {
    max-width: var(--content-max);
    margin: 0 auto;
    padding: 0 var(--inset-margin);
  }

  @media (min-width: 768px) {
    .content-area {
      margin-left: var(--sidebar-width);
      padding-bottom: 32px;
    }
    .content-inner {
      padding: 0 24px;
    }
  }
</style>
