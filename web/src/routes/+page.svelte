<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import { formatDistance, formatDuration, formatDate, formatTime, formatDateTime, relativeTime, isToday, isThisWeek, isThisMonth, deviceStatusColor } from '$lib/utils/format';
  import StatCard from '$lib/components/StatCard.svelte';
  import StatusDot from '$lib/components/StatusDot.svelte';
  import MapLibreMap from '$lib/components/MapLibreMap.svelte';
  import Card from '$lib/components/Card.svelte';
  import type { Map as MLMap } from 'maplibre-gl';

  let loading = $state(true);
  let trips: any[] = $state([]);
  let totalTrips = $state(0);
  let devices: any[] = $state([]);
  let syncStatus = $state<'checking' | 'synced' | 'offline'>('checking');
  let lastSyncTime = $state('');

  let todayCount = $derived(trips.filter(t => isToday(t.started_at)).length);
  let weekCount = $derived(trips.filter(t => isThisWeek(t.started_at)).length);
  let monthCount = $derived(trips.filter(t => isThisMonth(t.started_at)).length);
  let recentTrip = $derived(trips[0] ?? null);
  let parkedLat = $derived(recentTrip?.end_lat || recentTrip?.start_lat);
  let parkedLon = $derived(recentTrip?.end_lon || recentTrip?.start_lon);

  onMount(async () => {
    const [statsRes, tripsRes, devicesRes] = await Promise.allSettled([
      api.stats(),
      api.trips({ limit: 50 }),
      api.devices(),
    ]);

    if (tripsRes.status === 'fulfilled' && tripsRes.value) {
      trips = tripsRes.value.trips || [];
      totalTrips = tripsRes.value.total || trips.length;
    }
    if (devicesRes.status === 'fulfilled' && devicesRes.value) {
      devices = devicesRes.value.devices || [];
    }
    if (statsRes.status === 'fulfilled' && statsRes.value) {
      syncStatus = 'synced';
      lastSyncTime = statsRes.value.recent_trip?.ended_at || '';
    } else {
      syncStatus = 'offline';
    }
    loading = false;
  });

  function setupRecentMap(map: MLMap) {
    if (!recentTrip) return;
    const points: [number, number][] = [];
    if (recentTrip.start_lat && recentTrip.start_lon) points.push([recentTrip.start_lon, recentTrip.start_lat]);
    if (recentTrip.end_lat && recentTrip.end_lon) points.push([recentTrip.end_lon, recentTrip.end_lat]);
    if (points.length === 0) return;

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
        'circle-radius': 6,
        'circle-color': ['match', ['get', 'type'], 'start', '#30d158', '#ff375f'],
        'circle-stroke-width': 2,
        'circle-stroke-color': '#ffffff',
      },
    });

    if (points.length === 2) {
      map.addSource('route-line', {
        type: 'geojson',
        data: {
          type: 'Feature',
          geometry: { type: 'LineString', coordinates: points },
          properties: {},
        },
      });
      map.addLayer({
        id: 'route-line-layer',
        type: 'line',
        source: 'route-line',
        paint: { 'line-color': '#ff375f', 'line-width': 2, 'line-opacity': 0.5 },
      }, 'endpoint-circles');

      const lngs = points.map(p => p[0]);
      const lats = points.map(p => p[1]);
      map.fitBounds(
        [[Math.min(...lngs), Math.min(...lats)], [Math.max(...lngs), Math.max(...lats)]],
        { padding: 30, duration: 0 },
      );
    } else {
      map.setCenter(points[0]);
      map.setZoom(14);
    }
  }

  function setupParkedMap(map: MLMap) {
    if (!parkedLat || !parkedLon) return;
    map.setCenter([parkedLon, parkedLat]);
    map.setZoom(15);

    map.addSource('parked', {
      type: 'geojson',
      data: {
        type: 'Feature',
        geometry: { type: 'Point', coordinates: [parkedLon, parkedLat] },
        properties: {},
      },
    });
    map.addLayer({
      id: 'parked-circle',
      type: 'circle',
      source: 'parked',
      paint: {
        'circle-radius': 8,
        'circle-color': '#ff375f',
        'circle-opacity': 0.8,
        'circle-stroke-width': 2,
        'circle-stroke-color': '#ffffff',
      },
    });
  }
