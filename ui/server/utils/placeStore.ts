import { DatabaseSync } from 'node:sqlite'
import { createReadStream, mkdirSync, statSync, existsSync } from 'node:fs'
import { createInterface } from 'node:readline'
import { dirname } from 'node:path'
import type { PlaceAddress, PlaceCandidate, PlaceSource } from './placeLabel'
import { distM } from './placeLabel'
import type { FsqRow } from './placeSources'

// Persistent cache of everything the external sources told us about a spot, so
// each location is looked up once. Candidates are stored raw: labels are chosen
// at read time, so changing the scoring never needs another network call.

export interface CachedPlace {
  id: number
  lat: number
  lon: number
  candidates: PlaceCandidate[]
  address: PlaceAddress | null
  // Sources that answered, and those that were tried but failed or were off.
  consulted: PlaceSource[]
  failed: PlaceSource[]
  resolved_at: number
  attempted_at: number
  version: number
}

const SCHEMA = `
CREATE TABLE IF NOT EXISTS resolutions (
  id INTEGER PRIMARY KEY,
  lat REAL NOT NULL, lon REAL NOT NULL,
  candidates TEXT NOT NULL, address TEXT,
  consulted TEXT NOT NULL, failed TEXT NOT NULL,
  resolved_at INTEGER NOT NULL, attempted_at INTEGER NOT NULL, version INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS resolutions_lat ON resolutions (lat);
CREATE TABLE IF NOT EXISTS source_calls (
  day TEXT NOT NULL, source TEXT NOT NULL, n INTEGER NOT NULL, PRIMARY KEY (day, source)
);
CREATE TABLE IF NOT EXISTS fsq_poi (
  lat REAL NOT NULL, lon REAL NOT NULL, name TEXT NOT NULL, category TEXT
);
CREATE INDEX IF NOT EXISTS fsq_poi_lat ON fsq_poi (lat, lon);
CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);
`

const M_PER_DEG_LAT = 110540

export class PlaceStore {
  private db: DatabaseSync

  constructor(path: string) {
    if (path !== ':memory:') mkdirSync(dirname(path), { recursive: true })
    this.db = new DatabaseSync(path)
    this.db.exec('PRAGMA journal_mode = WAL')
    this.db.exec(SCHEMA)
  }

  close() { this.db.close() }

  nearest(lat: number, lon: number, radiusM = 40): CachedPlace | null {
    const dLat = radiusM / M_PER_DEG_LAT
    const rows = this.db.prepare('SELECT * FROM resolutions WHERE lat BETWEEN ? AND ?').all(lat - dLat, lat + dLat) as any[]
    let best: any = null
    let bestD = radiusM
    for (const r of rows) {
      const d = distM(lat, lon, r.lat, r.lon)
      if (d <= bestD) { best = r; bestD = d }
    }
    return best ? this.fromRow(best) : null
  }

  save(p: Omit<CachedPlace, 'id'> & { id?: number }): void {
    const args = [
      p.lat, p.lon, JSON.stringify(p.candidates), p.address ? JSON.stringify(p.address) : null,
      JSON.stringify(p.consulted), JSON.stringify(p.failed), p.resolved_at, p.attempted_at, p.version,
    ]
    if (p.id != null) {
      this.db.prepare(`UPDATE resolutions SET lat=?, lon=?, candidates=?, address=?, consulted=?, failed=?,
        resolved_at=?, attempted_at=?, version=? WHERE id=?`).run(...args as any[], p.id)
    } else {
      this.db.prepare(`INSERT INTO resolutions (lat, lon, candidates, address, consulted, failed,
        resolved_at, attempted_at, version) VALUES (?,?,?,?,?,?,?,?,?)`).run(...args as any[])
    }
  }

  private fromRow(r: any): CachedPlace {
    return {
      id: r.id, lat: r.lat, lon: r.lon,
      candidates: JSON.parse(r.candidates),
      address: r.address ? JSON.parse(r.address) : null,
      consulted: JSON.parse(r.consulted),
      failed: JSON.parse(r.failed),
      resolved_at: r.resolved_at, attempted_at: r.attempted_at, version: r.version,
    }
  }

  callsToday(source: PlaceSource, day: string): number {
    const r = this.db.prepare('SELECT n FROM source_calls WHERE day=? AND source=?').get(day, source) as any
    return r?.n ?? 0
  }

  addCalls(source: PlaceSource, day: string, n: number): void {
    this.db.prepare(`INSERT INTO source_calls (day, source, n) VALUES (?,?,?)
      ON CONFLICT(day, source) DO UPDATE SET n = n + excluded.n`).run(day, source, n)
  }

  fsqCount(): number {
    return (this.db.prepare('SELECT count(*) AS n FROM fsq_poi').get() as any).n
  }

  fsqNear(lat: number, lon: number, radiusM: number): FsqRow[] {
    const dLat = radiusM / M_PER_DEG_LAT
    const dLon = radiusM / (111320 * Math.cos(lat * Math.PI / 180))
    return this.db.prepare(
      'SELECT lat, lon, name, category FROM fsq_poi WHERE lat BETWEEN ? AND ? AND lon BETWEEN ? AND ?',
    ).all(lat - dLat, lat + dLat, lon - dLon, lon + dLon) as unknown as FsqRow[]
  }

  getMeta(key: string): string | null {
    return (this.db.prepare('SELECT value FROM meta WHERE key=?').get(key) as any)?.value ?? null
  }

  setMeta(key: string, value: string): void {
    this.db.prepare('INSERT INTO meta (key, value) VALUES (?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value').run(key, value)
  }

  // Loads an NDJSON export from cairn-fsq (one {name,lat,lon,category} per line)
  // when it is new or changed. Returns the number of rows loaded, or 0 if the
  // file is absent or already loaded.
  private importing: Promise<number> | null = null

  // Concurrent callers share one import.
  importFsq(file: string): Promise<number> {
    if (!this.importing) {
      this.importing = this.doImportFsq(file).finally(() => { this.importing = null })
    }
    return this.importing
  }

  private async doImportFsq(file: string): Promise<number> {
    if (!existsSync(file)) return 0
    const st = statSync(file)
    const stamp = `${st.mtimeMs}:${st.size}`
    if (this.getMeta('fsq_stamp') === stamp) return 0

    this.db.exec('BEGIN')
    try {
      this.db.exec('DELETE FROM fsq_poi')
      const ins = this.db.prepare('INSERT INTO fsq_poi (lat, lon, name, category) VALUES (?,?,?,?)')
      let n = 0
      const rl = createInterface({ input: createReadStream(file), crlfDelay: Infinity })
      for await (const line of rl) {
        if (!line.trim()) continue
        const r = JSON.parse(line)
        if (!r.name || r.lat == null || r.lon == null) continue
        ins.run(r.lat, r.lon, r.name, r.category ?? null)
        n++
      }
      this.setMeta('fsq_stamp', stamp)
      this.db.exec('COMMIT')
      return n
    } catch (e) {
      this.db.exec('ROLLBACK')
      throw e
    }
  }
}
