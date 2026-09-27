<script lang="ts">
  import { onMount, onDestroy } from 'svelte';

  let {
    class: className = '',
    center = [37.7749, -122.4194] as [number, number],
    zoom = 13,
    interactive = true,
    onready
  }: {
    class?: string;
    center?: [number, number];
    zoom?: number;
    interactive?: boolean;
    onready?: (map: any) => void;
  } = $props();

  let container: HTMLDivElement;
  let map: any;

  const DARK_TILES = 'https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png';
  const TILE_ATTR = '&copy; <a href="https://www.openstreetmap.org/copyright">OSM</a> &copy; <a href="https://carto.com/">CARTO</a>';

  onMount(() => {
    const L = (window as any).L;
    if (!L) return;

    map = L.map(container, {
      zoomControl: interactive,
      attributionControl: interactive,
      dragging: interactive,
      scrollWheelZoom: interactive,
      touchZoom: interactive,
      doubleClickZoom: interactive,
    });

    L.tileLayer(DARK_TILES, { attribution: TILE_ATTR, maxZoom: 19 }).addTo(map);
    map.setView(center, zoom);

    onready?.(map);
  });

  onDestroy(() => {
    map?.remove();
  });
</script>

<div bind:this={container} class="map-container {className}"></div>

<style>
  .map-container {
    width: 100%;
    height: 100%;
    border-radius: var(--radius-md);
    overflow: hidden;
    border: 1px solid var(--border-default);
    background: var(--bg-base);
  }
</style>
