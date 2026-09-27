<script lang="ts">
  import StatCard from '$lib/components/StatCard.svelte';
  import StatusDot from '$lib/components/StatusDot.svelte';
  import Card from '$lib/components/Card.svelte';
  import Tag from '$lib/components/Tag.svelte';
</script>

<div class="page">
  <header class="large-title-header">
    <h1 class="large-title">Today</h1>
    <div class="sync-pill">
      <StatusDot color="green" pulse />
      <span>Synced 12m ago</span>
    </div>
  </header>

  <section class="grouped-section">
    <div class="stat-grid">
      <StatCard value={3} label="Today" />
      <StatCard value={12} label="This Week" />
      <StatCard value={47} label="This Month" />
      <StatCard value={284} label="All Time" />
    </div>
  </section>

  <section class="grouped-section">
    <h2 class="section-header">Most Recent Trip</h2>
    <Card>
      <div class="recent-trip">
        <div class="recent-trip-info">
          <div class="recent-trip-date">Thu, Sep 26</div>
          <div class="recent-trip-time">5:42 PM - 6:18 PM</div>
          <div class="trip-metrics">
            <div class="metric">
              <span class="metric-value">14.3 km</span>
              <span class="metric-label">Distance</span>
            </div>
            <div class="metric">
              <span class="metric-value">36m</span>
              <span class="metric-label">Duration</span>
            </div>
            <div class="metric">
              <span class="metric-value">5:42 PM</span>
              <span class="metric-label">Start</span>
            </div>
            <div class="metric">
              <span class="metric-value">6:18 PM</span>
              <span class="metric-label">End</span>
            </div>
          </div>
          <div class="trip-tags">
            <Tag label="commute" />
            <Tag label="highway" />
          </div>
        </div>
        <div class="mock-map">
          <svg viewBox="0 0 200 140" xmlns="http://www.w3.org/2000/svg">
            <rect width="200" height="140" fill="#0d0d12"/>
            <line x1="0" y1="35" x2="200" y2="35" stroke="#1a1a24" stroke-width="1.2"/>
            <line x1="0" y1="70" x2="200" y2="70" stroke="#1a1a24" stroke-width="1.2"/>
            <line x1="0" y1="105" x2="200" y2="105" stroke="#1a1a24" stroke-width="1.2"/>
            <line x1="50" y1="0" x2="50" y2="140" stroke="#1a1a24" stroke-width="1.2"/>
            <line x1="100" y1="0" x2="100" y2="140" stroke="#1a1a24" stroke-width="1.2"/>
            <line x1="150" y1="0" x2="150" y2="140" stroke="#1a1a24" stroke-width="1.2"/>
            <rect x="55" y="40" width="40" height="25" rx="1.5" fill="#141420" opacity="0.5"/>
            <rect x="105" y="40" width="40" height="25" rx="1.5" fill="#141420" opacity="0.4"/>
            <rect x="55" y="75" width="40" height="25" rx="1.5" fill="#141420" opacity="0.3"/>
            <rect x="105" y="75" width="40" height="25" rx="1.5" fill="#141420" opacity="0.5"/>
            <path d="M25 115 C45 105, 55 85, 75 70 S110 40, 130 38 S165 45, 175 28" fill="none" stroke="#ff375f" stroke-width="2.5" opacity="0.75" stroke-linecap="round"/>
            <circle cx="25" cy="115" r="5" fill="#30d158"/>
            <circle cx="175" cy="28" r="5" fill="#ff375f"/>
          </svg>
        </div>
      </div>
    </Card>
  </section>

  <div class="two-col">
    <section class="grouped-section">
      <h2 class="section-header">Last Parked</h2>
      <Card>
        <div class="parked-map">
          <svg viewBox="0 0 400 170" xmlns="http://www.w3.org/2000/svg">
            <rect width="400" height="170" fill="#0d0d12"/>
            <line x1="0" y1="45" x2="400" y2="45" stroke="#1a1a24" stroke-width="1.5"/>
            <line x1="0" y1="90" x2="400" y2="90" stroke="#1a1a24" stroke-width="1.5"/>
            <line x1="0" y1="135" x2="400" y2="135" stroke="#1a1a24" stroke-width="1.5"/>
            <line x1="80" y1="0" x2="80" y2="170" stroke="#1a1a24" stroke-width="1.5"/>
            <line x1="200" y1="0" x2="200" y2="170" stroke="#1a1a24" stroke-width="1.5"/>
            <line x1="320" y1="0" x2="320" y2="170" stroke="#1a1a24" stroke-width="1.5"/>
            <rect x="90" y="50" width="100" height="35" rx="2" fill="#141420" opacity="0.5"/>
            <rect x="210" y="50" width="100" height="35" rx="2" fill="#141420" opacity="0.5"/>
            <rect x="90" y="95" width="100" height="35" rx="2" fill="#141420" opacity="0.3"/>
            <rect x="210" y="95" width="100" height="35" rx="2" fill="#141420" opacity="0.4"/>
            <circle cx="200" cy="85" r="18" fill="none" stroke="#ff375f" stroke-width="1.5" opacity="0.2"/>
            <circle cx="200" cy="85" r="10" fill="#ff375f" opacity="0.85"/>
            <circle cx="200" cy="85" r="3.5" fill="#0d0d12"/>
          </svg>
        </div>
        <div class="parked-info">
          <span class="parked-coords">37.77490, -122.41940</span>
          <span class="parked-time">36m ago</span>
        </div>
      </Card>
    </section>

    <section class="grouped-section">
      <h2 class="section-header">Devices</h2>
      <Card>
        <div class="device-row">
          <StatusDot color="green" pulse />
          <span class="device-id">FMTC-001A</span>
          <span class="device-time">12m ago</span>
        </div>
        <div class="device-row border-top">
          <StatusDot color="amber" />
          <span class="device-id">FMTC-002B</span>
          <span class="device-time">6h ago</span>
        </div>
      </Card>
    </section>
  </div>
