<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import { formatDistance, formatDuration, formatDate, formatTime } from '$lib/utils/format';
  import Card from '$lib/components/Card.svelte';
  import Tag from '$lib/components/Tag.svelte';
  import EmptyState from '$lib/components/EmptyState.svelte';
  import MapLibreMap from '$lib/components/MapLibreMap.svelte';
  import type { Map as MLMap } from 'maplibre-gl';

  let loading = $state(true);
  let trips: any[] = $state([]);
  let total = $state(0);
  let offset = $state(0);
  const limit = 20;

  let filterFrom = $state('');
  let filterTo = $state('');
  let filterDevice = $state('');
  let filterTag = $state('');
  let showFilters = $state(false);

  onMount(async () => {
    const params = new URLSearchParams(window.location.search);
    if (params.get('device_id')) {
      filterDevice = params.get('device_id')!;
      showFilters = true;
    }
    await loadTrips();
  });

  async function loadTrips() {
    loading = true;
    try {
      const params: Record<string, string | number> = { limit, offset };
      if (filterFrom) params.from = new Date(filterFrom).toISOString();
      if (filterTo) params.to = new Date(filterTo + 'T23:59:59').toISOString();
      if (filterDevice) params.device_id = filterDevice;
      if (filterTag) params.tag = filterTag;

      const data = await api.trips(params);
      const newTrips = data.trips || [];
      total = data.total || 0;
      trips = offset === 0 ? newTrips : [...trips, ...newTrips];
    } catch {
      trips = [];
    }
    loading = false;
  }

  function applyFilters() {
    offset = 0;
    trips = [];
    loadTrips();
  }

  function clearFilters() {
    filterFrom = '';
    filterTo = '';
    filterDevice = '';
    filterTag = '';
    offset = 0;
    trips = [];
    loadTrips();
  }

  function loadMore() {
    offset += limit;
    loadTrips();
  }

  function setupMiniMap(trip: any) {
    return (map: MLMap) => {
      const points: [number, number][] = [];
      if (trip.start_lat && trip.start_lon) points.push([trip.start_lon, trip.start_lat]);
      if (trip.end_lat && trip.end_lon) points.push([trip.end_lon, trip.end_lat]);
      if (points.length === 0) return;

      const srcId = `ep-${trip.id}`;
      map.addSource(srcId, {
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
        id: `${srcId}-circles`,
        type: 'circle',
        source: srcId,
        paint: {
          'circle-radius': 4,
          'circle-color': ['match', ['get', 'type'], 'start', '#30d158', '#ff375f'],
          'circle-stroke-width': 1,
          'circle-stroke-color': '#ffffff',
        },
      });

      if (points.length === 2) {
        map.addSource(`${srcId}-line`, {
          type: 'geojson',
          data: {
            type: 'Feature',
            geometry: { type: 'LineString', coordinates: points },
            properties: {},
          },
        });
        map.addLayer({
          id: `${srcId}-line-layer`,
          type: 'line',
          source: `${srcId}-line`,
          paint: { 'line-color': '#ff375f', 'line-width': 2, 'line-opacity': 0.4, 'line-dasharray': [4, 4] },
        }, `${srcId}-circles`);

        const lngs = points.map(p => p[0]);
        const lats = points.map(p => p[1]);
        map.fitBounds(
          [[Math.min(...lngs), Math.min(...lats)], [Math.max(...lngs), Math.max(...lats)]],
          { padding: 20, duration: 0 },
        );
      } else {
        map.setCenter(points[0]);
        map.setZoom(14);
      }
    };
  }
</script>

