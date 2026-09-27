<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { mapSettings, getActiveProvider, TILE_PROVIDERS } from '$lib/stores/mapSettings';
  import type { MapSettings } from '$lib/stores/mapSettings';

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
  let currentLayer: any = null;
  let currentSettings: MapSettings | null = null;
  let unsubscribe: (() => void) | null = null;

  async function applyTileLayer(settings: MapSettings) {
    const L = (window as any).L;
    if (!L || !map) return;

    if (currentLayer) {
      map.removeLayer(currentLayer);
      currentLayer = null;
    }

    if (settings.providerId === 'pmtiles' && settings.pmtilesUrl) {
      try {
        const { leafletLayer } = await import('protomaps-leaflet');
        currentLayer = leafletLayer({
          url: settings.pmtilesUrl,
        } as any);
        currentLayer.addTo(map);
      } catch {
        const fallback = TILE_PROVIDERS[0];
        currentLayer = L.tileLayer(fallback.url, {
          attribution: fallback.attribution,
          maxZoom: fallback.maxZoom,
        });
        currentLayer.addTo(map);
      }
    } else if (settings.providerId === 'custom' && settings.customTileUrl) {
      currentLayer = L.tileLayer(settings.customTileUrl, {
        attribution: settings.customAttribution || 'Custom tiles',
        maxZoom: 20,
      });
      currentLayer.addTo(map);
    } else {
      const provider = getActiveProvider(settings) ?? TILE_PROVIDERS[0];
      currentLayer = L.tileLayer(provider.url, {
        attribution: provider.attribution,
        maxZoom: provider.maxZoom,
      });
      currentLayer.addTo(map);
    }
  }

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

    map.setView(center, zoom);

    unsubscribe = mapSettings.subscribe((settings) => {
      if (!currentSettings || settings.providerId !== currentSettings.providerId
          || settings.customTileUrl !== currentSettings.customTileUrl
          || settings.pmtilesUrl !== currentSettings.pmtilesUrl) {
        currentSettings = { ...settings };
        applyTileLayer(settings);
      }
    });

    onready?.(map);
  });

  onDestroy(() => {
    unsubscribe?.();
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
    background: var(--system-bg);
  }
</style>