</script>

<div class="page">
  <header class="large-title-header">
    <h1 class="large-title">Today</h1>
    <div class="sync-pill">
      {#if syncStatus === 'checking'}
        <StatusDot color="amber" />
        <span>Checking...</span>
      {:else if syncStatus === 'synced'}
        <StatusDot color="green" pulse />
        <span>Synced {lastSyncTime ? relativeTime(lastSyncTime) : ''}</span>
      {:else}
        <StatusDot color="red" />
        <span>Offline</span>
      {/if}
    </div>
  </header>

  <section class="grouped-section">
    <div class="stat-grid">
      <StatCard value={loading ? '--' : todayCount} label="Today" {loading} />
      <StatCard value={loading ? '--' : weekCount} label="This Week" {loading} />
      <StatCard value={loading ? '--' : monthCount} label="This Month" {loading} />
      <StatCard value={loading ? '--' : totalTrips} label="All Time" {loading} />
    </div>
  </section>

  <section class="grouped-section">
    <h2 class="section-header">Most Recent Trip</h2>
    {#if loading}
      <Card>
        <div class="skeleton-block"></div>
      </Card>
    {:else if recentTrip}
      <Card clickable href="/trips/{recentTrip.id}">
        <div class="recent-trip">
          <div class="recent-trip-info">
            <div class="recent-trip-date">{formatDate(recentTrip.started_at)}</div>
            <div class="recent-trip-time">{formatTime(recentTrip.started_at)} - {formatTime(recentTrip.ended_at)}</div>
            <div class="trip-metrics">
              <div class="metric">
                <span class="metric-value">{formatDistance(recentTrip.distance_m)}</span>
                <span class="metric-label">Distance</span>
              </div>
              <div class="metric">
                <span class="metric-value">{formatDuration(recentTrip.duration_s)}</span>
                <span class="metric-label">Duration</span>
              </div>
            </div>
          </div>
          {#if recentTrip.start_lat}
            <div class="recent-trip-map">
              <MapLibreMap interactive={false} onready={setupRecentMap} />
            </div>
          {/if}
        </div>
      </Card>
    {:else}
      <Card>
        <div class="empty-inline">
          <span class="empty-inline-text">No trips yet. Trips appear here once your device uploads data.</span>
        </div>
      </Card>
    {/if}
  </section>

  <div class="two-col">
    <section class="grouped-section">
      <h2 class="section-header">Last Parked</h2>
      <Card>
        {#if parkedLat && parkedLon}
          <div class="parked-map-wrap">
            <MapLibreMap interactive={false} onready={setupParkedMap} />
          </div>
          <div class="parked-info">
            <span class="parked-coords">{parkedLat?.toFixed(5)}, {parkedLon?.toFixed(5)}</span>
            <span class="parked-time">{relativeTime(recentTrip?.ended_at)}</span>
          </div>
        {:else}
          <div class="empty-inline">
            <span class="empty-inline-text">No location data</span>
          </div>
        {/if}
      </Card>
    </section>

    <section class="grouped-section">
      <h2 class="section-header">Devices</h2>
      <Card>
        {#if loading}
          <div class="skeleton-line"></div>
          <div class="skeleton-line short"></div>
        {:else if devices.length > 0}
          {#each devices as d, i}
            <div class="device-row" class:border-top={i > 0}>
              <StatusDot color={deviceStatusColor(d.last_seen_at)} />
              <span class="device-id">{d.id}</span>
              <span class="device-time">{relativeTime(d.last_seen_at)}</span>
            </div>
          {/each}
        {:else}
          <div class="empty-inline">
            <span class="empty-inline-text">No devices found</span>
          </div>
        {/if}
      </Card>
    </section>
  </div>
</div>

<style>
  .page {
    padding-top: 16px;
    animation: slide-up var(--duration-slow) var(--ease-decelerate);
  }
  .large-title-header {
    padding: 8px 4px 4px;
    margin-bottom: 8px;
  }
  .large-title {
    font-size: 34px;
    font-weight: 700;
    letter-spacing: -0.03em;
    line-height: 1.1;
    color: var(--label-primary);
  }
  .sync-pill {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    margin-top: 8px;
    padding: 4px 12px 4px 8px;
    background: var(--system-bg-secondary);
    border-radius: var(--radius-full);
    font-size: 13px;
    color: var(--label-secondary);
    font-weight: 500;
  }
  .grouped-section { margin-bottom: 24px; }
  .section-header {
    font-size: 20px;
    font-weight: 700;
    letter-spacing: -0.02em;
    color: var(--label-primary);
    padding: 0 4px;
    margin-bottom: 8px;
  }
  .stat-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 8px;
  }
  @media (min-width: 600px) {
    .stat-grid { grid-template-columns: repeat(4, 1fr); }
  }
  .recent-trip {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  @media (min-width: 600px) {
    .recent-trip { flex-direction: row; align-items: stretch; }
  }
  .recent-trip-info { flex: 1; }
  .recent-trip-date { font-size: 17px; font-weight: 600; letter-spacing: -0.01em; }
  .recent-trip-time { font-size: 13px; color: var(--label-secondary); margin-top: 2px; }
  .trip-metrics { display: flex; gap: 24px; margin-top: 12px; }
  .metric { display: flex; flex-direction: column; }
  .metric-value { font-size: 17px; font-weight: 600; font-variant-numeric: tabular-nums; }
  .metric-label {
    font-size: 11px;
    color: var(--label-tertiary);
    text-transform: uppercase;
    letter-spacing: 0.04em;
    font-weight: 600;
  }
  .recent-trip-map {
    width: 100%;
    height: 140px;
    border-radius: var(--radius-md);
    overflow: hidden;
  }
  @media (min-width: 600px) {
    .recent-trip-map { width: 180px; height: auto; min-height: 120px; }
  }
  .two-col { display: grid; grid-template-columns: 1fr; gap: 0; }
  @media (min-width: 600px) {
    .two-col { grid-template-columns: 1fr 1fr; gap: 16px; }
  }
  .parked-map-wrap {
    height: 180px;
    border-radius: var(--radius-md);
    overflow: hidden;
    margin-bottom: 10px;
  }
  .parked-info { display: flex; flex-direction: column; gap: 2px; }
  .parked-coords { font-size: 13px; font-family: var(--font-mono); color: var(--label-secondary); }
  .parked-time { font-size: 12px; color: var(--label-tertiary); }
  .device-row { display: flex; align-items: center; gap: 8px; padding: 10px 0; }
  .device-row.border-top { border-top: 0.5px solid var(--separator); }
  .device-id { font-family: var(--font-mono); font-size: 14px; font-weight: 500; flex: 1; }
  .device-time { font-size: 13px; color: var(--label-tertiary); }
  .empty-inline { padding: 16px 0; }
  .empty-inline-text { font-size: 15px; color: var(--label-tertiary); }
  .skeleton-block {
    height: 160px;
    background: linear-gradient(90deg, var(--system-bg-tertiary) 25%, var(--fill-quaternary) 50%, var(--system-bg-tertiary) 75%);
    background-size: 200% 100%;
    animation: shimmer 1.5s infinite;
    border-radius: var(--radius-md);
  }
  .skeleton-line {
    height: 14px;
    width: 100%;
    background: linear-gradient(90deg, var(--system-bg-tertiary) 25%, var(--fill-quaternary) 50%, var(--system-bg-tertiary) 75%);
    background-size: 200% 100%;
    animation: shimmer 1.5s infinite;
    border-radius: var(--radius-sm);
    margin-bottom: 8px;
  }
  .skeleton-line.short { width: 60%; }
</style>
