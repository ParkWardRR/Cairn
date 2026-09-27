<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { mapSettings, getMapStyle } from '$lib/stores/mapSettings';
  import type { MapSettings } from '$lib/stores/mapSettings';
  import * as maplibregl from 'maplibre-gl';

  let {
    class: className = '',
    center = [-122.4194, 37.7749] as [number, number],
    zoom = 13,
    interactive = true,
    pitch = 0,
    bearing = 0,
    onready,
  }: {
    class?: string;
    center?: [number, number];
    zoom?: number;
    interactive?: boolean;
    pitch?: number;
    bearing?: number;
    onready?: (map: maplibregl.Map) => void;
  } = $props();

  let container: HTMLDivElement;
  let map: maplibregl.Map | null = null;
  let currentSettings: MapSettings | null = null;
  let unsubscribe: (() => void) | null = null;

  onMount(() => {
    const initialSettings = getSettingsSync();
    const style = getMapStyle(initialSettings);

    map = new maplibregl.Map({
      container,
      style,
      center,
      zoom,
      pitch,
      bearing,
      interactive,
      attributionControl: interactive ? {} : false,
      maxZoom: 20,
      fadeDuration: 200,
    });

    if (!interactive) {
      map.dragPan.disable();
      map.scrollZoom.disable();
      map.touchZoomRotate.disable();
      map.doubleClickZoom.disable();
      map.keyboard.disable();
    }

    map.on('load', () => {
      onready?.(map!);
    });

    currentSettings = { ...initialSettings };

    unsubscribe = mapSettings.subscribe((settings) => {
      if (!map) return;
      if (!currentSettings ||
          settings.providerId !== currentSettings.providerId ||
          settings.customTileUrl !== currentSettings.customTileUrl ||
          settings.pmtilesUrl !== currentSettings.pmtilesUrl) {
        currentSettings = { ...settings };
        map.setStyle(getMapStyle(settings));
      }
    });
  });

  function getSettingsSync(): MapSettings {
    let settings: MapSettings | null = null;
    const unsub = mapSettings.subscribe((s) => { settings = s; });
    unsub();
    return settings!;
  }

  onDestroy(() => {
    unsubscribe?.();
    map?.remove();
    map = null;
  });
</script>

<div bind:this={container} class="maplibre-container {className}"></div>

<style>
  .maplibre-container {
    width: 100%;
    height: 100%;
    border-radius: var(--radius-md);
    overflow: hidden;
    background: var(--system-bg-secondary);
  }
</style>
