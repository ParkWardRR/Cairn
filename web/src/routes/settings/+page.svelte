<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import { toasts } from '$lib/stores/toast';
  import { formatDistance, formatDuration } from '$lib/utils/format';
  import { mapSettings, TILE_PROVIDERS, getActiveProvider } from '$lib/stores/mapSettings';
  import type { MapSettings } from '$lib/stores/mapSettings';
  import Card from '$lib/components/Card.svelte';
  import StatCard from '$lib/components/StatCard.svelte';
  import Dialog from '$lib/components/Dialog.svelte';
  import LeafletMap from '$lib/components/LeafletMap.svelte';

  let loading = $state(true);
  let stats: any = $state(null);
  let bulkDeleteDate = $state('');
  let deleteOpen = $state(false);

  let currentMapSettings = $state<MapSettings>({
    providerId: 'carto-dark',
    customTileUrl: '',
    customAttribution: '',
    pmtilesUrl: '',
  });

  let customUrlInput = $state('');
  let customAttrInput = $state('');
  let pmtilesUrlInput = $state('');

  const providerGroups = $derived(() => {
    const groups: Record<string, typeof TILE_PROVIDERS> = {};
    for (const p of TILE_PROVIDERS) {
      (groups[p.group] ??= []).push(p);
    }
    return groups;
  });

  let unsub: (() => void) | null = null;

  onMount(() => {
    unsub = mapSettings.subscribe((s) => {
      currentMapSettings = { ...s };
      customUrlInput = s.customTileUrl;
      customAttrInput = s.customAttribution;
      pmtilesUrlInput = s.pmtilesUrl;
    });

    api.stats().then((s) => { stats = s; }).catch(() => {}).finally(() => { loading = false; });

    return () => { unsub?.(); };
  });

  function selectProvider(id: string) {
    mapSettings.setProvider(id);
    toasts.show(`Map: ${TILE_PROVIDERS.find(p => p.id === id)?.name ?? id}`);
  }

  function applyCustomUrl() {
    if (!customUrlInput.trim()) { toasts.show('Enter a tile URL'); return; }
    mapSettings.setCustomTileUrl(customUrlInput.trim(), customAttrInput.trim());
    toasts.show('Map: Custom tiles applied');
  }

  function applyPmtiles() {
    if (!pmtilesUrlInput.trim()) { toasts.show('Enter a PMTiles URL'); return; }
    mapSettings.setPmtilesUrl(pmtilesUrlInput.trim());
    toasts.show('Map: PMTiles layer applied');
  }

  async function exportAll() {
    toasts.show('Preparing export...');
    try {
      const allTrips: any[] = [];
      let offset = 0;
      while (true) {
        const data = await api.trips({ limit: 100, offset });
        const trips = data.trips || [];
        allTrips.push(...trips);
        if (trips.length < 100) break;
        offset += 100;
      }
      const features = allTrips.map(t => ({
        type: 'Feature',
        properties: { id: t.id, device_id: t.device_id, started_at: t.started_at, ended_at: t.ended_at, distance_m: t.distance_m, duration_s: t.duration_s, tags: t.tags },
        geometry: { type: 'LineString', coordinates: [[t.start_lon, t.start_lat], [t.end_lon, t.end_lat]].filter(c => c[0] && c[1]) },
      }));
      const blob = new Blob([JSON.stringify({ type: 'FeatureCollection', features }, null, 2)], { type: 'application/json' });
      const a = document.createElement('a');
      a.href = URL.createObjectURL(blob);
      a.download = `cairn-export-${new Date().toISOString().split('T')[0]}.geojson`;
      a.click();
      URL.revokeObjectURL(a.href);
      toasts.show(`Exported ${allTrips.length} trips`);
    } catch {
      toasts.show('Export failed');
    }
  }

  async function bulkDelete() {
    toasts.show('Deleting trips...');
    try {
      const data = await api.trips({ limit: 1000, to: new Date(bulkDeleteDate).toISOString() });
      let deleted = 0;
      for (const trip of data.trips || []) {
        try { await api.deleteTrip(trip.id); deleted++; } catch {}
      }
      toasts.show(`Deleted ${deleted} trips`);
    } catch {
      toasts.show('Bulk delete failed');
    }
  }
</script>

