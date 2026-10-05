import { describe, expect, it } from 'vitest'
import { STOP_COLOR, STOP_LABEL, STOP_SIZE } from '../app/composables/useStopStyle'
import { MEDIUM_MAX_S, SHORT_MAX_S, categorize } from '../server/utils/stops'
import { clusterPlaces, visitsFromStops } from '../server/utils/places'

// Every category the server can produce must have a colour, label and size on the
// client: a missing one rendered as "undefined" and crashed a page that called a
// method on the label.
describe('stop categories line up between server and client', () => {
  const produced = [categorize(SHORT_MAX_S), categorize(SHORT_MAX_S + 1), categorize(MEDIUM_MAX_S + 1)]

  it('has a colour, label and size for each category the server produces', () => {
    expect(new Set(produced).size).toBe(3)
    for (const c of produced) {
      expect(STOP_COLOR[c], `colour for ${c}`).toMatch(/^#/)
      expect(STOP_LABEL[c], `label for ${c}`).toBeTruthy()
      expect(STOP_SIZE[c], `size for ${c}`).toBeGreaterThan(0)
    }
  })

  it('has no keys the server does not produce', () => {
    for (const table of [STOP_COLOR, STOP_LABEL, STOP_SIZE]) {
      expect(Object.keys(table).sort()).toEqual([...produced].sort())
    }
  })

  it('counts a place\'s stops under the same keys', () => {
    const stop = (secs: number) => ({
      start_at: null, start_offset_s: 0, start_mono_ms: secs, end_mono_ms: secs, duration_s: secs,
      lat: 34, lon: -118.4, category: categorize(secs), inferred: false,
    })
    const [p] = clusterPlaces([...visitsFromStops('a', [stop(200), stop(600), stop(2000)])])
    expect([p.short, p.medium, p.long]).toEqual([1, 1, 1])
    expect(p.longest_category).toBe('long')
  })
})
