<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { api } from '$lib/api';
  import { toasts } from '$lib/stores/toast';
  import { formatDistance, formatDuration, formatDate, formatTime, formatDateTime, formatEventType, speedColor } from '$lib/utils/format';
  import Card from '$lib/components/Card.svelte';
  import Tag from '$lib/components/Tag.svelte';
  import Dialog from '$lib/components/Dialog.svelte';
  import MapLibreMap from '$lib/components/MapLibreMap.svelte';
  import type { Map as MLMap } from 'maplibre-gl';

  const id = page.params.id!;

  let loading = $state(true);
  let trip: any = $state(null);
  let events: any[] = $state([]);
  let routeData: any = $state(null);
  let deleteOpen = $state(false);
  let tags = $state<string[]>([]);
  let newTag = $state('');

  onMount(async () => {
    const [tripRes, routeRes, eventsRes] = await Promise.allSettled([
      api.trip(id),
      api.tripRoute(id),
      api.tripEvents(id),
    ]);
    if (tripRes.status === 'fulfilled') {
      trip = tripRes.value;
      tags = trip?.tags || [];
    }
    if (routeRes.status === 'fulfilled') routeData = routeRes.value;
    if (eventsRes.status === 'fulfilled') events = eventsRes.value?.events || [];
    loading = false;
  });

  function setupMap(map: MLMap) {
    if (routeData?.features?.length > 0) {
      const feature = routeData.features[0];
      const coords: number[][] = feature.geometry?.coordinates || [];
      const speeds: number[] = feature.properties?.speed || [];

      if (coords.length > 1) {
        const maxSpeed = Math.max(...speeds.filter((s: number) => s > 0), 60);

        if (speeds.length > 0) {
          const segmentFeatures = [];
          for (let i = 1; i < coords.length; i++) {
            const ratio = Math.min((speeds[i] || 0) / maxSpeed, 1);
            segmentFeatures.push({
              type: 'Feature' as const,
              geometry: {
                type: 'LineString' as const,
                coordinates: [coords[i-1].slice(0, 2), coords[i].slice(0, 2)],
              },
              properties: { speed: ratio },
            });
          }

          map.addSource('route-segments', {
            type: 'geojson',
            data: { type: 'FeatureCollection', features: segmentFeatures },
          });

          map.addLayer({
            id: 'route-speed',
            type: 'line',
            source: 'route-segments',
            paint: {
              'line-width': 4,
              'line-color': [
                'interpolate', ['linear'], ['get', 'speed'],
                0, '#30d158',
                0.5, '#ffd60a',
                1, '#ff375f',
              ],
              'line-opacity': 0.85,
            },
            layout: { 'line-cap': 'round', 'line-join': 'round' },
          });
        } else {
          map.addSource('route', {
            type: 'geojson',
            data: {
              type: 'Feature',
              geometry: { type: 'LineString', coordinates: coords.map(c => c.slice(0, 2)) },
              properties: {},
            },
          });
          map.addLayer({
            id: 'route-line',
            type: 'line',
            source: 'route',
            paint: { 'line-color': '#ff375f', 'line-width': 4, 'line-opacity': 0.85 },
            layout: { 'line-cap': 'round', 'line-join': 'round' },
          });
        }

        const start = coords[0];
        const end = coords[coords.length - 1];
        map.addSource('endpoints', {
          type: 'geojson',
          data: {
            type: 'FeatureCollection',
            features: [
              { type: 'Feature', geometry: { type: 'Point', coordinates: [start[0], start[1]] }, properties: { type: 'start' } },
              { type: 'Feature', geometry: { type: 'Point', coordinates: [end[0], end[1]] }, properties: { type: 'end' } },
            ],
          },
        });
        map.addLayer({
          id: 'endpoint-circles',
          type: 'circle',
          source: 'endpoints',
          paint: {
            'circle-radius': 7,
            'circle-color': ['match', ['get', 'type'], 'start', '#30d158', '#ff375f'],
            'circle-stroke-width': 2,
            'circle-stroke-color': '#ffffff',
          },
        });

        const lngs = coords.map(c => c[0]);
        const lats = coords.map(c => c[1]);
        map.fitBounds(
          [[Math.min(...lngs), Math.min(...lats)], [Math.max(...lngs), Math.max(...lats)]],
          { padding: 40, duration: 0 },
        );
      }
    } else if (trip) {
      const points: [number, number][] = [];
      if (trip.start_lat && trip.start_lon) points.push([trip.start_lon, trip.start_lat]);
      if (trip.end_lat && trip.end_lon) points.push([trip.end_lon, trip.end_lat]);

      if (points.length > 0) {
        map.addSource('endpoints', {
          type: 'geojson',
          data: {
            type: 'FeatureCollection',
            features: points.map((p, i) => ({
              type: 'Feature' as const,
              geometry: { type: 'Point' as const, coordinates: p },
              properties: { type: i === 0 ? 'start' : 'end' },
            })),
          },
        });
        map.addLayer({
          id: 'endpoint-circles',
          type: 'circle',
          source: 'endpoints',
          paint: {
            'circle-radius': 7,
            'circle-color': ['match', ['get', 'type'], 'start', '#30d158', '#ff375f'],
            'circle-stroke-width': 2,
            'circle-stroke-color': '#ffffff',
          },
        });

        if (points.length === 2) {
          const lngs = points.map(p => p[0]);
          const lats = points.map(p => p[1]);
          map.fitBounds(
            [[Math.min(...lngs), Math.min(...lats)], [Math.max(...lngs), Math.max(...lats)]],
            { padding: 50, duration: 0 },
          );
        } else {
          map.setCenter(points[0]);
          map.setZoom(14);
        }
      }
    }
  }

  async function deleteTrip() {
    try {
      await api.deleteTrip(id);
      toasts.show('Trip deleted');
      goto('/trips');
    } catch {
      toasts.show('Failed to delete trip');
    }
  }

  async function addTag() {
    const t = newTag.trim();
    if (!t) return;
    try {
      await api.addTag(id, t);
      tags = [...tags, t];
      newTag = '';
      toasts.show(`Added "${t}"`);
    } catch {
      toasts.show('Failed to add tag');
    }
  }

  async function removeTag(tag: string) {
    try {
      await api.removeTag(id, tag);
      tags = tags.filter(t => t !== tag);
      toasts.show(`Removed "${tag}"`);
    } catch {
      toasts.show('Failed to remove tag');
    }
  }
