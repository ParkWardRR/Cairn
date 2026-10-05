import { DatabaseSync } from 'node:sqlite'
import { existsSync, mkdirSync, readdirSync, readFileSync, renameSync, statSync, unlinkSync, writeFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { distM } from './placeLabel'

// Places the user has named or corrected, and places the system has learned
// from repeat visits. Kept apart from the lookup cache: the cache can be deleted
// and rebuilt at any time, but these cannot be derived from anything, so they
// are written out as JSON after every change and rotated into backups, and a
// lost database is restored from that file on the next start.

export type PlaceSourceKind = 'user' | 'learned'

export interface SavedPlace {
  id: number
  name: string
  category: string | null
  lat: number
  lon: number
  radius_m: number
  note: string | null
  // user: named or confirmed by a person. learned: found automatically because
  // the spot kept coming up; it stays learned until someone edits or confirms it.
  source: PlaceSourceKind
  // For learned places, JSON describing why: the lookup sources, confidence and
  // trips that led to it.
  learned_from: string | null
  created_at: number
  updated_at: number
}

export type SavedInput = Pick<SavedPlace, 'name' | 'lat' | 'lon'> &
  Partial<Pick<SavedPlace, 'category' | 'radius_m' | 'note'>>

export interface Dismissal {
  lat: number
  lon: number
  radius_m: number
  created_at: number
}

export interface SavedExport {
  version: 1
  exported_at: string
  saved: Array<Omit<SavedPlace, 'id'>>
  dismissed: Dismissal[]
}

export const DEFAULT_RADIUS_M = 100
export const MIN_RADIUS_M = 25
export const MAX_RADIUS_M = 500
const BACKUP_KEEP = 30
const BACKUP_EVERY_MS = 60 * 60 * 1000

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
CREATE TABLE IF NOT EXISTS dismissed_places (
  id INTEGER PRIMARY KEY,
  lat REAL NOT NULL, lon REAL NOT NULL, radius_m INTEGER NOT NULL, created_at INTEGER NOT NULL
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

export interface SavedPlacesOptions {
  // Where the JSON mirror and rotating backups go. Omit for none.
  backupDir?: string
  // Called when a backup could not be written; defaults to console.error.
  onBackupError?: (e: unknown) => void
}

export class SavedPlaces {
  private db: DatabaseSync
  private backupDir?: string
  private onBackupError: (e: unknown) => void

  constructor(path: string, opts: SavedPlacesOptions = {}) {
    if (path !== ':memory:') mkdirSync(dirname(path), { recursive: true })
    this.db = new DatabaseSync(path)
    this.db.exec('PRAGMA journal_mode = WAL')
    this.db.exec(SCHEMA)
    this.migrate()
    this.backupDir = opts.backupDir
    this.onBackupError = opts.onBackupError ?? (e => console.error('[places] saved-places backup failed:', e))
    this.restoreIfEmpty()

    // Make sure a mirror exists from the start, but never replace an existing one
    // with an empty export (it may be the only good copy, or merely unreadable).
    const mirror = this.mirrorPath
    if (mirror && (!existsSync(mirror) || this.list().length > 0)) this.snapshot()
  }

  close() { this.db.close() }

  // Adds columns introduced after the first release to an existing file.
  private migrate() {
    const cols = new Set((this.db.prepare('PRAGMA table_info(saved_places)').all() as any[]).map(c => c.name))
    if (!cols.has('source')) this.db.exec(`ALTER TABLE saved_places ADD COLUMN source TEXT NOT NULL DEFAULT 'user'`)
    if (!cols.has('learned_from')) this.db.exec('ALTER TABLE saved_places ADD COLUMN learned_from TEXT')
  }

  // ---- reads ----

  list(): SavedPlace[] {
    return this.db.prepare('SELECT * FROM saved_places ORDER BY name COLLATE NOCASE').all() as unknown as SavedPlace[]
  }

  get(id: number): SavedPlace | null {
    return (this.db.prepare('SELECT * FROM saved_places WHERE id = ?').get(id) as unknown as SavedPlace) ?? null
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

  dismissals(): Dismissal[] {
    return this.db.prepare('SELECT lat, lon, radius_m, created_at FROM dismissed_places').all() as unknown as Dismissal[]
  }

  // True when the user removed a place here, so it must not be learned again.
  isDismissed(lat: number, lon: number): boolean {
    return this.dismissals().some(d => distM(lat, lon, d.lat, d.lon) <= d.radius_m)
  }

  // ---- writes ----

  create(input: SavedInput, opts: { source?: PlaceSourceKind; learned_from?: unknown } = {}, now = Date.now()): SavedPlace {
    const c = cleanSaved(input) as SavedInput
    const res = this.db.prepare(`INSERT INTO saved_places
      (name, category, lat, lon, radius_m, note, source, learned_from, created_at, updated_at)
      VALUES (?,?,?,?,?,?,?,?,?,?)`).run(
      c.name, c.category ?? null, c.lat, c.lon, c.radius_m ?? DEFAULT_RADIUS_M, c.note ?? null,
      opts.source ?? 'user', opts.learned_from == null ? null : JSON.stringify(opts.learned_from), now, now,
    )
    this.snapshot()
    return this.get(Number(res.lastInsertRowid))!
  }

  // Any edit makes the place the user's: a learned place stops being learned the
  // moment someone touches it.
  update(id: number, patch: Partial<SavedInput>, now = Date.now()): SavedPlace | null {
    const cur = this.get(id)
    if (!cur) return null
    const c = cleanSaved(patch, true)
    const next = { ...cur, ...c }
    this.db.prepare(`UPDATE saved_places SET name=?, category=?, lat=?, lon=?, radius_m=?, note=?, source='user', updated_at=? WHERE id=?`)
      .run(next.name, next.category, next.lat, next.lon, next.radius_m, next.note, now, id)
    this.snapshot()
    return this.get(id)
  }

  // Accepts a learned place as it is.
  confirm(id: number, now = Date.now()): SavedPlace | null {
    if (!this.get(id)) return null
    this.db.prepare(`UPDATE saved_places SET source='user', updated_at=? WHERE id=?`).run(now, id)
    this.snapshot()
    return this.get(id)
  }

  // Removing a place also remembers that the spot was rejected, so the learner
  // does not bring it straight back.
  remove(id: number, now = Date.now()): boolean {
    const cur = this.get(id)
    if (!cur) return false
    this.db.prepare('DELETE FROM saved_places WHERE id = ?').run(id)
    this.db.prepare('INSERT INTO dismissed_places (lat, lon, radius_m, created_at) VALUES (?,?,?,?)').run(cur.lat, cur.lon, cur.radius_m, now)
    this.snapshot()
    return true
  }

  // ---- export, import, backup ----

  exportAll(now = new Date()): SavedExport {
    return {
      version: 1,
      exported_at: now.toISOString(),
      saved: this.list().map(({ id: _id, ...rest }) => rest),
      dismissed: this.dismissals(),
    }
  }

  // Merges an export into this store without touching what is already here: a
  // place is skipped when one with the same name lies within 10 m.
  importAll(data: unknown): { added: number; skipped: number; dismissed: number } {
    const d = data as Partial<SavedExport> | null
    if (!d || d.version !== 1 || !Array.isArray(d.saved)) throw new SavedPlaceError('Not a Cairn saved-places export')

    const have = this.list()
    let added = 0
    let skipped = 0
    for (const s of d.saved) {
      const clean = cleanSaved({ name: s.name, lat: s.lat, lon: s.lon, radius_m: s.radius_m, category: s.category, note: s.note }) as SavedInput
      if (have.some(h => h.name.toLowerCase() === clean.name.toLowerCase() && distM(h.lat, h.lon, clean.lat, clean.lon) <= 10)) {
        skipped++
        continue
      }
      this.createRaw(clean, s.source === 'learned' ? 'learned' : 'user', s.learned_from ?? null, s.created_at, s.updated_at)
      added++
    }

    let dismissed = 0
    const known = this.dismissals()
    for (const x of Array.isArray(d.dismissed) ? d.dismissed : []) {
      if (![x?.lat, x?.lon, x?.radius_m].every(Number.isFinite)) continue
      if (known.some(k => distM(k.lat, k.lon, x.lat, x.lon) <= 10)) continue
      this.db.prepare('INSERT INTO dismissed_places (lat, lon, radius_m, created_at) VALUES (?,?,?,?)')
        .run(x.lat, x.lon, x.radius_m, x.created_at ?? Date.now())
      dismissed++
    }

    this.snapshot()
    return { added, skipped, dismissed }
  }

  // Inserts without snapshotting, keeping the original timestamps.
  private createRaw(c: SavedInput, source: PlaceSourceKind, learnedFrom: string | null, createdAt?: number, updatedAt?: number) {
    const now = Date.now()
    this.db.prepare(`INSERT INTO saved_places
      (name, category, lat, lon, radius_m, note, source, learned_from, created_at, updated_at)
      VALUES (?,?,?,?,?,?,?,?,?,?)`).run(
      c.name, c.category ?? null, c.lat, c.lon, c.radius_m ?? DEFAULT_RADIUS_M, c.note ?? null,
      source, learnedFrom, createdAt ?? now, updatedAt ?? now,
    )
  }

  // The path of the always-current JSON mirror, if backups are on.
  get mirrorPath(): string | null {
    return this.backupDir ? join(this.backupDir, 'saved-places.json') : null
  }

  // Writes the JSON mirror after every change, and a timestamped copy at most
  // once an hour so one bad edit cannot overwrite the only good file.
  snapshot(now = new Date()): void {
    if (!this.backupDir) return
    try {
      mkdirSync(join(this.backupDir, 'backups'), { recursive: true })
      const body = JSON.stringify(this.exportAll(now), null, 2) + '\n'
      const tmp = join(this.backupDir, 'saved-places.json.tmp')
      writeFileSync(tmp, body, { mode: 0o640 })
      renameSync(tmp, this.mirrorPath!)

      const dir = join(this.backupDir, 'backups')
      const files = readdirSync(dir).filter(f => f.startsWith('saved-places-') && f.endsWith('.json')).sort()
      const newest = files.at(-1)
      const age = newest ? now.getTime() - statSync(join(dir, newest)).mtimeMs : Infinity
      if (age >= BACKUP_EVERY_MS) {
        const stamp = now.toISOString().replace(/[-:]/g, '').replace(/\.\d+Z$/, 'Z')
        writeFileSync(join(dir, `saved-places-${stamp}.json`), body, { mode: 0o640 })
        files.push(`saved-places-${stamp}.json`)
        for (const old of files.slice(0, Math.max(0, files.length - BACKUP_KEEP))) unlinkSync(join(dir, old))
      }
    } catch (e) {
      this.onBackupError(e)
    }
  }

  // When the database is empty but a mirror exists (the sqlite file was lost or
  // replaced), bring the places back from it.
  private restoreIfEmpty() {
    const mirror = this.mirrorPath
    if (!mirror || !existsSync(mirror)) return
    const empty = this.list().length === 0 && this.dismissals().length === 0
    if (!empty) return
    try {
      const r = this.importAll(JSON.parse(readFileSync(mirror, 'utf8')))
      if (r.added) console.log(`[places] restored ${r.added} saved places from ${mirror}`)
    } catch (e) {
      this.onBackupError(e)
    }
  }

  // The newest timestamped backup, for the UI to show.
  lastBackupAt(): number | null {
    if (!this.backupDir) return null
    try {
      const dir = join(this.backupDir, 'backups')
      const files = readdirSync(dir).filter(f => f.startsWith('saved-places-')).sort()
      return files.length ? statSync(join(dir, files.at(-1)!)).mtimeMs : null
    } catch { return null }
  }
}
