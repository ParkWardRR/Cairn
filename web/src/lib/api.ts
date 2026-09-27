import { mockStats, mockTrips, mockTrip, mockTripRoute, mockTripEvents, mockDevices, mockPlaces } from '$lib/mock/data';

const BASE = '/api/v1';

let useMock = false;

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  if (useMock) throw new Error('Using mock data');
  const resp = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json', ...options.headers },
    ...options,
  });
  if (resp.status === 204) return null as T;
  if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
  const ct = resp.headers.get('content-type') || '';
  if (ct.includes('json')) return resp.json();
  return resp as unknown as T;
}

async function withMockFallback<T>(live: () => Promise<T>, mock: () => T): Promise<T> {
  if (useMock) return mock();
  try {
    return await live();
  } catch {
    useMock = true;
    return mock();
  }
}

export const api = {
  health: () => withMockFallback(
    () => request<any>('/health'),
    () => ({ status: 'ok', mock: true }),
  ),
  stats: () => withMockFallback(
    () => request<any>('/stats'),
    () => mockStats,
  ),
  trips: (params: Record<string, string | number> = {}) => withMockFallback(
    () => {
      const q = new URLSearchParams();
      for (const [k, v] of Object.entries(params)) {
        if (v != null && v !== '') q.set(k, String(v));
      }
      const qs = q.toString();
      return request<any>(`/trips${qs ? '?' + qs : ''}`);
    },
    () => mockTrips(params),
  ),
  trip: (id: string) => withMockFallback(
    () => request<any>(`/trips/${id}`),
    () => mockTrip(id),
  ),
  tripRoute: (id: string) => withMockFallback(
    () => request<any>(`/trips/${id}/route`),
    () => mockTripRoute(id),
  ),
  tripEvents: (id: string) => withMockFallback(
    () => request<any>(`/trips/${id}/events`),
    () => mockTripEvents(id),
  ),
  deleteTrip: (id: string) => withMockFallback(
    () => request<void>(`/trips/${id}`, { method: 'DELETE' }),
    () => undefined as any,
  ),
  addTag: (tripId: string, tag: string) => withMockFallback(
    () => request<void>(`/trips/${tripId}/tags`, { method: 'POST', body: JSON.stringify({ tag }) }),
    () => undefined as any,
  ),
  removeTag: (tripId: string, tag: string) => withMockFallback(
    () => request<void>(`/trips/${tripId}/tags/${encodeURIComponent(tag)}`, { method: 'DELETE' }),
    () => undefined as any,
  ),
  devices: () => withMockFallback(
    () => request<any>('/devices'),
    () => mockDevices,
  ),
  deviceStatus: (id: string) => withMockFallback(
    () => request<any>(`/devices/${id}/status`),
    () => ({}),
  ),
  places: () => withMockFallback(
    () => request<any>('/places'),
    () => mockPlaces,
  ),
  createPlace: (data: any) => withMockFallback(
    () => request<any>('/places', { method: 'POST', body: JSON.stringify(data) }),
    () => ({ ...data, id: `place-${Date.now()}` }),
  ),
  updatePlace: (id: string, data: any) => withMockFallback(
    () => request<any>(`/places/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
    () => data,
  ),
  deletePlace: (id: string) => withMockFallback(
    () => request<void>(`/places/${id}`, { method: 'DELETE' }),
    () => undefined as any,
  ),
};
