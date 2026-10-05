import { DatabaseSync } from 'node:sqlite'
import { mkdirSync } from 'node:fs'
import { dirname } from 'node:path'
import { distM } from './placeLabel'

// Places the user has named or corrected. Kept in a file of its own, apart from
// the lookup cache: the cache can be deleted at any time and rebuilt, but these
// are the user's decisions and must survive that.

export interface SavedPlace {
  id: number
  name: string
  category: string | null
  lat: number
  lon: number
  radius_m: number
  note: string | null
  created_at: number
  updated_at: number
}

export type SavedInput = Pick<SavedPlace, 'name' | 'lat' | 'lon'> &
  Partial<Pick<SavedPlace, 'category' | 'radius_m' | 'note'>>

export const DEFAULT_RADIUS_M = 100
export const MIN_RADIUS_M = 25
export const MAX_RADIUS_M = 500

const SCHEMA = `
CREATE TABLE IF NOT EXISTS saved_places (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  category TEXT,
  lat REAL NOT NULL, lon REAL NOT NULL,
  radius_m INTEGER NOT NULL,
  note TEXT,
  created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL
);
`

export class SavedPlaceError extends Error {}

// Validates and normalises user input; throws SavedPlaceError with a message
// fit to show the user.
export function cleanSaved(input: Partial<SavedInput>, partial = false): Partial<SavedInput> {
  const out: Partial<SavedInput> = {}

  if (input.name !== undefined || !partial) {
    const name = String(input.name ?? '').trim()
    if (!name) throw new SavedPlaceError('A place needs a name')
    if (name.length > 80) throw new SavedPlaceError('Names are limited to 80 characters')
    out.name = name
  }
  for (const k of ['lat', 'lon'] as const) {
    if (input[k] !== undefined || !partial) {
      const v = Number(input[k])
      if (!Number.isFinite(v)) throw new SavedPlaceError(`${k} must be a number`)
      if (k === 'lat' && Math.abs(v) > 90) throw new SavedPlaceError('lat is out of range')
      if (k === 'lon' && Math.abs(v) > 180) throw new SavedPlaceError('lon is out of range')
      out[k] = v
    }
  }
  if (input.radius_m !== undefined) {
    const r = Math.round(Number(input.radius_m))
    if (!Number.isFinite(r) || r < MIN_RADIUS_M || r > MAX_RADIUS_M) {
      throw new SavedPlaceError(`Radius must be ${MIN_RADIUS_M}–${MAX_RADIUS_M} m`)
    }
    out.radius_m = r
  }
  if (input.category !== undefined) {
    const c = input.category == null ? '' : String(input.category).trim()
    if (c.length > 24) throw new SavedPlaceError('Categories are limited to 24 characters')
    out.category = c || null
  }
  if (input.note !== undefined) {
    const n = input.note == null ? '' : String(input.note).trim()
    if (n.length > 500) throw new SavedPlaceError('Notes are limited to 500 characters')
    out.note = n || null
  }
  return out
}

export class SavedPlaces {
  private db: DatabaseSync

  constructor(path: string) {
    if (path !== ':memory:') mkdirSync(dirname(path), { recursive: true })
    this.db = new DatabaseSync(path)
    this.db.exec('PRAGMA journal_mode = WAL')
    this.db.exec(SCHEMA)
  }

  close() { this.db.close() }

  list(): SavedPlace[] {
    return this.db.prepare('SELECT * FROM saved_places ORDER BY name COLLATE NOCASE').all() as unknown as SavedPlace[]
  }

  get(id: number): SavedPlace | null {
    return (this.db.prepare('SELECT * FROM saved_places WHERE id = ?').get(id) as unknown as SavedPlace) ?? null
  }

  create(input: SavedInput, now = Date.now()): SavedPlace {
    const c = cleanSaved(input) as SavedInput
    const res = this.db.prepare(`INSERT INTO saved_places (name, category, lat, lon, radius_m, note, created_at, updated_at)
      VALUES (?,?,?,?,?,?,?,?)`).run(c.name, c.category ?? null, c.lat, c.lon, c.radius_m ?? DEFAULT_RADIUS_M, c.note ?? null, now, now)
    return this.get(Number(res.lastInsertRowid))!
  }

  update(id: number, patch: Partial<SavedInput>, now = Date.now()): SavedPlace | null {
    const cur = this.get(id)
    if (!cur) return null
    const c = cleanSaved(patch, true)
    const next = { ...cur, ...c }
    this.db.prepare(`UPDATE saved_places SET name=?, category=?, lat=?, lon=?, radius_m=?, note=?, updated_at=? WHERE id=?`)
      .run(next.name, next.category, next.lat, next.lon, next.radius_m, next.note, now, id)
    return this.get(id)
  }

  remove(id: number): boolean {
    return Number(this.db.prepare('DELETE FROM saved_places WHERE id = ?').run(id).changes) > 0
  }

  // The saved place whose circle covers the point; the nearest centre wins when
  // several do, so a small saved place inside a bigger one still resolves to it.
  match(lat: number, lon: number): SavedPlace | null {
    let best: SavedPlace | null = null
    let bestD = Infinity
    for (const p of this.list()) {
      const d = distM(lat, lon, p.lat, p.lon)
      if (d <= p.radius_m && d < bestD) { best = p; bestD = d }
    }
    return best
  }
}