<div class="page">
  <header class="large-title-header">
    <h1 class="large-title">Settings</h1>
    <p class="subtitle">Map, privacy, and data</p>
  </header>

  <!-- Map Engine -->
  <section class="grouped-section">
    <h2 class="section-header">Map Tiles</h2>
    <Card>
      <div class="map-preview">
        <LeafletMap interactive={false} zoom={10} />
      </div>
    </Card>
    <div class="provider-grid">
      {#each Object.entries(providerGroups()) as [group, providers]}
        <div class="provider-group">
          <span class="provider-group-label">{group}</span>
          {#each providers as p}
            <button
              class="provider-btn"
              class:active={currentMapSettings.providerId === p.id}
              onclick={() => selectProvider(p.id)}
            >
              <span class="provider-name">{p.name}</span>
              {#if p.dark}
                <span class="provider-badge dark">Dark</span>
              {:else}
                <span class="provider-badge light">Light</span>
              {/if}
            </button>
          {/each}
        </div>
      {/each}
    </div>
  </section>

  <!-- Self-Hosted Tiles -->
  <section class="grouped-section">
    <h2 class="section-header">Self-Hosted Tiles</h2>
    <Card>
      <p class="cell-description">
        Point to your own tile server for fully offline maps. Use a Rust-based server like
        <strong>martin</strong> or serve pre-rendered tiles from your homelab.
      </p>
      <div class="field-group">
        <label class="form-label" for="custom-tile-url">Raster Tile URL</label>
        <input
          id="custom-tile-url"
          type="url"
          bind:value={customUrlInput}
          placeholder="https://tiles.local/&#123;z&#125;/&#123;x&#125;/&#123;y&#125;.png"
        />
      </div>
      <div class="field-group">
        <label class="form-label" for="custom-tile-attr">Attribution</label>
        <input
          id="custom-tile-attr"
          type="text"
          bind:value={customAttrInput}
          placeholder="Self-hosted tiles"
        />
      </div>
      <button
        class="btn-tint"
        class:active={currentMapSettings.providerId === 'custom'}
        onclick={applyCustomUrl}
      >
        {currentMapSettings.providerId === 'custom' ? 'Active' : 'Use Custom Tiles'}
      </button>
    </Card>
  </section>

  <!-- PMTiles (Offline) -->
  <section class="grouped-section">
    <h2 class="section-header">PMTiles (Offline Vector)</h2>
    <Card>
      <p class="cell-description">
        Load a <strong>.pmtiles</strong> file for fully offline vector maps. Download a regional
        extract from Protomaps and serve it from your homelab. No external tile server needed.
      </p>
      <div class="field-group">
        <label class="form-label" for="pmtiles-url">PMTiles URL</label>
        <input
          id="pmtiles-url"
          type="url"
          bind:value={pmtilesUrlInput}
          placeholder="https://cairn.local/tiles/region.pmtiles"
        />
      </div>
      <button
        class="btn-tint"
        class:active={currentMapSettings.providerId === 'pmtiles'}
        onclick={applyPmtiles}
      >
        {currentMapSettings.providerId === 'pmtiles' ? 'Active' : 'Use PMTiles'}
      </button>
    </Card>
  </section>

  <!-- Data Summary -->
  <section class="grouped-section">
    <h2 class="section-header">Data Summary</h2>
    <div class="stat-grid">
      <StatCard value={stats?.total_trips ?? '--'} label="Trips" {loading} />
      <StatCard value={stats ? formatDistance(stats.total_distance_m) : '--'} label="Distance" {loading} />
      <StatCard value={stats ? formatDuration(stats.total_duration_s) : '--'} label="Driving Time" {loading} />
      <StatCard value={stats?.device_count ?? '--'} label="Devices" {loading} />
    </div>
  </section>

  <!-- Export -->
  <section class="grouped-section">
    <h2 class="section-header">Export</h2>
    <Card>
      <p class="cell-description">Download all your trip data as a single GeoJSON file.</p>
      <button class="btn-tint" onclick={exportAll}>Export All Trips</button>
    </Card>
  </section>

  <!-- Manage -->
  <section class="grouped-section">
    <h2 class="section-header">Manage Trips</h2>
    <Card>
      <p class="cell-description">Browse and delete individual trips from the <a href="/trips">Trips</a> view.</p>
    </Card>
  </section>

  <!-- Bulk Delete -->
  <section class="grouped-section">
    <h2 class="section-header">Bulk Delete</h2>
    <Card>
      <p class="cell-description">Delete all trips before a specific date. This action is irreversible.</p>
      <div class="bulk-row">
        <span class="bulk-label">Delete all trips before</span>
        <input type="date" bind:value={bulkDeleteDate} class="date-input" />
        <button
          class="btn-destructive"
          onclick={() => { if (bulkDeleteDate) deleteOpen = true; else toasts.show('Select a date'); }}
        >Delete</button>
      </div>
    </Card>
  </section>

  <!-- About -->
  <section class="grouped-section">
    <h2 class="section-header">About Cairn</h2>
    <Card>
      <p class="about-text">
        Cairn is an offline-first car journal. Trip data is collected by a Freematics ONE+ device,
        uploaded to your homelab, and stored in PostgreSQL with PostGIS. All data stays on your infrastructure.
      </p>
      <p class="about-tagline">No cloud. No tracking. Your data, your roads.</p>
    </Card>
  </section>
</div>

<Dialog title="Bulk Delete" text="Delete ALL trips before {bulkDeleteDate}? This cannot be undone." bind:open={deleteOpen} onconfirm={bulkDelete} />

<style>
  .page { padding-top: 16px; animation: slide-up var(--duration-slow) var(--ease-decelerate); }
  .large-title-header { padding: 8px 4px 4px; margin-bottom: 12px; }
  .large-title { font-size: 34px; font-weight: 700; letter-spacing: -0.03em; line-height: 1.1; }
  .subtitle { font-size: 15px; color: var(--label-secondary); margin-top: 2px; }
  .grouped-section { margin-bottom: 24px; }
  .section-header { font-size: 20px; font-weight: 700; letter-spacing: -0.02em; padding: 0 4px; margin-bottom: 8px; }

  /* Map preview */
  .map-preview {
    height: 200px;
    border-radius: var(--radius-md);
    overflow: hidden;
    margin-bottom: 4px;
  }

  /* Provider grid */
  .provider-grid {
    display: flex;
    flex-direction: column;
    gap: 12px;
    margin-top: 12px;
  }
  .provider-group {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    align-items: center;
  }
  .provider-group-label {
    font-size: 12px;
    font-weight: 600;
    color: var(--label-tertiary);
    text-transform: uppercase;
    letter-spacing: 0.05em;
    width: 70px;
    flex-shrink: 0;
  }
  .provider-btn {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 7px 14px;
    border-radius: var(--radius-full);
    background: var(--system-bg-secondary);
    color: var(--label-secondary);
    font-size: 14px;
    font-weight: 500;
    border: 1px solid transparent;
    cursor: pointer;
    transition: all var(--duration-fast) var(--ease-default);
  }
  .provider-btn:hover {
    background: var(--system-bg-tertiary);
    color: var(--label-primary);
  }
  .provider-btn.active {
    background: var(--tint-dim);
    color: var(--tint);
    border-color: var(--tint);
  }
  .provider-btn:active { transform: scale(0.97); }
  .provider-name { white-space: nowrap; }
  .provider-badge {
    font-size: 10px;
    font-weight: 600;
    padding: 1px 6px;
    border-radius: var(--radius-full);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }
  .provider-badge.dark { background: rgba(255,255,255,0.08); color: var(--label-tertiary); }
  .provider-badge.light { background: rgba(255,255,255,0.15); color: var(--label-secondary); }

  /* Self-hosted forms */
  .field-group {
    display: flex;
    flex-direction: column;
    gap: 4px;
    margin-bottom: 10px;
  }
  .form-label {
    font-size: 12px;
    font-weight: 600;
    color: var(--label-tertiary);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .stat-grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: 8px; }
  @media (min-width: 600px) { .stat-grid { grid-template-columns: repeat(4, 1fr); } }

  .cell-description {
    font-size: 15px;
    color: var(--label-secondary);
    line-height: 1.45;
    margin-bottom: 12px;
  }

  .btn-tint {
    padding: 10px 24px;
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
  .btn-tint.active {
    background: var(--system-green);
  }

  .bulk-row {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    margin-top: 4px;
  }
  .bulk-label { font-size: 15px; color: var(--label-secondary); }
  .date-input { width: auto; min-width: 150px; }
  .btn-destructive {
    padding: 8px 16px;
    border-radius: var(--radius-sm);
    background: none;
    color: var(--system-red);
    font-size: 15px;
    font-weight: 500;
    border: 1px solid var(--system-red);
    cursor: pointer;
  }
  .btn-destructive:active { background: rgba(255, 69, 58, 0.1); }

  .about-text {
    font-size: 15px;
    color: var(--label-secondary);
    line-height: 1.55;
    margin-bottom: 8px;
  }
  .about-tagline {
    font-size: 13px;
    color: var(--label-tertiary);
    font-style: italic;
  }
</style>
