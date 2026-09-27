<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import { toasts } from '$lib/stores/toast';
  import Card from '$lib/components/Card.svelte';
  import Dialog from '$lib/components/Dialog.svelte';
  import EmptyState from '$lib/components/EmptyState.svelte';
  import MapLibreMap from '$lib/components/MapLibreMap.svelte';
  import * as maplibregl from 'maplibre-gl';
  import type { Map as MLMap } from 'maplibre-gl';

  let places: any[] = $state([]);
  let loading = $state(true);
  let map: MLMap | null = $state(null);

  let placeName = $state('');
  let placeLat = $state('');
  let placeLon = $state('');
  let placeRadius = $state('100');
  let editingId = $state<string | null>(null);

  let deleteOpen = $state(false);
  let deleteTarget = $state<{ id: string; name: string } | null>(null);

  let clickMarker: maplibregl.Marker | null = null;

  onMount(() => {});

  function setupMap(m: MLMap) {
    map = m;
    map.setCenter([-122.4194, 37.7749]);
    map.setZoom(10);

    map.on('click', (e) => {
      placeLat = e.lngLat.lat.toFixed(5);
      placeLon = e.lngLat.lng.toFixed(5);
      const radius = parseInt(placeRadius) || 100;

      if (clickMarker) clickMarker.remove();
      clickMarker = new maplibregl.Marker({ color: '#ff375f' })
        .setLngLat(e.lngLat)
        .addTo(map!);

      if (map!.getSource('click-radius')) {
        (map!.getSource('click-radius') as any).setData(
          radiusCircleGeoJSON(e.lngLat.lng, e.lngLat.lat, radius)
        );
      } else {
        map!.addSource('click-radius', {
          type: 'geojson',
          data: radiusCircleGeoJSON(e.lngLat.lng, e.lngLat.lat, radius),
        });
        map!.addLayer({
          id: 'click-radius-fill',
          type: 'fill',
          source: 'click-radius',
          paint: { 'fill-color': '#ff375f', 'fill-opacity': 0.1 },
        });
        map!.addLayer({
          id: 'click-radius-stroke',
          type: 'line',
          source: 'click-radius',
          paint: { 'line-color': '#ff375f', 'line-width': 1 },
        });
      }
    });

    loadPlaces();
  }

  async function loadPlaces() {
    try {
      const data = await api.places();
      places = data.places || [];
      renderPlacesOnMap();
    } catch {
      places = [];
    }
    loading = false;
  }

  function renderPlacesOnMap() {
    if (!map) return;

    if (map.getLayer('places-radius')) map.removeLayer('places-radius');
    if (map.getLayer('places-center')) map.removeLayer('places-center');
    if (map.getSource('places-data')) map.removeSource('places-data');
    if (map.getSource('places-radius-data')) map.removeSource('places-radius-data');

    const valid = places.filter(p => p.lat && p.lon);
    if (valid.length === 0) return;

    const radiusFeatures = valid.map(p => radiusCircleGeoJSON(p.lon, p.lat, p.radius_m || 100).features[0]);
    map.addSource('places-radius-data', {
      type: 'geojson',
      data: { type: 'FeatureCollection', features: radiusFeatures },
    });
    map.addLayer({
      id: 'places-radius',
      type: 'fill',
      source: 'places-radius-data',
      paint: { 'fill-color': '#5e5ce6', 'fill-opacity': 0.12 },
    });

    map.addSource('places-data', {
      type: 'geojson',
      data: {
        type: 'FeatureCollection',
        features: valid.map(p => ({
          type: 'Feature' as const,
          geometry: { type: 'Point' as const, coordinates: [p.lon, p.lat] },
          properties: { name: p.name, visits: p.visit_count || 0 },
        })),
      },
    });
    map.addLayer({
      id: 'places-center',
      type: 'circle',
      source: 'places-data',
      paint: {
        'circle-radius': 6,
        'circle-color': '#30d158',
        'circle-stroke-width': 2,
        'circle-stroke-color': '#ffffff',
      },
    });

    const lngs = valid.map(p => p.lon);
    const lats = valid.map(p => p.lat);
    map.fitBounds(
      [[Math.min(...lngs), Math.min(...lats)], [Math.max(...lngs), Math.max(...lats)]],
      { padding: 50, maxZoom: 14, duration: 0 },
    );

    map.on('click', 'places-center', (e) => {
      const props = e.features?.[0]?.properties;
      if (props) {
        new maplibregl.Popup({ offset: 10 })
          .setLngLat(e.lngLat)
          .setHTML(`<b>${props.name}</b><br>${props.visits} visits`)
          .addTo(map!);
      }
    });
    map.on('mouseenter', 'places-center', () => { map!.getCanvas().style.cursor = 'pointer'; });
    map.on('mouseleave', 'places-center', () => { map!.getCanvas().style.cursor = ''; });
  }

  function radiusCircleGeoJSON(lng: number, lat: number, radiusM: number) {
    const steps = 64;
    const coords: [number, number][] = [];
    for (let i = 0; i <= steps; i++) {
      const angle = (i / steps) * 2 * Math.PI;
      const dx = radiusM * Math.cos(angle);
      const dy = radiusM * Math.sin(angle);
      coords.push([
        lng + dx / (111320 * Math.cos(lat * Math.PI / 180)),
        lat + dy / 110540,
      ]);
    }
    return {
      type: 'FeatureCollection' as const,
      features: [{
        type: 'Feature' as const,
        geometry: { type: 'Polygon' as const, coordinates: [coords] },
        properties: {},
      }],
    };
  }

  async function savePlace() {
    if (!placeName.trim()) { toasts.show('Enter a name'); return; }
    const lat = parseFloat(placeLat);
    const lon = parseFloat(placeLon);
    if (isNaN(lat) || isNaN(lon)) { toasts.show('Tap the map to set location'); return; }
    const radius_m = parseInt(placeRadius) || 100;

    try {
      if (editingId) {
        await api.updatePlace(editingId, { name: placeName.trim(), lat, lon, radius_m });
        toasts.show(`Updated "${placeName.trim()}"`);
      } else {
        await api.createPlace({ name: placeName.trim(), lat, lon, radius_m });
        toasts.show(`Saved "${placeName.trim()}"`);
      }
      resetForm();
      loading = true;
      await loadPlaces();
    } catch {
      toasts.show('Failed to save place');
    }
  }

  function startEdit(p: any) {
    editingId = p.id;
    placeName = p.name;
    placeLat = String(p.lat);
    placeLon = String(p.lon);
    placeRadius = String(p.radius_m || 100);
  }

  function resetForm() {
    editingId = null;
    placeName = '';
    placeLat = '';
    placeLon = '';
    placeRadius = '100';
  }

  function confirmDelete(p: any) {
    deleteTarget = { id: p.id, name: p.name };
    deleteOpen = true;
  }

  async function doDelete() {
    if (!deleteTarget) return;
    try {
      await api.deletePlace(deleteTarget.id);
      toasts.show(`Deleted "${deleteTarget.name}"`);
      loading = true;
      await loadPlaces();
    } catch {
      toasts.show('Failed to delete');
    }
  }
