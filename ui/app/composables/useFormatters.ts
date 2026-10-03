export function useFormatters() {
  function formatDuration(seconds: number): string {
    if (seconds < 60) return `${Math.round(seconds)}s`
    const mins = Math.floor(seconds / 60)
    if (mins < 60) return `${mins}m`
    const hrs = Math.floor(mins / 60)
    const remainMins = mins % 60
    return remainMins > 0 ? `${hrs}h ${remainMins}m` : `${hrs}h`
  }

  function formatDistance(meters: number): string {
    if (meters < 1000) return `${Math.round(meters)} m`
    const km = meters / 1000
    return km >= 100 ? `${Math.round(km)} km` : `${km.toFixed(1)} km`
  }

  function formatSpeed(kph: number): string {
    return `${Math.round(kph)} km/h`
  }

  function formatTimeAgo(date: string | Date): string {
    const d = typeof date === 'string' ? new Date(date) : date
    const diff = (Date.now() - d.getTime()) / 1000
    if (diff < 60) return 'just now'
    if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
    if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`
    return `${Math.floor(diff / 86400)}d ago`
  }

  function formatDate(date: string | Date): string {
    const d = typeof date === 'string' ? new Date(date) : date
    return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })
  }

  function formatTime(date: string | Date): string {
    const d = typeof date === 'string' ? new Date(date) : date
    return d.toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit' })
  }

  function formatVoltage(mv: number): string {
    return `${(mv / 1000).toFixed(1)}V`
  }

  function formatTemp(c: number): string {
    return `${c}°C`
  }

  return { formatDuration, formatDistance, formatSpeed, formatTimeAgo, formatDate, formatTime, formatVoltage, formatTemp }
}
