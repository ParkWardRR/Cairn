export function formatDistance(meters: number | null | undefined): string {
  if (meters == null) return '--';
  const km = meters / 1000;
  if (km < 1) return `${Math.round(meters)} m`;
  return `${km.toFixed(1)} km`;
}

export function formatDuration(seconds: number | null | undefined): string {
  if (seconds == null) return '--';
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  if (h === 0 && m === 0) return `${Math.round(seconds)}s`;
  if (h === 0) return `${m}m`;
  return `${h}h ${m}m`;
}

export function formatTime(iso: string | null | undefined): string {
  if (!iso) return '--';
  return new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

export function formatDate(iso: string | null | undefined): string {
  if (!iso) return '--';
  return new Date(iso).toLocaleDateString([], { weekday: 'short', month: 'short', day: 'numeric' });
}

export function formatDateTime(iso: string | null | undefined): string {
  if (!iso) return '--';
  return `${formatDate(iso)} ${formatTime(iso)}`;
}

export function relativeTime(iso: string | null | undefined): string {
  if (!iso) return 'unknown';
  const diff = Math.max(0, Date.now() - new Date(iso).getTime());
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days}d ago`;
  return formatDate(iso);
}

export function isToday(iso: string | null | undefined): boolean {
  if (!iso) return false;
  return new Date(iso).toDateString() === new Date().toDateString();
}

export function isThisWeek(iso: string | null | undefined): boolean {
  if (!iso) return false;
  const weekAgo = new Date();
  weekAgo.setDate(weekAgo.getDate() - 7);
  return new Date(iso) >= weekAgo;
}

export function isThisMonth(iso: string | null | undefined): boolean {
  if (!iso) return false;
  const d = new Date(iso);
  const now = new Date();
  return d.getMonth() === now.getMonth() && d.getFullYear() === now.getFullYear();
}

export function deviceStatusColor(lastSeen: string | null | undefined): 'green' | 'amber' | 'red' {
  if (!lastSeen) return 'red';
  const diff = Date.now() - new Date(lastSeen).getTime();
  if (diff < 3600000) return 'green';
  if (diff < 86400000) return 'amber';
  return 'red';
}

export function speedColor(ratio: number): string {
  if (ratio <= 0.5) {
    const t = ratio * 2;
    const r = Math.round(78 + t * (240 - 78));
    const g = Math.round(204 + t * (165 - 204));
    const b = Math.round(163 + t * (0 - 163));
    return `rgb(${r},${g},${b})`;
  }
  const t = (ratio - 0.5) * 2;
  const r = Math.round(240 + t * (233 - 240));
  const g = Math.round(165 + t * (69 - 165));
  const b = Math.round(0 + t * (96 - 0));
  return `rgb(${r},${g},${b})`;
}

export function formatEventType(type: string | null | undefined): string {
  if (!type) return 'Unknown';
  return type.replace(/_/g, ' ').replace(/\b\w/g, c => c.toUpperCase());
}
