import { describe, expect, it } from 'vitest'
import { buildVehicleScope, parseVehicleParam } from '../server/utils/vehicle'

const N20 = '0190a1b2c3d4e5f60718293a4b5c6d7e'

function status(fn: () => unknown): number | undefined {
  try {
    fn()
  } catch (e: any) {
    return e.statusCode
  }
  return undefined
}

describe('parseVehicleParam', () => {
  it('treats absent, empty and "all" as every vehicle', () => {
    expect(parseVehicleParam(undefined)).toBeNull()
    expect(parseVehicleParam(null)).toBeNull()
    expect(parseVehicleParam('')).toBeNull()
    expect(parseVehicleParam('all')).toBeNull()
    expect(parseVehicleParam('ALL')).toBeNull()
  })

  it('lowercases a 32-hex id', () => {
    expect(parseVehicleParam(N20.toUpperCase())).toBe(N20)
    expect(parseVehicleParam(` ${N20} `)).toBe(N20)
  })

  it('rejects anything else with a 400', () => {
    expect(status(() => parseVehicleParam(N20.slice(1)))).toBe(400)
    expect(status(() => parseVehicleParam(`${N20}0`))).toBe(400)
    expect(status(() => parseVehicleParam('0190a1b2-c3d4-e5f6-0718-293a4b5c6d7e'))).toBe(400)
    expect(status(() => parseVehicleParam(`${N20.slice(2)}' OR '1`))).toBe(400)
    expect(status(() => parseVehicleParam([N20, N20]))).toBe(400)
  })
})

describe('buildVehicleScope', () => {
  it('adds nothing when unfiltered', () => {
    const s = buildVehicleScope(null)
    expect(s.id).toBeNull()
    expect(s.and()).toBe('')
    expect(s.where('b')).toBe('')
  })

  it('filters on one vehicle, with or without a table alias', () => {
    const s = buildVehicleScope(N20)
    expect(s.id).toBe(N20)
    expect(s.and()).toBe(` AND vehicle_id = '${N20}'`)
    expect(s.and('b')).toBe(` AND b.vehicle_id = '${N20}'`)
    expect(s.where()).toBe(` WHERE vehicle_id = '${N20}'`)
    expect(s.where('d')).toBe(` WHERE d.vehicle_id = '${N20}'`)
  })

  it('matches nothing when a single-vehicle route has no vehicle to use', () => {
    const s = buildVehicleScope(null, true)
    expect(s.id).toBeNull()
    expect(s.and('b')).toBe(' AND FALSE')
    expect(s.where()).toBe(' WHERE FALSE')
  })

  it('refuses an id that would not be safe to splice into SQL', () => {
    expect(() => buildVehicleScope("x' OR TRUE --")).toThrow()
  })
})