</script>

<div class="page">
  <header class="detail-header">
    <a href="/trips" class="back-btn">
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
        <path d="M15 19l-7-7 7-7"/>
      </svg>
      <span>Trips</span>
    </a>
    <h1 class="detail-title">{loading ? 'Trip Details' : formatDate(trip?.started_at)}</h1>
    {#if trip}
      <p class="detail-subtitle">{formatTime(trip.started_at)} - {formatTime(trip.ended_at)}</p>
    {/if}
  </header>

  <section class="map-section">
    <div class="map-wrap">
      {#if !loading}
        <MapLibreMap onready={setupMap} />
      {/if}
    </div>
    <div class="speed-legend">
      <span>Slow</span>
      <div class="speed-bar"></div>
      <span>Fast</span>
    </div>
  </section>

  <div class="detail-layout">
    <div class="detail-main">
      <section class="grouped-section">
        <h2 class="section-header">Events</h2>
        <Card>
          {#if events.length > 0}
            <div class="timeline">
              {#each events as e}
                <div class="timeline-item">
                  <div class="timeline-dot"></div>
                  <div class="timeline-content">
                    <span class="timeline-time">{formatTime(e.timestamp_at)}</span>
                    <span class="timeline-label">{formatEventType(e.event_type)}</span>
                  </div>
                </div>
              {/each}
            </div>
          {:else}
            <div class="empty-inline">
              <span class="empty-text">No events recorded</span>
            </div>
          {/if}
        </Card>
      </section>
    </div>

    <div class="detail-sidebar">
      <section class="grouped-section">
        <h2 class="section-header">Info</h2>
        <Card>
          {#if trip}
            <div class="info-table">
              <div class="info-row">
                <span class="info-label">Start</span>
                <span class="info-value">{formatDateTime(trip.started_at)}</span>
              </div>
              <div class="info-row">
                <span class="info-label">End</span>
                <span class="info-value">{formatDateTime(trip.ended_at)}</span>
              </div>
              <div class="info-row">
                <span class="info-label">Distance</span>
                <span class="info-value">{formatDistance(trip.distance_m)}</span>
              </div>
              <div class="info-row">
                <span class="info-label">Duration</span>
                <span class="info-value">{formatDuration(trip.duration_s)}</span>
              </div>
              <div class="info-row last">
                <span class="info-label">Device</span>
                <span class="info-value mono">{trip.device_id}</span>
              </div>
            </div>
          {/if}
        </Card>
      </section>

      <section class="grouped-section">
        <h2 class="section-header">Tags</h2>
        <Card>
          <div class="tags-area">
            {#if tags.length > 0}
              <div class="tags-list">
                {#each tags as tag}
                  <Tag label={tag} removable onremove={() => removeTag(tag)} />
                {/each}
              </div>
            {:else}
              <span class="empty-text">No tags</span>
            {/if}
            <div class="tag-input-row">
              <input
                type="text"
                bind:value={newTag}
                placeholder="Add tag..."
                class="tag-input"
                onkeydown={(e) => { if (e.key === 'Enter') addTag(); }}
              />
              <button class="btn-tint-sm" onclick={addTag}>Add</button>
            </div>
          </div>
        </Card>
      </section>

      <section class="grouped-section">
        <h2 class="section-header">Export</h2>
        <Card>
          <div class="export-row">
            <a href="/api/v1/trips/{id}/export/gpx" class="export-btn" download>GPX</a>
            <a href="/api/v1/trips/{id}/export/geojson" class="export-btn" download>GeoJSON</a>
            <a href="/api/v1/trips/{id}/export/csv" class="export-btn" download>CSV</a>
          </div>
        </Card>
      </section>

      <section class="grouped-section">
        <button class="delete-btn" onclick={() => deleteOpen = true}>Delete Trip</button>
      </section>
    </div>
  </div>
</div>

<Dialog title="Delete Trip" text="Are you sure? This action cannot be undone." bind:open={deleteOpen} onconfirm={deleteTrip} />

<style>
  .page { padding-top: 8px; animation: slide-up var(--duration-slow) var(--ease-decelerate); }
  .detail-header { padding: 8px 4px 12px; }
  .back-btn {
    display: inline-flex;
    align-items: center;
    gap: 2px;
    color: var(--tint);
    font-size: 17px;
    font-weight: 400;
    text-decoration: none;
    margin-bottom: 8px;
  }
  .back-btn:active { opacity: 0.5; }
  .detail-title { font-size: 28px; font-weight: 700; letter-spacing: -0.03em; line-height: 1.15; }
  .detail-subtitle { font-size: 15px; color: var(--label-secondary); margin-top: 2px; }
  .map-section { margin-bottom: 20px; }
  .map-wrap {
    width: 100%;
    height: 300px;
    border-radius: var(--radius-card);
    overflow: hidden;
    background: var(--system-bg-secondary);
  }
  @media (min-width: 768px) { .map-wrap { height: 400px; } }
  .speed-legend {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 11px;
    color: var(--label-tertiary);
    margin-top: 6px;
    padding: 0 4px;
  }
  .speed-bar {
    width: 80px;
    height: 4px;
    border-radius: 2px;
    background: linear-gradient(to right, #30d158, #ffd60a, #ff375f);
  }
  .detail-layout { display: grid; grid-template-columns: 1fr; gap: 0; }
  @media (min-width: 768px) { .detail-layout { grid-template-columns: 1fr 300px; gap: 20px; } }
  .grouped-section { margin-bottom: 20px; }
  .section-header { font-size: 20px; font-weight: 700; letter-spacing: -0.02em; padding: 0 4px; margin-bottom: 8px; }
  .detail-sidebar { display: flex; flex-direction: column; }
  .info-table { font-size: 15px; }
  .info-row { display: flex; justify-content: space-between; padding: 11px 0; border-bottom: 0.5px solid var(--separator); }
  .info-row.last { border-bottom: none; }
  .info-label { color: var(--label-secondary); }
  .info-value { font-weight: 500; text-align: right; }
  .info-value.mono { font-family: var(--font-mono); font-size: 14px; }
  .timeline { padding-left: 20px; position: relative; }
  .timeline::before {
    content: '';
    position: absolute;
    left: 6px;
    top: 4px;
    bottom: 4px;
    width: 1.5px;
    background: var(--separator-opaque);
  }
  .timeline-item { position: relative; padding-bottom: 16px; display: flex; gap: 12px; }
  .timeline-item:last-child { padding-bottom: 0; }
  .timeline-dot {
    position: absolute;
    left: -20px;
    top: 5px;
    width: 10px;
    height: 10px;
    border-radius: 50%;
    background: var(--system-bg-grouped-secondary);
    border: 2px solid var(--tint);
    z-index: 1;
  }
  .timeline-content { display: flex; flex-direction: column; }
  .timeline-time { font-size: 12px; font-family: var(--font-mono); color: var(--label-tertiary); }
  .timeline-label { font-size: 15px; font-weight: 500; }
  .tags-area { display: flex; flex-direction: column; gap: 10px; }
  .tags-list { display: flex; flex-wrap: wrap; gap: 6px; }
  .tag-input-row { display: flex; gap: 6px; }
  .tag-input { flex: 1; font-size: 15px; padding: 8px 12px; }
  .btn-tint-sm {
    padding: 8px 14px;
    border-radius: var(--radius-sm);
    background: var(--tint);
    color: white;
    font-size: 15px;
    font-weight: 600;
    border: none;
    cursor: pointer;
  }
  .btn-tint-sm:active { opacity: 0.7; }
  .export-row { display: flex; gap: 8px; }
  .export-btn {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 10px;
    border-radius: var(--radius-sm);
    background: var(--system-bg-tertiary);
    color: var(--label-primary);
    font-size: 14px;
    font-weight: 500;
    text-decoration: none;
    transition: background var(--duration-fast);
  }
  .export-btn:active { background: var(--fill-secondary); }
  .delete-btn {
    width: 100%;
    padding: 14px;
    border-radius: var(--radius-card);
    background: var(--system-bg-grouped-secondary);
    color: var(--system-red);
    font-size: 17px;
    font-weight: 400;
    border: none;
    cursor: pointer;
    text-align: center;
    transition: background var(--duration-fast);
  }
  .delete-btn:active { background: var(--system-bg-grouped-tertiary); }
  .empty-inline { padding: 12px 0; }
  .empty-text { font-size: 15px; color: var(--label-tertiary); }
</style>
