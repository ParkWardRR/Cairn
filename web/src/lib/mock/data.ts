const SF_LOCATIONS: [number, number, string][] = [
  [37.7749, -122.4194, 'Downtown SF'],
  [37.7849, -122.4094, 'Nob Hill'],
  [37.7599, -122.4269, 'Mission District'],
  [37.8044, -122.2712, 'Oakland'],
  [37.8716, -122.2727, 'Berkeley'],
  [37.5585, -122.2711, 'Fremont'],
  [37.3382, -121.8863, 'San Jose'],
  [37.4419, -122.1430, 'Palo Alto'],
  [37.4849, -122.2294, 'Redwood City'],
  [37.6879, -122.4702, 'Daly City'],
  [37.5480, -122.0576, 'Milpitas'],
  [37.3861, -122.0839, 'Cupertino'],
  [37.5025, -122.2531, 'San Mateo'],
  [37.6547, -122.4077, 'South SF'],
  [37.7559, -122.4529, 'Golden Gate Park'],
  [37.8199, -122.4783, 'Golden Gate Bridge'],
  [37.7956, -122.3933, 'Embarcadero'],
  [37.7694, -122.4862, 'Outer Sunset'],
  [37.6213, -122.3790, 'SFO Airport'],
  [37.7219, -122.4782, 'Ocean Beach'],
];

const TAG_SETS = [
  ['commute'],
  ['commute', 'highway'],
  ['errands'],
  ['weekend'],
  ['night-drive'],
  ['road-trip'],
  ['grocery'],
  ['commute', 'morning'],
  ['commute', 'evening'],
  ['weekend', 'scenic'],
  ['appointment'],
  [],
  [],
  [],
];

function seededRandom(seed: number): () => number {
  let s = seed;
  return () => {
    s = (s * 1664525 + 1013904223) & 0xffffffff;
    return (s >>> 0) / 0xffffffff;
  };
}

function generateTrips(count: number) {
  const rand = seededRandom(42);
  const trips: any[] = [];
  const now = new Date();

  for (let i = 0; i < count; i++) {
    const daysAgo = Math.floor(rand() * 90);
    const hour = rand() < 0.4 ? 7 + Math.floor(rand() * 3) : rand() < 0.7 ? 17 + Math.floor(rand() * 3) : 10 + Math.floor(rand() * 8);
    const minute = Math.floor(rand() * 60);

    const startDate = new Date(now);
    startDate.setDate(startDate.getDate() - daysAgo);
    startDate.setHours(hour, minute, 0, 0);

    const durationMin = 8 + Math.floor(rand() * 55);
    const endDate = new Date(startDate.getTime() + durationMin * 60000);

    const startIdx = Math.floor(rand() * SF_LOCATIONS.length);
    let endIdx = Math.floor(rand() * SF_LOCATIONS.length);
    if (endIdx === startIdx) endIdx = (endIdx + 1) % SF_LOCATIONS.length;

    const [startLat, startLon] = SF_LOCATIONS[startIdx];
    const [endLat, endLon] = SF_LOCATIONS[endIdx];

    const distLat = Math.abs(endLat - startLat) * 111000;
    const distLon = Math.abs(endLon - startLon) * 85000;
    const straightLine = Math.sqrt(distLat * distLat + distLon * distLon);
    const distance = straightLine * (1.2 + rand() * 0.5);

    const tags = TAG_SETS[Math.floor(rand() * TAG_SETS.length)];
    const deviceId = rand() < 0.65 ? 'FMTC-001A' : 'FMTC-002B';

    trips.push({
      id: `trip-${String(i + 1).padStart(4, '0')}`,
      device_id: deviceId,
      started_at: startDate.toISOString(),
      ended_at: endDate.toISOString(),
      start_lat: startLat + (rand() - 0.5) * 0.005,
      start_lon: startLon + (rand() - 0.5) * 0.005,
      end_lat: endLat + (rand() - 0.5) * 0.005,
      end_lon: endLon + (rand() - 0.5) * 0.005,
      distance_m: Math.round(distance),
      duration_s: durationMin * 60,
      tags,
    });
  }

  trips.sort((a, b) => new Date(b.started_at).getTime() - new Date(a.started_at).getTime());
  return trips;
}

const ALL_TRIPS = generateTrips(284);

function isToday(dateStr: string) {
  const d = new Date(dateStr);
  const now = new Date();
  return d.toDateString() === now.toDateString();
}

function isThisWeek(dateStr: string) {
  const d = new Date(dateStr);
  const now = new Date();
  const weekAgo = new Date(now);
  weekAgo.setDate(weekAgo.getDate() - 7);
  return d >= weekAgo;
}

function isThisMonth(dateStr: string) {
  const d = new Date(dateStr);
  const now = new Date();
  return d.getMonth() === now.getMonth() && d.getFullYear() === now.getFullYear();
}

const todayTrips = ALL_TRIPS.filter(t => isToday(t.started_at));
const weekTrips = ALL_TRIPS.filter(t => isThisWeek(t.started_at));
const monthTrips = ALL_TRIPS.filter(t => isThisMonth(t.started_at));