</script>

<div class="page">
  <header class="large-title-header">
    <h1 class="large-title">Places</h1>
    <p class="subtitle">Saved locations and geofences</p>
  </header>

  <div class="places-layout">
    <div class="places-main">
      <section class="grouped-section">
        <div class="map-wrap">
          <MapLibreMap onready={setupMap} />
        </div>
      </section>

      <section class="grouped-section">
        <h2 class="section-header">{editingId ? 'Edit Place' : 'Add Place'}</h2>
        <Card>
          <p class="form-hint">Tap the map to set a location, then fill in the details.</p>
          <div class="form-grid">
            <div class="form-field">
              <label class="form-label" for="place-name">Name</label>
              <input id="place-name" type="text" bind:value={placeName} placeholder="Home, Work..." />
            </div>
            <div class="form-field">
              <label class="form-label" for="place-radius">Radius (m)</label>
              <input id="place-radius" type="number" bind:value={placeRadius} min="10" max="5000" />
            </div>
            <div class="form-field">
              <label class="form-label" for="place-lat">Latitude</label>
              <input id="place-lat" type="number" bind:value={placeLat} step="0.00001" placeholder="Tap map" />
            </div>
            <div class="form-field">
              <label class="form-label" for="place-lon">Longitude</label>
              <input id="place-lon" type="number" bind:value={placeLon} step="0.00001" placeholder="Tap map" />
            </div>
          </div>
          <div class="form-actions">
            <button class="btn-tint" onclick={savePlace}>{editingId ? 'Update' : 'Save Place'}</button>
            {#if editingId}
              <button class="btn-plain" onclick={resetForm}>Cancel</button>
            {/if}
          </div>
        </Card>
      </section>
    </div>

    <div class="places-sidebar">
      <section class="grouped-section">
        <h2 class="section-header">Saved Places</h2>
        {#if loading}
          <div class="skeleton-card"></div>
          <div class="skeleton-card short"></div>
        {:else if places.length === 0}
          <EmptyState title="No places saved" description="Tap the map and add your first place." />
        {:else}
          <div class="places-list">
            {#each places as p (p.id)}
              <Card>
                <div class="place-row">
                  <div class="place-info">
                    <div class="place-name">{p.name}</div>
                    <div class="place-detail">{p.lat?.toFixed(4)}, {p.lon?.toFixed(4)} &middot; {p.radius_m || 100}m &middot; {p.visit_count || 0} visits</div>
                  </div>
                  <div class="place-actions">
                    <button class="icon-btn" onclick={() => startEdit(p)} aria-label="Edit">
                      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>
                    </button>
                    <button class="icon-btn destructive" onclick={() => confirmDelete(p)} aria-label="Delete">
                      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M3 6h18M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a2 2 0 012-2h4a2 2 0 012 2v2"/></svg>
                    </button>
                  </div>
                </div>
              </Card>
            {/each}
          </div>
        {/if}
      </section>
    </div>
  </div>
</div>

<Dialog title="Delete Place" text="Delete &quot;{deleteTarget?.name}&quot;? This cannot be undone." bind:open={deleteOpen} onconfirm={doDelete} />

<style>
  .page { padding-top: 16px; animation: slide-up var(--duration-slow) var(--ease-decelerate); }
  .large-title-header { padding: 8px 4px 4px; margin-bottom: 12px; }
  .large-title { font-size: 34px; font-weight: 700; letter-spacing: -0.03em; line-height: 1.1; }
  .subtitle { font-size: 15px; color: var(--label-secondary); margin-top: 2px; }
  .grouped-section { margin-bottom: 20px; }
  .section-header { font-size: 20px; font-weight: 700; letter-spacing: -0.02em; padding: 0 4px; margin-bottom: 8px; }
  .places-layout { display: grid; grid-template-columns: 1fr; gap: 0; }
  @media (min-width: 768px) { .places-layout { grid-template-columns: 1fr 1fr; gap: 20px; } }
  .map-wrap { height: 300px; border-radius: var(--radius-card); overflow: hidden; }
  @media (min-width: 768px) { .map-wrap { height: 350px; } }
  .form-hint { font-size: 13px; color: var(--label-tertiary); margin-bottom: 12px; }
  .form-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; margin-bottom: 12px; }
  .form-field { display: flex; flex-direction: column; gap: 4px; }
  .form-label { font-size: 12px; font-weight: 600; color: var(--label-tertiary); text-transform: uppercase; letter-spacing: 0.04em; }
  .form-actions { display: flex; gap: 8px; }
  .btn-tint { padding: 10px 24px; border-radius: var(--radius-sm); background: var(--tint); color: white; font-size: 15px; font-weight: 600; border: none; cursor: pointer; }
  .btn-tint:active { opacity: 0.7; }
  .btn-plain { padding: 10px 16px; background: none; color: var(--system-blue); font-size: 15px; border: none; cursor: pointer; }
  .places-list { display: flex; flex-direction: column; gap: 6px; }
  .place-row { display: flex; justify-content: space-between; align-items: center; }
  .place-info { flex: 1; min-width: 0; }
  .place-name { font-size: 17px; font-weight: 500; }
  .place-detail { font-size: 13px; color: var(--label-tertiary); margin-top: 2px; }
  .place-actions { display: flex; gap: 4px; }
  .icon-btn { width: 36px; height: 36px; display: flex; align-items: center; justify-content: center; border-radius: var(--radius-full); color: var(--label-secondary); transition: background var(--duration-fast); }
  .icon-btn:active { background: var(--fill-tertiary); }
  .icon-btn.destructive { color: var(--system-red); }
  .skeleton-card { height: 72px; background: linear-gradient(90deg, var(--system-bg-secondary) 25%, var(--fill-quaternary) 50%, var(--system-bg-secondary) 75%); background-size: 200% 100%; animation: shimmer 1.5s infinite; border-radius: var(--radius-card); margin-bottom: 6px; }
  .skeleton-card.short { width: 80%; }
</style>
