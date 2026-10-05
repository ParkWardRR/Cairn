// How a stop is drawn, shared by the trip map, the stops sidebar and Places.
// Cool colours on purpose: red and amber filled circles read as crash markers.
export type StopCategory = 'short' | 'medium' | 'long'

export const STOP_COLOR: Record<StopCategory, string> = {
  quick: '#38bdf8',
  medium: '#2dd4bf',
  long: '#a78bfa',
}

export const STOP_LABEL: Record<StopCategory, string> = {
  quick: 'Short',
  medium: 'Medium',
  long: 'Long',
}

export const STOP_SIZE: Record<StopCategory, number> = {
  quick: 20,
  medium: 24,
  long: 28,
}

export const ENDPOINT_COLOR = '#64748b'

// A rounded badge with a pause glyph, ringed in white when selected.
export function stopBadgeHtml(category: StopCategory, opts: { selected?: boolean; inferred?: boolean } = {}): string {
  const size = STOP_SIZE[category]
  const color = STOP_COLOR[category]
  const ring = opts.selected ? '3px solid #fff' : '2px solid #0f1117'
  const dash = opts.inferred ? 'opacity:.75;' : ''
  const bar = Math.max(3, Math.round(size / 7))
  const h = Math.round(size * 0.46)
  return `<div style="width:${size}px;height:${size}px;border-radius:7px;background:${color};border:${ring};${dash}box-shadow:0 2px 8px rgba(0,0,0,.55);display:flex;align-items:center;justify-content:center;gap:${bar}px"><span style="width:${bar}px;height:${h}px;background:#0f1117;border-radius:1px"></span><span style="width:${bar}px;height:${h}px;background:#0f1117;border-radius:1px"></span></div>`
}