</div>

<style>
  .page {
    padding-top: 16px;
    animation: slide-up var(--duration-slow) var(--ease-decelerate);
  }
  .large-title-header { padding: 8px 4px 4px; margin-bottom: 8px; }
  .large-title { font-size: 34px; font-weight: 700; letter-spacing: -0.03em; line-height: 1.1; }
  .sync-pill {
    display: inline-flex; align-items: center; gap: 6px;
    margin-top: 8px; padding: 4px 12px 4px 8px;
    background: var(--system-bg-secondary);
    border-radius: var(--radius-full);
    font-size: 13px; color: var(--label-secondary); font-weight: 500;
  }
  .grouped-section { margin-bottom: 24px; }
  .section-header { font-size: 20px; font-weight: 700; letter-spacing: -0.02em; padding: 0 4px; margin-bottom: 8px; }
  .stat-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 8px;
  }
  @media (min-width: 600px) {
    .stat-grid { grid-template-columns: repeat(4, 1fr); }
  }

  .recent-trip { display: flex; flex-direction: column; gap: 12px; }
  @media (min-width: 600px) {
    .recent-trip { flex-direction: row; align-items: stretch; }
  }
  .recent-trip-info { flex: 1; }
  .recent-trip-date { font-size: 17px; font-weight: 600; letter-spacing: -0.01em; }
  .recent-trip-time { font-size: 13px; color: var(--label-secondary); margin-top: 2px; }
  .trip-metrics { display: flex; gap: 24px; margin-top: 12px; }
  .metric { display: flex; flex-direction: column; }
  .metric-value { font-size: 17px; font-weight: 600; font-variant-numeric: tabular-nums; }
  .metric-label { font-size: 11px; color: var(--label-tertiary); text-transform: uppercase; letter-spacing: 0.04em; font-weight: 600; }
  .trip-tags { display: flex; gap: 6px; margin-top: 12px; }

  .mock-map {
    width: 100%; min-height: 140px;
    border-radius: 10px; overflow: hidden; flex-shrink: 0;
  }
  @media (min-width: 600px) {
    .mock-map { width: 200px; }
  }

  .two-col {
    display: grid;
    grid-template-columns: 1fr;
    gap: 0;
  }
  @media (min-width: 600px) {
    .two-col { grid-template-columns: 1fr 1fr; gap: 16px; }
  }

  .parked-map {
    height: 170px; border-radius: 10px; overflow: hidden; margin-bottom: 10px;
  }
  .parked-info { display: flex; flex-direction: column; gap: 2px; }
  .parked-coords { font-size: 13px; font-family: var(--font-mono); color: var(--label-secondary); }
  .parked-time { font-size: 12px; color: var(--label-tertiary); }

  .device-row { display: flex; align-items: center; gap: 8px; padding: 10px 0; }
  .device-row.border-top { border-top: 0.5px solid var(--separator); }
  .device-id { font-family: var(--font-mono); font-size: 14px; font-weight: 500; flex: 1; }
  .device-time { font-size: 13px; color: var(--label-tertiary); }
</style>