const totalDistance = ALL_TRIPS.reduce((sum, t) => sum + t.distance_m, 0);
const totalDuration = ALL_TRIPS.reduce((sum, t) => sum + t.duration_s, 0);

export const mockStats = {
  total_trips: ALL_TRIPS.length,
  total_distance_m: totalDistance,
  total_duration_s: totalDuration,
  device_count: 2,
  today_trips: todayTrips.length,
  week_trips: weekTrips.length,
  month_trips: monthTrips.length,
  recent_trip: ALL_TRIPS[0] ?? null,
};

export function mockTrips(params: Record<string, any> = {}) {
  let filtered = [...ALL_TRIPS];

  if (params.device_id) filtered = filtered.filter(t => t.device_id === params.device_id);
  if (params.tag) filtered = filtered.filter(t => t.tags?.includes(params.tag));
  if (params.from) filtered = filtered.filter(t => new Date(t.started_at) >= new Date(params.from));
  if (params.to) filtered = filtered.filter(t => new Date(t.started_at) <= new Date(params.to));

  const limit = params.limit ?? 20;
  const offset = params.offset ?? 0;

  return {
    trips: filtered.slice(offset, offset + limit),
    total: filtered.length,
  };
}

export function mockTrip(id: string) {
  return ALL_TRIPS.find(t => t.id === id) ?? null;
}

export function mockTripRoute(id: string) {
  const trip = mockTrip(id);
  if (!trip) return null;

  const rand = seededRandom(parseInt(id.replace(/\D/g, '')) || 1);
  const steps = 20 + Math.floor(rand() * 40);
  const coords: [number, number, number][] = [];
  const speeds: number[] = [];

  for (let i = 0; i <= steps; i++) {
    const t = i / steps;
    const lat = trip.start_lat + (trip.end_lat - trip.start_lat) * t + (rand() - 0.5) * 0.008;
    const lon = trip.start_lon + (trip.end_lon - trip.start_lon) * t + (rand() - 0.5) * 0.008;
    const alt = 10 + rand() * 50;
    coords.push([lon, lat, alt]);

    const baseSpeed = 20 + rand() * 80;
    const edgeFactor = Math.min(t, 1 - t) * 4;
    speeds.push(Math.round(baseSpeed * Math.min(edgeFactor, 1)));
  }

  return {
    type: 'FeatureCollection',
    features: [{
      type: 'Feature',
      geometry: { type: 'LineString', coordinates: coords },
      properties: { speed: speeds },
    }],
  };
}

export function mockTripEvents(id: string) {
  const trip = mockTrip(id);
  if (!trip) return { events: [] };

  const start = new Date(trip.started_at);
  const end = new Date(trip.ended_at);
  const mid = new Date((start.getTime() + end.getTime()) / 2);

  return {
    events: [
      { event_type: 'trip_start', timestamp_at: trip.started_at },
      { event_type: 'motion_detected', timestamp_at: new Date(start.getTime() + 5000).toISOString() },
      { event_type: 'gnss_fix_acquired', timestamp_at: new Date(start.getTime() + 12000).toISOString() },
      { event_type: 'speed_threshold', timestamp_at: new Date(start.getTime() + 60000).toISOString() },
      { event_type: 'route_segment', timestamp_at: mid.toISOString() },
      { event_type: 'speed_threshold', timestamp_at: new Date(end.getTime() - 120000).toISOString() },
      { event_type: 'motion_stopped', timestamp_at: new Date(end.getTime() - 30000).toISOString() },
      { event_type: 'trip_end', timestamp_at: trip.ended_at },
    ],
  };
}

export const mockDevices = {
  devices: [
    {
      id: 'FMTC-001A',
      firmware_version: '2.4.1',
      last_seen_at: new Date(Date.now() - 12 * 60000).toISOString(),
      trip_count: ALL_TRIPS.filter(t => t.device_id === 'FMTC-001A').length,
      total_distance_m: ALL_TRIPS.filter(t => t.device_id === 'FMTC-001A').reduce((s, t) => s + t.distance_m, 0),
    },
    {
      id: 'FMTC-002B',
      firmware_version: '2.3.8',
      last_seen_at: new Date(Date.now() - 6 * 3600000).toISOString(),
      trip_count: ALL_TRIPS.filter(t => t.device_id === 'FMTC-002B').length,
      total_distance_m: ALL_TRIPS.filter(t => t.device_id === 'FMTC-002B').reduce((s, t) => s + t.distance_m, 0),
    },
  ],
};

export const mockPlaces = {
  places: [
    { id: 'place-1', name: 'Home', lat: 37.7749, lon: -122.4194, radius_m: 150, visit_count: 142 },
    { id: 'place-2', name: 'Work', lat: 37.7849, lon: -122.4094, radius_m: 200, visit_count: 98 },
    { id: 'place-3', name: 'Gym', lat: 37.7599, lon: -122.4269, radius_m: 100, visit_count: 34 },
    { id: 'place-4', name: 'Grocery Store', lat: 37.7694, lon: -122.4862, radius_m: 80, visit_count: 27 },
  ],
};
