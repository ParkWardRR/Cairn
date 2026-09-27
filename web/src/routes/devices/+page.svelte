<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import { formatDistance, relativeTime, deviceStatusColor } from '$lib/utils/format';
  import Card from '$lib/components/Card.svelte';
  import StatusDot from '$lib/components/StatusDot.svelte';
  import EmptyState from '$lib/components/EmptyState.svelte';

  let loading = $state(true);
  let devices: any[] = $state([]);

  onMount(async () => {
    try {
      const data = await api.devices();
      devices = data.devices || [];
    } catch {}
    loading = false;
  });
</script>

<div class="page">
  <header class="large-title-header">
    <h1 class="large-title">Devices</h1>
    <p class="subtitle">Connected Freematics devices</p>
  </header>

  {#if loading}
    <section class="device-list">
      {#each Array(2) as _}
        <div class="skeleton-card"></div>
      {/each}
    </section>
  {:else if devices.length === 0}
    <EmptyState
      title="No devices found"
      description="Devices appear here once they connect and upload data."
    />
  {:else}
    <section class="device-list">
      {#each devices as d (d.id)}
        <Card>
          <div class="device-card">
            <div class="device-header">
              <StatusDot color={deviceStatusColor(d.last_seen_at)} pulse={deviceStatusColor(d.last_seen_at) === 'green'} />
              <span class="device-id">{d.id}</span>
              <span class="device-seen">{relativeTime(d.last_seen_at)}</span>
            </div>
            {#if d.firmware_version}
              <div class="device-firmware">Firmware {d.firmware_version}</div>
            {/if}
            <div class="device-metrics">
              <div class="device-metric">
                <span class="device-metric-val">{d.trip_count ?? '--'}</span>
                <span class="device-metric-label">Trips</span>
              </div>
              <div class="device-metric">
                <span class="device-metric-val">{formatDistance(d.total_distance_m)}</span>
                <span class="device-metric-label">Distance</span>
              </div>
            </div>
            <a href="/trips?device_id={encodeURIComponent(d.id)}" class="view-trips-btn">
              View Trips
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><path d="M9 5l7 7-7 7"/></svg>
            </a>
          </div>
        </Card>
      {/each}
    </section>
  {/if}
</div>

<style>
  .page { padding-top: 16px; animation: slide-up var(--duration-slow) var(--ease-decelerate); }
  .large-title-header { padding: 8px 4px 4px; margin-bottom: 12px; }
  .large-title { font-size: 34px; font-weight: 700; letter-spacing: -0.03em; line-height: 1.1; }
  .subtitle { font-size: 15px; color: var(--label-secondary); margin-top: 2px; }

  .device-list { display: flex; flex-direction: column; gap: 8px; }

  .device-card { display: flex; flex-direction: column; gap: 8px; }
  .device-header { display: flex; align-items: center; gap: 8px; }
  .device-id { font-family: var(--font-mono); font-size: 16px; font-weight: 600; flex: 1; }
  .device-seen { font-size: 13px; color: var(--label-tertiary); }
  .device-firmware { font-size: 13px; color: var(--label-tertiary); }

  .device-metrics { display: flex; gap: 24px; }
  .device-metric { display: flex; flex-direction: column; }
  .device-metric-val { font-size: 20px; font-weight: 700; font-variant-numeric: tabular-nums; letter-spacing: -0.02em; }
  .device-metric-label { font-size: 11px; color: var(--label-tertiary); text-transform: uppercase; letter-spacing: 0.04em; font-weight: 600; }

  .view-trips-btn {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    color: var(--tint);
    font-size: 15px;
    font-weight: 400;
    text-decoration: none;
    margin-top: 4px;
  }
  .view-trips-btn:active { opacity: 0.5; }

  .skeleton-card {
    height: 120px;
    background: linear-gradient(90deg, var(--system-bg-secondary) 25%, var(--fill-quaternary) 50%, var(--system-bg-secondary) 75%);
    background-size: 200% 100%;
    animation: shimmer 1.5s infinite;
    border-radius: var(--radius-card);
  }
</style>
