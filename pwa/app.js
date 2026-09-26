/* ============================================================
   Cairn PWA — app.js
   Router, API client, Views, Utilities
   ============================================================ */

// ─── Utilities ────────────────────────────────────────────────

function formatDistance(meters) {
  if (meters == null) return '--';
  const km = meters / 1000;
  if (km < 1) return `${Math.round(meters)} m`;
  return `${km.toFixed(1)} km`;
}

function formatDuration(seconds) {
  if (seconds == null) return '--';
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  if (h === 0 && m === 0) return `${Math.round(seconds)}s`;
  if (h === 0) return `${m}m`;
  return `${h}h ${m}m`;
}

function formatTime(iso) {
  if (!iso) return '--';
  const d = new Date(iso);
  return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

function formatDate(iso) {
  if (!iso) return '--';
  const d = new Date(iso);
  return d.toLocaleDateString([], { weekday: 'short', month: 'short', day: 'numeric' });
}

function formatDateTime(iso) {
  if (!iso) return '--';
  return `${formatDate(iso)} ${formatTime(iso)}`;
}

function relativeTime(iso) {
  if (!iso) return 'unknown';
  const now = Date.now();
  const then = new Date(iso).getTime();
  const diff = Math.max(0, now - then);
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days}d ago`;
  return formatDate(iso);
}

function isToday(iso) {
  if (!iso) return false;
  const d = new Date(iso);
  const now = new Date();
  return d.toDateString() === now.toDateString();
}

function isThisWeek(iso) {
  if (!iso) return false;
  const d = new Date(iso);
  const now = new Date();
  const weekAgo = new Date(now);
  weekAgo.setDate(weekAgo.getDate() - 7);
  return d >= weekAgo;
}

function isThisMonth(iso) {
  if (!iso) return false;
  const d = new Date(iso);
  const now = new Date();
  return d.getMonth() === now.getMonth() && d.getFullYear() === now.getFullYear();
}

function escapeHtml(str) {
  const div = document.createElement('div');
  div.textContent = str;
  return div.innerHTML;
}

function deviceStatusColor(lastSeen) {
  if (!lastSeen) return 'red';
  const diff = Date.now() - new Date(lastSeen).getTime();
  if (diff < 3600000) return 'green';
  if (diff < 86400000) return 'yellow';
  return 'red';
}

function skeleton(type = 'text', extra = '') {
  return `<div class="skeleton skeleton-${type} ${extra}"></div>`;
}

function showToast(message) {
  const existing = document.querySelector('.toast');
  if (existing) existing.remove();
  const el = document.createElement('div');
  el.className = 'toast';
  el.textContent = message;
  document.body.appendChild(el);
  setTimeout(() => el.remove(), 3000);
}

function confirm(title, text) {
  return new Promise((resolve) => {
    const overlay = document.createElement('div');
    overlay.className = 'dialog-overlay';
    overlay.innerHTML = `
      <div class="dialog">
        <div class="dialog-title">${escapeHtml(title)}</div>
        <div class="dialog-text">${escapeHtml(text)}</div>
        <div class="dialog-actions">
          <button class="btn btn-secondary" data-action="cancel">Cancel</button>
          <button class="btn btn-danger" data-action="confirm">Delete</button>
        </div>
      </div>
    `;
    overlay.querySelector('[data-action="cancel"]').onclick = () => { overlay.remove(); resolve(false); };
    overlay.querySelector('[data-action="confirm"]').onclick = () => { overlay.remove(); resolve(true); };
    overlay.onclick = (e) => { if (e.target === overlay) { overlay.remove(); resolve(false); } };
    document.body.appendChild(overlay);
  });
}

// ─── Tile Layer ───────────────────────────────────────────────

const DARK_TILES = 'https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png';
const TILE_ATTR = '&copy; <a href="https://www.openstreetmap.org/copyright">OSM</a> &copy; <a href="https://carto.com/">CARTO</a>';

function createTileLayer() {
  return L.tileLayer(DARK_TILES, { attribution: TILE_ATTR, maxZoom: 19 });
}

// Map instance tracking for cleanup
const mapInstances = new Map();

function destroyMaps() {
  for (const [id, map] of mapInstances) {
    map.remove();
  }
  mapInstances.clear();
}

function createMap(containerId, options = {}) {
  const el = document.getElementById(containerId);
  if (!el) return null;
  const map = L.map(containerId, {
    zoomControl: options.zoomControl !== false,
    attributionControl: options.attributionControl !== false,
    ...options,
  });
  createTileLayer().addTo(map);
  mapInstances.set(containerId, map);
  return map;
}

// ─── API Client ───────────────────────────────────────────────

const API = {
  base: '/api/v1',

  async request(path, options = {}) {
    const url = `${this.base}${path}`;
    try {
      const resp = await fetch(url, {
        headers: { 'Content-Type': 'application/json', ...options.headers },
        ...options,
      });
      if (resp.status === 204) return null;
      if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
      const ct = resp.headers.get('content-type') || '';
      if (ct.includes('json')) return resp.json();
      return resp;
    } catch (err) {
      console.error(`API ${options.method || 'GET'} ${path}:`, err);
      throw err;
    }
  },

  get(path)          { return this.request(path); },
  post(path, body)   { return this.request(path, { method: 'POST', body: JSON.stringify(body) }); },
  put(path, body)    { return this.request(path, { method: 'PUT', body: JSON.stringify(body) }); },
  del(path)          { return this.request(path, { method: 'DELETE' }); },

  // Endpoints
  health()                  { return this.get('/health'); },
  stats()                   { return this.get('/stats'); },
  trips(params = {})        {
    const q = new URLSearchParams();
    if (params.limit)     q.set('limit', params.limit);
    if (params.offset)    q.set('offset', params.offset);
    if (params.device_id) q.set('device_id', params.device_id);
    if (params.tag)       q.set('tag', params.tag);
    if (params.from)      q.set('from', params.from);
    if (params.to)        q.set('to', params.to);
    const qs = q.toString();
    return this.get(`/trips${qs ? '?' + qs : ''}`);
  },
  trip(id)                  { return this.get(`/trips/${id}`); },
  tripRoute(id)             { return this.get(`/trips/${id}/route`); },
  tripEvents(id)            { return this.get(`/trips/${id}/events`); },
  deleteTrip(id)            { return this.del(`/trips/${id}`); },
  addTag(tripId, tag)       { return this.post(`/trips/${tripId}/tags`, { tag }); },
  removeTag(tripId, tag)    { return this.del(`/trips/${tripId}/tags/${encodeURIComponent(tag)}`); },
  devices()                 { return this.get('/devices'); },
  deviceStatus(id)          { return this.get(`/devices/${id}/status`); },
  places()                  { return this.get('/places'); },
  createPlace(data)         { return this.post('/places', data); },
  updatePlace(id, data)     { return this.put(`/places/${id}`, data); },
  deletePlace(id)           { return this.del(`/places/${id}`); },
};

// ─── Router ───────────────────────────────────────────────────

const Router = {
  routes: [],
  current: null,

  add(pattern, handler) {
    // Convert "/trips/:id" into a regex
    const keys = [];
    const re = pattern.replace(/:(\w+)/g, (_, key) => {
      keys.push(key);
      return '([^/]+)';
    });
    this.routes.push({ pattern: new RegExp(`^${re}$`), keys, handler });
  },

  resolve(path) {
    for (const route of this.routes) {
      const match = path.match(route.pattern);
      if (match) {
        const params = {};
        route.keys.forEach((key, i) => { params[key] = match[i + 1]; });
        return { handler: route.handler, params };
      }
    }
    return null;
  },

  async navigate(path, pushState = true) {
    if (pushState) {
      window.history.pushState(null, '', path);
    }
    const result = this.resolve(path);
    if (result) {
      this.current = path;
      this.updateActiveNav(path);
      destroyMaps();
      const app = document.getElementById('app');
      app.style.animation = 'none';
      // Force reflow
      void app.offsetHeight;
      app.style.animation = '';
      await result.handler(result.params);
    } else {
      document.getElementById('app').innerHTML = `
        <div class="empty-state">
          <div class="empty-state-icon">404</div>
          <div class="empty-state-title">Page not found</div>
          <div class="empty-state-text">The page you're looking for doesn't exist.</div>
        </div>
      `;
    }
  },

  updateActiveNav(path) {
    document.querySelectorAll('.nav-link').forEach((link) => {
      const route = link.dataset.route;
      if (route === '/' && path === '/') {
        link.classList.add('active');
      } else if (route !== '/' && path.startsWith(route)) {
        link.classList.add('active');
      } else {
        link.classList.remove('active');
      }
    });
  },

  init() {
    // Handle link clicks
    document.addEventListener('click', (e) => {
      const link = e.target.closest('a[href]');
      if (!link) return;
      const href = link.getAttribute('href');
      if (!href || href.startsWith('http') || href.startsWith('#')) return;
      e.preventDefault();
      this.navigate(href);
    });

    // Handle back/forward
    window.addEventListener('popstate', () => {
      this.navigate(window.location.pathname, false);
    });

    // Initial route
    this.navigate(window.location.pathname, false);
  },
};

// ─── Views ────────────────────────────────────────────────────

// === Today View ===

async function viewToday() {
  const app = document.getElementById('app');
  app.innerHTML = `
    <div class="page-header">
      <h1 class="page-title">Today</h1>
      <div id="sync-status" class="sync-status" style="margin-top:0.5rem">
        <span class="status-dot yellow"></span> Checking...
      </div>
    </div>
    <div class="stat-grid" id="quick-stats">
      <div class="stat-card">${skeleton('stat')}<div class="stat-label">Today</div></div>
      <div class="stat-card">${skeleton('stat')}<div class="stat-label">This week</div></div>
      <div class="stat-card">${skeleton('stat')}<div class="stat-label">This month</div></div>
      <div class="stat-card">${skeleton('stat')}<div class="stat-label">Total</div></div>
    </div>
    <div class="dashboard-grid">
      <div class="card dashboard-wide" id="recent-trip">
        <div class="card-header"><span class="card-title">Most Recent Trip</span></div>
        <div class="card-body">
          ${skeleton('text', 'medium')}
          ${skeleton('text', 'short')}
          ${skeleton('map')}
        </div>
      </div>
      <div class="card" id="parked-location">
        <div class="card-header"><span class="card-title">Last Parked</span></div>
        <div class="card-body">
          <div id="parked-map" class="map-mini" style="margin-bottom:0.5rem"></div>
          <div id="parked-info">${skeleton('text', 'medium')}</div>
        </div>
      </div>
      <div class="card" id="device-health">
        <div class="card-header"><span class="card-title">Devices</span></div>
        <div class="card-body" id="device-list-mini">
          ${skeleton('text')}${skeleton('text')}
        </div>
      </div>
    </div>
  `;

  // Load all data concurrently
  const [statsData, tripsData, devicesData] = await Promise.allSettled([
    API.stats(),
    API.trips({ limit: 50 }),
    API.devices(),
  ]);

  // Quick stats
  const statsEl = document.getElementById('quick-stats');
  if (tripsData.status === 'fulfilled' && tripsData.value) {
    const trips = tripsData.value.trips || [];
    const todayCount = trips.filter(t => isToday(t.started_at)).length;
    const weekCount = trips.filter(t => isThisWeek(t.started_at)).length;
    const monthCount = trips.filter(t => isThisMonth(t.started_at)).length;
    const total = tripsData.value.total || trips.length;

    statsEl.innerHTML = `
      <div class="stat-card"><div class="stat-value">${todayCount}</div><div class="stat-label">Today</div></div>
      <div class="stat-card"><div class="stat-value">${weekCount}</div><div class="stat-label">This week</div></div>
      <div class="stat-card"><div class="stat-value">${monthCount}</div><div class="stat-label">This month</div></div>
      <div class="stat-card"><div class="stat-value">${total}</div><div class="stat-label">Total trips</div></div>
    `;
  } else {
    statsEl.innerHTML = `
      <div class="stat-card"><div class="stat-value">--</div><div class="stat-label">Today</div></div>
      <div class="stat-card"><div class="stat-value">--</div><div class="stat-label">This week</div></div>
      <div class="stat-card"><div class="stat-value">--</div><div class="stat-label">This month</div></div>
      <div class="stat-card"><div class="stat-value">--</div><div class="stat-label">Total trips</div></div>
    `;
  }

  // Sync status
  const syncEl = document.getElementById('sync-status');
  if (statsData.status === 'fulfilled' && statsData.value) {
    const st = statsData.value;
    const lastUpload = st.recent_trip?.ended_at;
    syncEl.innerHTML = `
      <span class="status-dot green"></span> Synced ${lastUpload ? relativeTime(lastUpload) : 'unknown'}
    `;
  } else {
    syncEl.innerHTML = `<span class="status-dot red"></span> Backend unreachable`;
  }

  // Recent trip
  const recentEl = document.getElementById('recent-trip');
  if (tripsData.status === 'fulfilled' && tripsData.value?.trips?.length > 0) {
    const trip = tripsData.value.trips[0];
    recentEl.innerHTML = `
      <div class="card-header">
        <span class="card-title">Most Recent Trip</span>
        <span class="card-meta">${formatDateTime(trip.started_at)}</span>
      </div>
      <div class="trip-card">
        <div>
          <div class="trip-route-label">${formatDate(trip.started_at)}</div>
          <div class="trip-stats">
            <div class="trip-stat"><span class="trip-stat-val">${formatDistance(trip.distance_m)}</span><span class="trip-stat-label">Distance</span></div>
            <div class="trip-stat"><span class="trip-stat-val">${formatDuration(trip.duration_s)}</span><span class="trip-stat-label">Duration</span></div>
            <div class="trip-stat"><span class="trip-stat-val">${formatTime(trip.started_at)}</span><span class="trip-stat-label">Start</span></div>
            <div class="trip-stat"><span class="trip-stat-val">${formatTime(trip.ended_at)}</span><span class="trip-stat-label">End</span></div>
          </div>
          <div style="margin-top:0.75rem">
            <a href="/trips/${trip.id}" class="btn btn-secondary btn-sm">View details</a>
          </div>
        </div>
        <div class="trip-mini-map" id="recent-trip-map"></div>
      </div>
    `;
    // Render mini map
    setTimeout(() => {
      if (trip.start_lat && trip.start_lon) {
        const map = createMap('recent-trip-map', { zoomControl: false, attributionControl: false });
        if (map) {
          const bounds = [];
          if (trip.start_lat && trip.start_lon) bounds.push([trip.start_lat, trip.start_lon]);
          if (trip.end_lat && trip.end_lon) bounds.push([trip.end_lat, trip.end_lon]);
          if (bounds.length === 2) {
            map.fitBounds(bounds, { padding: [20, 20] });
            L.circleMarker(bounds[0], { radius: 5, color: '#4ecca3', fillOpacity: 1 }).addTo(map);
            L.circleMarker(bounds[1], { radius: 5, color: '#e94560', fillOpacity: 1 }).addTo(map);
            L.polyline(bounds, { color: '#e94560', weight: 2, opacity: 0.5 }).addTo(map);
          } else if (bounds.length === 1) {
            map.setView(bounds[0], 14);
            L.circleMarker(bounds[0], { radius: 5, color: '#4ecca3', fillOpacity: 1 }).addTo(map);
          }
        }
      }
    }, 50);
  } else {
    recentEl.innerHTML = `
      <div class="card-header"><span class="card-title">Most Recent Trip</span></div>
      <div class="empty-state">
        <div class="empty-state-title">No trips yet</div>
        <div class="empty-state-text">Trips will appear here once your device uploads data.</div>
      </div>
    `;
  }

  // Last parked location
  const parkedInfo = document.getElementById('parked-info');
  if (tripsData.status === 'fulfilled' && tripsData.value?.trips?.length > 0) {
    const trip = tripsData.value.trips[0];
    const lat = trip.end_lat || trip.start_lat;
    const lon = trip.end_lon || trip.start_lon;
    if (lat && lon) {
      parkedInfo.innerHTML = `
        <span style="font-size:0.85rem;color:var(--text-dim)">${lat.toFixed(5)}, ${lon.toFixed(5)}</span>
        <br><span style="font-size:0.8rem;color:var(--text-muted)">${relativeTime(trip.ended_at)}</span>
      `;
      setTimeout(() => {
        const map = createMap('parked-map', { zoomControl: false, attributionControl: false });
        if (map) {
          map.setView([lat, lon], 15);
          L.circleMarker([lat, lon], { radius: 7, color: '#e94560', fillColor: '#e94560', fillOpacity: 0.8 }).addTo(map);
        }
      }, 100);
    } else {
      parkedInfo.innerHTML = '<span style="color:var(--text-dim)">No GPS data available</span>';
    }
  } else {
    parkedInfo.innerHTML = '<span style="color:var(--text-dim)">No trips recorded</span>';
  }

  // Devices
  const deviceListEl = document.getElementById('device-list-mini');
  if (devicesData.status === 'fulfilled' && devicesData.value?.devices?.length > 0) {
    deviceListEl.innerHTML = devicesData.value.devices.map(d => `
      <div style="display:flex;align-items:center;gap:0.5rem;padding:0.4rem 0;border-bottom:1px solid var(--border)">
        <span class="status-dot ${deviceStatusColor(d.last_seen_at)}"></span>
        <span style="font-family:var(--font-mono);font-size:0.85rem;flex:1">${escapeHtml(String(d.id))}</span>
        <span style="font-size:0.8rem;color:var(--text-dim)">${relativeTime(d.last_seen_at)}</span>
      </div>
    `).join('');
  } else {
    deviceListEl.innerHTML = '<span style="color:var(--text-dim)">No devices found</span>';
  }
}

// === Trips View ===

let tripsState = { offset: 0, limit: 20, trips: [], total: 0, filters: {} };

async function viewTrips() {
  tripsState = { offset: 0, limit: 20, trips: [], total: 0, filters: {} };
  const app = document.getElementById('app');
  app.innerHTML = `
    <div class="page-header">
      <h1 class="page-title">Trips</h1>
      <p class="page-subtitle">Your driving history</p>
    </div>
    <div class="filters" id="trip-filters">
      <input type="date" id="filter-from" placeholder="From date">
      <input type="date" id="filter-to" placeholder="To date">
      <input type="text" id="filter-device" placeholder="Device ID" style="max-width:140px">
      <input type="text" id="filter-tag" placeholder="Tag" style="max-width:120px">
      <button class="btn btn-secondary btn-sm" id="filter-apply">Filter</button>
      <button class="btn btn-ghost btn-sm" id="filter-clear">Clear</button>
    </div>
    <div id="trips-list">
      ${Array(3).fill(0).map(() => `
        <div class="card" style="margin-bottom:0.75rem">
          ${skeleton('text', 'medium')}
          ${skeleton('text', 'short')}
          ${skeleton('text')}
        </div>
      `).join('')}
    </div>
    <div class="load-more" id="load-more" style="display:none">
      <button class="btn btn-secondary" id="load-more-btn">Load more</button>
    </div>
  `;

  document.getElementById('filter-apply').onclick = () => applyTripFilters();
  document.getElementById('filter-clear').onclick = () => clearTripFilters();
  document.getElementById('load-more-btn').onclick = () => loadMoreTrips();

  // Check for query params (device filter from device view)
  const urlParams = new URLSearchParams(window.location.search);
  if (urlParams.get('device_id')) {
    document.getElementById('filter-device').value = urlParams.get('device_id');
    tripsState.filters.device_id = urlParams.get('device_id');
  }

  await loadTrips();
}

async function applyTripFilters() {
  const from = document.getElementById('filter-from').value;
  const to = document.getElementById('filter-to').value;
  const device = document.getElementById('filter-device').value.trim();
  const tag = document.getElementById('filter-tag').value.trim();
  tripsState.filters = {};
  if (from) tripsState.filters.from = new Date(from).toISOString();
  if (to) tripsState.filters.to = new Date(to + 'T23:59:59').toISOString();
  if (device) tripsState.filters.device_id = device;
  if (tag) tripsState.filters.tag = tag;
  tripsState.offset = 0;
  tripsState.trips = [];
  await loadTrips();
}

function clearTripFilters() {
  document.getElementById('filter-from').value = '';
  document.getElementById('filter-to').value = '';
  document.getElementById('filter-device').value = '';
  document.getElementById('filter-tag').value = '';
  tripsState.filters = {};
  tripsState.offset = 0;
  tripsState.trips = [];
  loadTrips();
}

async function loadTrips() {
  try {
    const data = await API.trips({
      limit: tripsState.limit,
      offset: tripsState.offset,
      ...tripsState.filters,
    });
    const trips = data.trips || [];
    tripsState.total = data.total || 0;
    tripsState.trips = tripsState.offset === 0 ? trips : [...tripsState.trips, ...trips];
    renderTripsList();
  } catch {
    document.getElementById('trips-list').innerHTML = `
      <div class="empty-state">
        <div class="empty-state-title">Could not load trips</div>
        <div class="empty-state-text">Check that the backend is running.</div>
      </div>
    `;
  }
}

async function loadMoreTrips() {
  tripsState.offset += tripsState.limit;
  await loadTrips();
}

function renderTripsList() {
  const listEl = document.getElementById('trips-list');
  const loadMoreEl = document.getElementById('load-more');

  if (tripsState.trips.length === 0) {
    listEl.innerHTML = `
      <div class="empty-state">
        <div class="empty-state-icon">
          <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M9 20l-5.447-2.724A1 1 0 013 16.382V5.618a1 1 0 011.447-.894L9 7m0 13l6-3m-6 3V7m6 10l5.447 2.724A1 1 0 0021 18.382V7.618a1 1 0 00-.553-.894L15 4m0 13V4m0 0L9 7"/></svg>
        </div>
        <div class="empty-state-title">No trips found</div>
        <div class="empty-state-text">Try adjusting your filters or wait for new trip uploads.</div>
      </div>
    `;
    loadMoreEl.style.display = 'none';
    return;
  }

  listEl.innerHTML = tripsState.trips.map((trip, i) => `
    <a href="/trips/${trip.id}" class="card card-clickable" style="display:block;text-decoration:none;color:inherit;margin-bottom:0.75rem">
      <div class="trip-card">
        <div>
          <div class="card-header" style="margin-bottom:0.35rem">
            <span class="trip-route-label">${formatDate(trip.started_at)}</span>
            <span class="card-meta">${formatTime(trip.started_at)} - ${formatTime(trip.ended_at)}</span>
          </div>
          <div class="trip-stats">
            <div class="trip-stat"><span class="trip-stat-val">${formatDistance(trip.distance_m)}</span><span class="trip-stat-label">Distance</span></div>
            <div class="trip-stat"><span class="trip-stat-val">${formatDuration(trip.duration_s)}</span><span class="trip-stat-label">Duration</span></div>
          </div>
          ${trip.tags && trip.tags.length > 0 ? `
            <div class="tags-list" style="margin-top:0.5rem">
              ${trip.tags.map(t => `<span class="tag">${escapeHtml(t)}</span>`).join('')}
            </div>
          ` : ''}
        </div>
        <div class="trip-mini-map" id="trip-map-${i}"></div>
      </div>
    </a>
  `).join('');

  // Show/hide load more
  if (tripsState.trips.length < tripsState.total) {
    loadMoreEl.style.display = 'flex';
  } else {
    loadMoreEl.style.display = 'none';
  }

  // Render mini maps
  setTimeout(() => {
    tripsState.trips.forEach((trip, i) => {
      const mapId = `trip-map-${i}`;
      const el = document.getElementById(mapId);
      if (!el || mapInstances.has(mapId)) return;
      if (trip.start_lat && trip.start_lon) {
        const map = createMap(mapId, { zoomControl: false, attributionControl: false, dragging: false, scrollWheelZoom: false });
        if (map) {
          const bounds = [];
          if (trip.start_lat && trip.start_lon) bounds.push([trip.start_lat, trip.start_lon]);
          if (trip.end_lat && trip.end_lon) bounds.push([trip.end_lat, trip.end_lon]);
          if (bounds.length === 2) {
            map.fitBounds(bounds, { padding: [15, 15] });
            L.circleMarker(bounds[0], { radius: 4, color: '#4ecca3', fillOpacity: 1 }).addTo(map);
            L.circleMarker(bounds[1], { radius: 4, color: '#e94560', fillOpacity: 1 }).addTo(map);
            L.polyline(bounds, { color: '#e94560', weight: 2, opacity: 0.4, dashArray: '5,5' }).addTo(map);
          } else if (bounds.length === 1) {
            map.setView(bounds[0], 14);
            L.circleMarker(bounds[0], { radius: 4, color: '#4ecca3', fillOpacity: 1 }).addTo(map);
          }
        }
      }
    });
  }, 100);
}

// === Trip Detail View ===

async function viewTripDetail({ id }) {
  const app = document.getElementById('app');
  app.innerHTML = `
    <div class="page-header" style="display:flex;align-items:center;gap:0.75rem">
      <a href="/trips" class="btn btn-ghost btn-sm" style="padding:0.3rem">
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M15 19l-7-7 7-7"/></svg>
      </a>
      <div>
        <h1 class="page-title">Trip Details</h1>
        <p class="page-subtitle" id="trip-date">${skeleton('text', 'short')}</p>
      </div>
    </div>
    <div class="trip-detail-layout">
      <div>
        <div class="map-full" id="trip-detail-map"></div>
        <div class="speed-legend">
          <span>Slow</span>
          <div class="speed-legend-bar"></div>
          <span>Fast</span>
        </div>
        <div class="card" style="margin-top:1rem" id="trip-events-card">
          <div class="card-header"><span class="card-title">Events</span></div>
          <div class="card-body" id="trip-events">
            ${skeleton('text')}${skeleton('text', 'medium')}${skeleton('text', 'short')}
          </div>
        </div>
      </div>
      <div class="trip-detail-sidebar">
        <div class="card" id="trip-info-card">
          <div class="card-header"><span class="card-title">Info</span></div>
          <div class="card-body" id="trip-info">
            ${skeleton('text')}${skeleton('text')}${skeleton('text')}${skeleton('text')}
          </div>
        </div>
        <div class="card" id="trip-tags-card">
          <div class="card-header"><span class="card-title">Tags</span></div>
          <div class="card-body" id="trip-tags">
            ${skeleton('text', 'short')}
          </div>
        </div>
        <div class="card">
          <div class="card-header"><span class="card-title">Export</span></div>
          <div class="card-body">
            <div class="btn-group">
              <a href="/api/v1/trips/${id}/export/gpx" class="btn btn-secondary btn-sm" download>GPX</a>
              <a href="/api/v1/trips/${id}/export/geojson" class="btn btn-secondary btn-sm" download>GeoJSON</a>
              <a href="/api/v1/trips/${id}/export/csv" class="btn btn-secondary btn-sm" download>CSV</a>
            </div>
          </div>
        </div>
        <div class="card">
          <div class="card-body">
            <button class="btn btn-danger" id="delete-trip-btn" style="width:100%">Delete Trip</button>
          </div>
        </div>
      </div>
    </div>
  `;

  document.getElementById('delete-trip-btn').onclick = async () => {
    const yes = await confirm('Delete Trip', 'Are you sure you want to permanently delete this trip? This cannot be undone.');
    if (yes) {
      try {
        await API.deleteTrip(id);
        showToast('Trip deleted');
        Router.navigate('/trips');
      } catch {
        showToast('Failed to delete trip');
      }
    }
  };

  // Load trip data, route, and events concurrently
  const [tripData, routeData, eventsData] = await Promise.allSettled([
    API.trip(id),
    API.tripRoute(id),
    API.tripEvents(id),
  ]);

  // Trip info
  if (tripData.status === 'fulfilled' && tripData.value) {
    const trip = tripData.value;
    document.getElementById('trip-date').textContent = formatDateTime(trip.started_at);

    document.getElementById('trip-info').innerHTML = `
      <div class="info-table">
        <div class="info-row"><span class="info-label">Start</span><span class="info-value">${formatDateTime(trip.started_at)}</span></div>
        <div class="info-row"><span class="info-label">End</span><span class="info-value">${formatDateTime(trip.ended_at)}</span></div>
        <div class="info-row"><span class="info-label">Distance</span><span class="info-value">${formatDistance(trip.distance_m)}</span></div>
        <div class="info-row"><span class="info-label">Duration</span><span class="info-value">${formatDuration(trip.duration_s)}</span></div>
        <div class="info-row"><span class="info-label">Device</span><span class="info-value" style="font-family:var(--font-mono)">${escapeHtml(String(trip.device_id))}</span></div>
      </div>
    `;

    // Tags
    renderTripTags(id, trip.tags || []);
  }

  // Map + route
  setTimeout(() => {
    const map = createMap('trip-detail-map');
    if (!map) return;

    if (routeData.status === 'fulfilled' && routeData.value) {
      const geojson = routeData.value;
      if (geojson.features && geojson.features.length > 0) {
        const feature = geojson.features[0];
        const coords = feature.geometry?.coordinates || [];
        const speeds = feature.properties?.speed || [];

        if (coords.length > 0) {
          // Draw route segments colored by speed
          const maxSpeed = Math.max(...speeds.filter(s => s > 0), 60);
          const points = coords.map((c, i) => ({
            lat: c[1], lng: c[0],
            speed: speeds[i] || 0,
          }));

          for (let i = 1; i < points.length; i++) {
            const speedRatio = Math.min(points[i].speed / maxSpeed, 1);
            const color = speedColor(speedRatio);
            L.polyline(
              [[points[i-1].lat, points[i-1].lng], [points[i].lat, points[i].lng]],
              { color, weight: 4, opacity: 0.85 }
            ).addTo(map);
          }

          // Start/end markers
          const start = points[0];
          const end = points[points.length - 1];
          L.circleMarker([start.lat, start.lng], { radius: 7, color: '#4ecca3', fillColor: '#4ecca3', fillOpacity: 1 })
            .bindPopup('Start').addTo(map);
          L.circleMarker([end.lat, end.lng], { radius: 7, color: '#e94560', fillColor: '#e94560', fillOpacity: 1 })
            .bindPopup('End').addTo(map);

          // Fit bounds
          const latlngs = points.map(p => [p.lat, p.lng]);
          map.fitBounds(latlngs, { padding: [30, 30] });
        }
      }
    } else if (tripData.status === 'fulfilled' && tripData.value) {
      // Fallback: just show start/end
      const trip = tripData.value;
      const bounds = [];
      if (trip.start_lat && trip.start_lon) {
        bounds.push([trip.start_lat, trip.start_lon]);
        L.circleMarker([trip.start_lat, trip.start_lon], { radius: 7, color: '#4ecca3', fillOpacity: 1 }).addTo(map);
      }
      if (trip.end_lat && trip.end_lon) {
        bounds.push([trip.end_lat, trip.end_lon]);
        L.circleMarker([trip.end_lat, trip.end_lon], { radius: 7, color: '#e94560', fillOpacity: 1 }).addTo(map);
      }
      if (bounds.length === 2) {
        map.fitBounds(bounds, { padding: [40, 40] });
      } else if (bounds.length === 1) {
        map.setView(bounds[0], 14);
      } else {
        map.setView([0, 0], 2);
      }
    } else {
      map.setView([0, 0], 2);
    }
  }, 50);

  // Events timeline
  const eventsEl = document.getElementById('trip-events');
  if (eventsData.status === 'fulfilled' && eventsData.value?.events?.length > 0) {
    const events = eventsData.value.events;
    eventsEl.innerHTML = `
      <div class="timeline">
        ${events.map(e => `
          <div class="timeline-item">
            <div class="timeline-time">${formatTime(e.timestamp_at)}</div>
            <div class="timeline-label">${formatEventType(e.event_type)}</div>
          </div>
        `).join('')}
      </div>
    `;
  } else {
    eventsEl.innerHTML = '<span style="color:var(--text-dim)">No events recorded</span>';
  }
}

function speedColor(ratio) {
  // 0 = green (#4ecca3), 0.5 = yellow (#f0a500), 1 = red (#e94560)
  if (ratio <= 0.5) {
    const t = ratio * 2;
    const r = Math.round(78 + t * (240 - 78));
    const g = Math.round(204 + t * (165 - 204));
    const b = Math.round(163 + t * (0 - 163));
    return `rgb(${r},${g},${b})`;
  } else {
    const t = (ratio - 0.5) * 2;
    const r = Math.round(240 + t * (233 - 240));
    const g = Math.round(165 + t * (69 - 165));
    const b = Math.round(0 + t * (96 - 0));
    return `rgb(${r},${g},${b})`;
  }
}

function formatEventType(type) {
  if (!type) return 'Unknown';
  return type.replace(/_/g, ' ').replace(/\b\w/g, c => c.toUpperCase());
}

function renderTripTags(tripId, tags) {
  const el = document.getElementById('trip-tags');
  el.innerHTML = `
    <div class="tags-list" style="margin-bottom:0.5rem">
      ${tags.length > 0
        ? tags.map(t => `
            <span class="tag">
              ${escapeHtml(t)}
              <span class="tag-remove" data-tag="${escapeHtml(t)}">&times;</span>
            </span>
          `).join('')
        : '<span style="color:var(--text-dim);font-size:0.85rem">No tags</span>'
      }
    </div>
    <div style="display:flex;gap:0.35rem">
      <input type="text" id="new-tag-input" placeholder="Add tag..." style="flex:1;font-size:0.8rem;padding:0.35rem 0.5rem">
      <button class="btn btn-secondary btn-sm" id="add-tag-btn">Add</button>
    </div>
  `;

  // Remove tag handlers
  el.querySelectorAll('.tag-remove').forEach(btn => {
    btn.onclick = async (e) => {
      e.stopPropagation();
      const tag = btn.dataset.tag;
      try {
        await API.removeTag(tripId, tag);
        const newTags = tags.filter(t => t !== tag);
        renderTripTags(tripId, newTags);
        showToast(`Removed tag "${tag}"`);
      } catch {
        showToast('Failed to remove tag');
      }
    };
  });

  // Add tag handler
  const addBtn = document.getElementById('add-tag-btn');
  const input = document.getElementById('new-tag-input');
  const doAdd = async () => {
    const tag = input.value.trim();
    if (!tag) return;
    try {
      await API.addTag(tripId, tag);
      tags.push(tag);
      renderTripTags(tripId, tags);
      showToast(`Added tag "${tag}"`);
    } catch {
      showToast('Failed to add tag');
    }
  };
  addBtn.onclick = doAdd;
  input.onkeydown = (e) => { if (e.key === 'Enter') doAdd(); };
}

// === Places View ===

async function viewPlaces() {
  const app = document.getElementById('app');
  app.innerHTML = `
    <div class="page-header">
      <h1 class="page-title">Places</h1>
      <p class="page-subtitle">Saved locations and geofences</p>
    </div>
    <div class="dashboard-grid">
      <div>
        <div class="map-full" id="places-map" style="height:350px;margin-bottom:1rem"></div>
        <div class="card" id="add-place-card">
          <div class="card-header"><span class="card-title">Add Place</span></div>
          <div class="card-body">
            <p style="font-size:0.85rem;color:var(--text-dim);margin-bottom:0.75rem">Click on the map to set the location, then fill in the details below.</p>
            <div class="form-row">
              <div class="form-group">
                <label class="form-label">Name</label>
                <input type="text" class="form-input" id="place-name" placeholder="Home, Work, etc.">
              </div>
              <div class="form-group">
                <label class="form-label">Radius (m)</label>
                <input type="number" class="form-input" id="place-radius" value="100" min="10" max="5000">
              </div>
            </div>
            <div class="form-row" style="margin-bottom:0.5rem">
              <div class="form-group">
                <label class="form-label">Latitude</label>
                <input type="number" class="form-input" id="place-lat" step="0.00001" placeholder="Click map">
              </div>
              <div class="form-group">
                <label class="form-label">Longitude</label>
                <input type="number" class="form-input" id="place-lon" step="0.00001" placeholder="Click map">
              </div>
            </div>
            <button class="btn btn-primary" id="save-place-btn">Save Place</button>
          </div>
        </div>
      </div>
      <div>
        <div id="places-list">
          ${skeleton('text')}${skeleton('text')}${skeleton('text')}
        </div>
      </div>
    </div>
  `;

  let placesData = [];
  let placeMarker = null;
  let placeCircle = null;

  // Initialize map
  setTimeout(async () => {
    const map = createMap('places-map');
    if (!map) return;
    map.setView([37.7749, -122.4194], 10); // Default view

    // Map click to set place
    map.on('click', (e) => {
      document.getElementById('place-lat').value = e.latlng.lat.toFixed(5);
      document.getElementById('place-lon').value = e.latlng.lng.toFixed(5);
      const radius = parseInt(document.getElementById('place-radius').value) || 100;

      if (placeMarker) map.removeLayer(placeMarker);
      if (placeCircle) map.removeLayer(placeCircle);
      placeMarker = L.circleMarker(e.latlng, { radius: 6, color: '#e94560', fillColor: '#e94560', fillOpacity: 1 }).addTo(map);
      placeCircle = L.circle(e.latlng, { radius, color: '#e94560', fillColor: '#e94560', fillOpacity: 0.1, weight: 1 }).addTo(map);
    });

    // Load existing places
    try {
      const data = await API.places();
      placesData = data.places || [];
      renderPlacesList(placesData);

      // Add places to map
      const bounds = [];
      placesData.forEach(p => {
        if (p.lat && p.lon) {
          L.circle([p.lat, p.lon], {
            radius: p.radius_m || 100,
            color: '#0f3460',
            fillColor: '#0f3460',
            fillOpacity: 0.15,
            weight: 2,
          }).bindPopup(`<b>${escapeHtml(p.name)}</b><br>${p.visit_count || 0} visits`).addTo(map);
          L.circleMarker([p.lat, p.lon], { radius: 5, color: '#4ecca3', fillColor: '#4ecca3', fillOpacity: 0.8 }).addTo(map);
          bounds.push([p.lat, p.lon]);
        }
      });
      if (bounds.length > 0) {
        map.fitBounds(bounds, { padding: [40, 40], maxZoom: 14 });
      }
    } catch {
      renderPlacesList([]);
    }
  }, 50);

  // Save place handler
  document.getElementById('save-place-btn').onclick = async () => {
    const name = document.getElementById('place-name').value.trim();
    const lat = parseFloat(document.getElementById('place-lat').value);
    const lon = parseFloat(document.getElementById('place-lon').value);
    const radius_m = parseInt(document.getElementById('place-radius').value) || 100;

    if (!name) { showToast('Please enter a name'); return; }
    if (isNaN(lat) || isNaN(lon)) { showToast('Please click the map to set a location'); return; }

    try {
      await API.createPlace({ name, lat, lon, radius_m });
      showToast(`Place "${name}" saved`);
      Router.navigate('/places'); // Reload
    } catch {
      showToast('Failed to save place');
    }
  };
}

function renderPlacesList(places) {
  const el = document.getElementById('places-list');
  if (places.length === 0) {
    el.innerHTML = `
      <div class="empty-state">
        <div class="empty-state-title">No places saved</div>
        <div class="empty-state-text">Click the map and add your first place.</div>
      </div>
    `;
    return;
  }

  el.innerHTML = places.map(p => `
    <div class="card place-card" style="margin-bottom:0.75rem">
      <div class="place-info">
        <div class="place-name">${escapeHtml(p.name)}</div>
        <div class="place-detail">${p.lat?.toFixed(4)}, ${p.lon?.toFixed(4)} &middot; ${p.radius_m || 100}m radius &middot; ${p.visit_count || 0} visits</div>
      </div>
      <div class="place-actions">
        <button class="btn btn-ghost btn-sm" data-edit-place="${p.id}" title="Edit">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>
        </button>
        <button class="btn btn-ghost btn-sm" data-delete-place="${p.id}" data-place-name="${escapeHtml(p.name)}" title="Delete" style="color:var(--danger)">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 6h18M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a2 2 0 012-2h4a2 2 0 012 2v2"/></svg>
        </button>
      </div>
    </div>
  `).join('');

  // Delete handlers
  el.querySelectorAll('[data-delete-place]').forEach(btn => {
    btn.onclick = async () => {
      const id = btn.dataset.deletePlace;
      const name = btn.dataset.placeName;
      const yes = await confirm('Delete Place', `Delete "${name}"? This cannot be undone.`);
      if (yes) {
        try {
          await API.deletePlace(id);
          showToast(`Deleted "${name}"`);
          Router.navigate('/places');
        } catch {
          showToast('Failed to delete place');
        }
      }
    };
  });

  // Edit handlers (simple: navigate to edit inline)
  el.querySelectorAll('[data-edit-place]').forEach(btn => {
    btn.onclick = () => {
      const id = btn.dataset.editPlace;
      const place = places.find(p => p.id === id);
      if (place) {
        document.getElementById('place-name').value = place.name;
        document.getElementById('place-lat').value = place.lat;
        document.getElementById('place-lon').value = place.lon;
        document.getElementById('place-radius').value = place.radius_m || 100;
        // Switch save button to update
        const saveBtn = document.getElementById('save-place-btn');
        saveBtn.textContent = 'Update Place';
        saveBtn.onclick = async () => {
          const name = document.getElementById('place-name').value.trim();
          const lat = parseFloat(document.getElementById('place-lat').value);
          const lon = parseFloat(document.getElementById('place-lon').value);
          const radius_m = parseInt(document.getElementById('place-radius').value) || 100;
          if (!name) { showToast('Please enter a name'); return; }
          try {
            await API.updatePlace(id, { name, lat, lon, radius_m });
            showToast(`Updated "${name}"`);
            Router.navigate('/places');
          } catch {
            showToast('Failed to update place');
          }
        };
        document.getElementById('place-name').focus();
      }
    };
  });
}

// === Devices View ===

async function viewDevices() {
  const app = document.getElementById('app');
  app.innerHTML = `
    <div class="page-header">
      <h1 class="page-title">Devices</h1>
      <p class="page-subtitle">Connected Freematics devices</p>
    </div>
    <div id="devices-list">
      ${Array(2).fill(0).map(() => `
        <div class="card" style="margin-bottom:0.75rem">
          ${skeleton('text', 'medium')}
          ${skeleton('text', 'short')}
        </div>
      `).join('')}
    </div>
  `;

  try {
    const data = await API.devices();
    const devices = data.devices || [];
    const listEl = document.getElementById('devices-list');

    if (devices.length === 0) {
      listEl.innerHTML = `
        <div class="empty-state">
          <div class="empty-state-icon">
            <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z"/></svg>
          </div>
          <div class="empty-state-title">No devices found</div>
          <div class="empty-state-text">Devices will appear here once they connect and upload data.</div>
        </div>
      `;
      return;
    }

    listEl.innerHTML = devices.map(d => `
      <div class="card device-card" style="margin-bottom:0.75rem">
        <div class="device-info">
          <div style="display:flex;align-items:center;gap:0.5rem">
            <span class="status-dot ${deviceStatusColor(d.last_seen_at)}"></span>
            <span class="device-id">${escapeHtml(String(d.id))}</span>
          </div>
          <div class="device-detail">
            ${d.firmware_version ? `Firmware: ${escapeHtml(d.firmware_version)} &middot; ` : ''}
            Last seen: ${relativeTime(d.last_seen_at)}
          </div>
        </div>
        <div class="device-stats">
          <div class="trip-stat">
            <span class="trip-stat-val">${d.trip_count ?? '--'}</span>
            <span class="trip-stat-label">Trips</span>
          </div>
          <div class="trip-stat">
            <span class="trip-stat-val">${formatDistance(d.total_distance_m)}</span>
            <span class="trip-stat-label">Distance</span>
          </div>
        </div>
        <a href="/trips?device_id=${encodeURIComponent(d.id)}" class="btn btn-secondary btn-sm">View Trips</a>
      </div>
    `).join('');
  } catch {
    document.getElementById('devices-list').innerHTML = `
      <div class="empty-state">
        <div class="empty-state-title">Could not load devices</div>
        <div class="empty-state-text">Check that the backend is running.</div>
      </div>
    `;
  }
}

// === Settings / Privacy View ===

async function viewSettings() {
  const app = document.getElementById('app');
  app.innerHTML = `
    <div class="page-header">
      <h1 class="page-title">Privacy & Data</h1>
      <p class="page-subtitle">Manage your driving data</p>
    </div>

    <div class="settings-section">
      <h2 class="settings-section-title">Data Summary</h2>
      <div class="stat-grid" id="data-stats">
        <div class="stat-card">${skeleton('stat')}<div class="stat-label">Total trips</div></div>
        <div class="stat-card">${skeleton('stat')}<div class="stat-label">Total distance</div></div>
        <div class="stat-card">${skeleton('stat')}<div class="stat-label">Total driving time</div></div>
        <div class="stat-card">${skeleton('stat')}<div class="stat-label">Devices</div></div>
      </div>
    </div>

    <div class="settings-section">
      <h2 class="settings-section-title">Export</h2>
      <div class="card">
        <div class="card-body">
          <p style="font-size:0.85rem;color:var(--text-dim);margin-bottom:0.75rem">
            Download all your trip data as a single GeoJSON file. This may take a moment depending on how many trips you have.
          </p>
          <button class="btn btn-primary" id="export-all-btn">Export All Trips (GeoJSON)</button>
        </div>
      </div>
    </div>

    <div class="settings-section">
      <h2 class="settings-section-title">Manage Trips</h2>
      <div class="card">
        <div class="card-body">
          <p style="font-size:0.85rem;color:var(--text-dim);margin-bottom:0.75rem">
            Browse and delete individual trips from the <a href="/trips">Trips</a> view. Each trip detail page has a delete button.
          </p>
        </div>
      </div>
    </div>

    <div class="settings-section">
      <h2 class="settings-section-title">Bulk Delete</h2>
      <div class="card">
        <div class="card-body">
          <p style="font-size:0.85rem;color:var(--text-dim);margin-bottom:0.75rem">
            Delete all trips before a specific date. This action is irreversible.
          </p>
          <div style="display:flex;gap:0.5rem;align-items:center;flex-wrap:wrap">
            <span style="font-size:0.85rem">Delete all trips before:</span>
            <input type="date" id="bulk-delete-date">
            <button class="btn btn-danger btn-sm" id="bulk-delete-btn">Delete</button>
          </div>
        </div>
      </div>
    </div>

    <div class="settings-section">
      <h2 class="settings-section-title">About Cairn</h2>
      <div class="card">
        <div class="card-body" style="font-size:0.85rem;color:var(--text-dim);line-height:1.6">
          Cairn is an offline-first car journal. Trip data is collected by a Freematics ONE+ device,
          uploaded to your homelab, and stored in PostgreSQL with PostGIS. All data stays on your infrastructure.
          <br><br>
          <span style="color:var(--text-muted)">No cloud. No tracking. Your data, your roads.</span>
        </div>
      </div>
    </div>
  `;

  // Load stats
  try {
    const stats = await API.stats();
    document.getElementById('data-stats').innerHTML = `
      <div class="stat-card"><div class="stat-value">${stats.total_trips ?? '--'}</div><div class="stat-label">Total trips</div></div>
      <div class="stat-card"><div class="stat-value">${formatDistance(stats.total_distance_m)}</div><div class="stat-label">Total distance</div></div>
      <div class="stat-card"><div class="stat-value">${formatDuration(stats.total_duration_s)}</div><div class="stat-label">Total driving time</div></div>
      <div class="stat-card"><div class="stat-value">${stats.device_count ?? '--'}</div><div class="stat-label">Devices</div></div>
    `;
  } catch {
    // Leave skeleton
  }

  // Export all
  document.getElementById('export-all-btn').onclick = async () => {
    showToast('Preparing export...');
    try {
      // Fetch all trips then download each as GeoJSON
      const allTrips = [];
      let offset = 0;
      const limit = 100;
      while (true) {
        const data = await API.trips({ limit, offset });
        const trips = data.trips || [];
        allTrips.push(...trips);
        if (trips.length < limit) break;
        offset += limit;
      }
      // Create a combined GeoJSON
      const features = allTrips.map(t => ({
        type: 'Feature',
        properties: {
          id: t.id,
          device_id: t.device_id,
          started_at: t.started_at,
          ended_at: t.ended_at,
          distance_m: t.distance_m,
          duration_s: t.duration_s,
          tags: t.tags,
        },
        geometry: {
          type: 'LineString',
          coordinates: [
            [t.start_lon, t.start_lat],
            [t.end_lon, t.end_lat],
          ].filter(c => c[0] && c[1]),
        },
      }));
      const geojson = { type: 'FeatureCollection', features };
      const blob = new Blob([JSON.stringify(geojson, null, 2)], { type: 'application/json' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `cairn-export-${new Date().toISOString().split('T')[0]}.geojson`;
      a.click();
      URL.revokeObjectURL(url);
      showToast(`Exported ${allTrips.length} trips`);
    } catch {
      showToast('Export failed');
    }
  };

  // Bulk delete
  document.getElementById('bulk-delete-btn').onclick = async () => {
    const dateVal = document.getElementById('bulk-delete-date').value;
    if (!dateVal) { showToast('Please select a date'); return; }
    const yes = await confirm('Bulk Delete', `Delete ALL trips before ${dateVal}? This cannot be undone.`);
    if (!yes) return;
    showToast('Deleting trips...');
    try {
      const data = await API.trips({ limit: 1000, to: new Date(dateVal).toISOString() });
      const trips = data.trips || [];
      let deleted = 0;
      for (const trip of trips) {
        try {
          await API.deleteTrip(trip.id);
          deleted++;
        } catch { /* continue */ }
      }
      showToast(`Deleted ${deleted} trips`);
    } catch {
      showToast('Bulk delete failed');
    }
  };
}

// ─── Register Routes ──────────────────────────────────────────

Router.add('/', viewToday);
Router.add('/trips', viewTrips);
Router.add('/trips/:id', viewTripDetail);
Router.add('/places', viewPlaces);
Router.add('/devices', viewDevices);
Router.add('/settings', viewSettings);

// ─── Service Worker Registration ──────────────────────────────

if ('serviceWorker' in navigator) {
  navigator.serviceWorker.register('/sw.js').catch(() => {
    // SW registration failed silently
  });
}

// ─── Init ─────────────────────────────────────────────────────

Router.init();
