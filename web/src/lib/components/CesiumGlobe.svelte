<script lang="ts">
  import { onMount, onDestroy } from 'svelte';

  let {
    class: className = '',
    center = [-122.4194, 37.7749] as [number, number],
    zoom = 13,
    interactive = true,
    onready,
  }: {
    class?: string;
    center?: [number, number];
    zoom?: number;
    interactive?: boolean;
    onready?: (viewer: any) => void;
  } = $props();

  let container: HTMLDivElement;
  let viewer: any = null;

  onMount(async () => {
    const Cesium = await import('cesium');

    (window as any).CESIUM_BASE_URL = 'https://cesium.com/downloads/cesiumjs/releases/1.122/Build/Cesium/';

    viewer = new Cesium.Viewer(container, {
      baseLayerPicker: false,
      geocoder: false,
      homeButton: false,
      sceneModePicker: false,
      navigationHelpButton: false,
      animation: false,
      timeline: false,
      fullscreenButton: false,
      vrButton: false,
      infoBox: false,
      selectionIndicator: false,
      creditContainer: document.createElement('div'),
    } as any);

    const osmProvider = new Cesium.OpenStreetMapImageryProvider({ url: 'https://a.tile.openstreetmap.org/' });
    viewer.imageryLayers.removeAll();
    viewer.imageryLayers.addImageryProvider(osmProvider as any);

    if (!interactive) {
      viewer.scene.screenSpaceCameraController.enableRotate = false;
      viewer.scene.screenSpaceCameraController.enableTranslate = false;
      viewer.scene.screenSpaceCameraController.enableZoom = false;
      viewer.scene.screenSpaceCameraController.enableTilt = false;
      viewer.scene.screenSpaceCameraController.enableLook = false;
    }

    const height = zoomToHeight(zoom);
    viewer.camera.flyTo({
      destination: Cesium.Cartesian3.fromDegrees(center[0], center[1], height),
      duration: 0,
    });

    onready?.(viewer);
  });

  function zoomToHeight(z: number): number {
    return 40_000_000 / Math.pow(2, z);
  }

  onDestroy(() => {
    viewer?.destroy();
    viewer = null;
  });
</script>

<div bind:this={container} class="cesium-container {className}"></div>

<style>
  .cesium-container {
    width: 100%;
    height: 100%;
    border-radius: var(--radius-md);
    overflow: hidden;
    background: var(--system-bg-secondary);
  }
  .cesium-container :global(.cesium-viewer) {
    width: 100%;
    height: 100%;
  }
  .cesium-container :global(.cesium-viewer-bottom) {
    display: none;
  }
</style>
