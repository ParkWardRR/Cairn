const BASE = '/api/v1';

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
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

export const api = {
  health: ()                 => request<any>('/health'),
  stats:  ()                 => request<any>('/stats'),
  trips:  (params: Record<string, string | number> = {}) => {
    const q = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) {
      if (v != null && v !== '') q.set(k, String(v));
    }
    const qs = q.toString();
    return request<any>(`/trips${qs ? '?' + qs : ''}`);
  },
  trip:       (id: string) => request<any>(`/trips/${id}`),
  tripRoute:  (id: string) => request<any>(`/trips/${id}/route`),
  tripEvents: (id: string) => request<any>(`/trips/${id}/events`),
  deleteTrip: (id: string) => request<void>(`/trips/${id}`, { method: 'DELETE' }),
  addTag:     (tripId: string, tag: string) => request<void>(`/trips/${tripId}/tags`, { method: 'POST', body: JSON.stringify({ tag }) }),
  removeTag:  (tripId: string, tag: string) => request<void>(`/trips/${tripId}/tags/${encodeURIComponent(tag)}`, { method: 'DELETE' }),
  devices:     ()                          => request<any>('/devices'),
  deviceStatus: (id: string)               => request<any>(`/devices/${id}/status`),
  places:       ()                          => request<any>('/places'),
  createPlace:  (data: any)                 => request<any>('/places', { method: 'POST', body: JSON.stringify(data) }),
  updatePlace:  (id: string, data: any)     => request<any>(`/places/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deletePlace:  (id: string)                => request<void>(`/places/${id}`, { method: 'DELETE' }),
};