<div class="page">
  <header class="large-title-header">
    <div class="title-row">
      <h1 class="large-title">Trips</h1>
      <button class="filter-toggle" class:active={showFilters} onclick={() => showFilters = !showFilters} aria-label="Toggle filters">
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round">
          <path d="M22 3H2l8 9.46V19l4 2v-8.54L22 3z"/>
        </svg>
      </button>
    </div>
    <p class="subtitle">Your driving history</p>
  </header>

  {#if showFilters}
    <section class="filter-bar">
      <div class="filter-row">
        <input type="date" bind:value={filterFrom} placeholder="From" class="filter-input" />
        <input type="date" bind:value={filterTo} placeholder="To" class="filter-input" />
      </div>
      <div class="filter-row">
        <input type="text" bind:value={filterDevice} placeholder="Device ID" class="filter-input" />
        <input type="text" bind:value={filterTag} placeholder="Tag" class="filter-input" />
      </div>
      <div class="filter-actions">
        <button class="btn-tint" onclick={applyFilters}>Apply</button>
        <button class="btn-plain" onclick={clearFilters}>Clear</button>
      </div>
    </section>
  {/if}

  {#if loading && trips.length === 0}
    <section class="grouped-section">
      {#each Array(3) as _}
        <div class="skeleton-card"></div>
      {/each}
    </section>
  {:else if trips.length === 0}
    <EmptyState
      title="No trips found"
      description="Try adjusting your filters or wait for new trip uploads."
    />
  {:else}
    <section class="trip-list">
      {#each trips as trip (trip.id)}
        <Card clickable href="/trips/{trip.id}" class="trip-card-outer">
          <div class="trip-card">
            <div class="trip-card-info">
              <div class="trip-card-date">{formatDate(trip.started_at)}</div>
              <div class="trip-card-time">{formatTime(trip.started_at)} - {formatTime(trip.ended_at)}</div>
              <div class="trip-card-metrics">
                <span class="trip-card-metric">{formatDistance(trip.distance_m)}</span>
                <span class="metric-sep"></span>
                <span class="trip-card-metric">{formatDuration(trip.duration_s)}</span>
              </div>
              {#if trip.tags?.length > 0}
                <div class="trip-card-tags">
                  {#each trip.tags as tag}
                    <Tag label={tag} />
                  {/each}
                </div>
              {/if}
            </div>
            {#if trip.start_lat && trip.start_lon}
              <div class="trip-card-map">
                <MapLibreMap interactive={false} onready={setupMiniMap(trip)} />
              </div>
            {/if}
          </div>
        </Card>
      {/each}
    </section>

    {#if trips.length < total}
      <div class="load-more">
        <button class="btn-tint" onclick={loadMore}>
          {#if loading}Loading...{:else}Load More{/if}
        </button>
      </div>
    {/if}
  {/if}
</div>

<style>
  .page { padding-top: 16px; animation: slide-up var(--duration-slow) var(--ease-decelerate); }
  .large-title-header { padding: 8px 4px 4px; margin-bottom: 12px; }
  .title-row { display: flex; justify-content: space-between; align-items: center; }
  .large-title { font-size: 34px; font-weight: 700; letter-spacing: -0.03em; line-height: 1.1; }
  .subtitle { font-size: 15px; color: var(--label-secondary); margin-top: 2px; }
  .filter-toggle {
    width: 36px; height: 36px;
    display: flex; align-items: center; justify-content: center;
    border-radius: var(--radius-full);
    color: var(--label-secondary);
    transition: all var(--duration-fast);
  }
  .filter-toggle:active { transform: scale(0.9); }
  .filter-toggle.active { color: var(--tint); background: var(--tint-dim); }
  .filter-bar {
    background: var(--system-bg-secondary);
    border-radius: var(--radius-card);
    padding: 12px;
    margin-bottom: 16px;
    animation: scale-in var(--duration-normal) var(--ease-decelerate);
  }
  .filter-row { display: flex; gap: 8px; margin-bottom: 8px; }
  .filter-input { flex: 1; font-size: 15px; padding: 10px 12px; }
  .filter-actions { display: flex; gap: 8px; justify-content: flex-end; }
  .btn-tint {
    padding: 8px 20px;
    border-radius: var(--radius-sm);
    background: var(--tint);
    color: white;
    font-size: 15px;
    font-weight: 600;
    border: none;
    cursor: pointer;
    transition: opacity var(--duration-fast);
  }
  .btn-tint:active { opacity: 0.7; }
  .btn-plain {
    padding: 8px 16px;
    border-radius: var(--radius-sm);
    background: none;
    color: var(--system-blue);
    font-size: 15px;
    font-weight: 400;
    border: none;
    cursor: pointer;
  }
  .btn-plain:active { opacity: 0.5; }
  .trip-list { display: flex; flex-direction: column; gap: 8px; }
  .trip-card { display: flex; gap: 12px; align-items: stretch; }
  .trip-card-info { flex: 1; min-width: 0; }
  .trip-card-date { font-size: 17px; font-weight: 600; letter-spacing: -0.01em; }
  .trip-card-time { font-size: 13px; color: var(--label-secondary); margin-top: 1px; }
  .trip-card-metrics { display: flex; align-items: center; gap: 8px; margin-top: 8px; }
  .trip-card-metric { font-size: 15px; font-weight: 600; font-variant-numeric: tabular-nums; color: var(--label-primary); }
  .metric-sep { width: 3px; height: 3px; border-radius: 50%; background: var(--label-tertiary); }
  .trip-card-tags { display: flex; flex-wrap: wrap; gap: 4px; margin-top: 8px; }
  .trip-card-map {
    width: 130px;
    min-height: 90px;
    border-radius: var(--radius-sm);
    overflow: hidden;
    flex-shrink: 0;
  }
  .load-more { display: flex; justify-content: center; padding: 20px 0; }
  .skeleton-card {
    height: 100px;
    background: linear-gradient(90deg, var(--system-bg-secondary) 25%, var(--fill-quaternary) 50%, var(--system-bg-secondary) 75%);
    background-size: 200% 100%;
    animation: shimmer 1.5s infinite;
    border-radius: var(--radius-card);
    margin-bottom: 8px;
  }
  .grouped-section { margin-bottom: 24px; }
</style>
