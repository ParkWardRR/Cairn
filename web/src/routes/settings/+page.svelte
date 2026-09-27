<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import { toasts } from '$lib/stores/toast';
  import { formatDistance, formatDuration } from '$lib/utils/format';
  import Card from '$lib/components/Card.svelte';
  import StatCard from '$lib/components/StatCard.svelte';
  import Dialog from '$lib/components/Dialog.svelte';

  let loading = $state(true);
  let stats: any = $state(null);
  let bulkDeleteDate = $state('');
  let deleteOpen = $state(false);

  onMount(async () => {
    try {
      stats = await api.stats();
    } catch {}
    loading = false;
  });

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
    <h1 class="large-title">Privacy & Data</h1>
    <p class="subtitle">Manage your driving data</p>
  </header>

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
  }
  .btn-tint:active { opacity: 0.7; }

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
