// Fuel maths shared by the economy page and the trip detail page. Everything is
// an estimate from MAF airflow and lambda: the ECU's own fuel rate is not read.

export const DEFAULT_ETHANOL_BLEND = 37
export const ETHANOL_BLEND_KEY = 'cairn-ethanol-blend'

export function stoichForBlend(ethPct: number): number {
  return 14.7 - (ethPct / 100) * 5.7
}

export function fuelDensityGPerGal(ethPct: number): number {
  return 2834 + (ethPct / 100) * 154
}

export function energyBtuPerGal(ethPct: number): number {
  return 114000 - (ethPct / 100) * 38000
}

// Fuel flow in gallons per hour from MAF (centigrams/s) and lambda.
export function fuelGalPerHr(mafCgps: number, lambda: number, ethPct: number): number {
  const fuelGs = (mafCgps / 100) / (stoichForBlend(ethPct) * lambda)
  return (fuelGs / fuelDensityGPerGal(ethPct)) * 3600
}

export function calcMpg(speedKph: number, mafCgps: number, lambda: number, ethPct: number): number {
  const gph = fuelGalPerHr(mafCgps, lambda, ethPct)
  if (gph <= 0) return 0
  return (speedKph / 1.60934) / gph
}

// The blend the user has set for this browser, falling back to the default.
export function readEthanolBlend(): number {
  if (typeof localStorage === 'undefined') return DEFAULT_ETHANOL_BLEND
  const stored = localStorage.getItem(ETHANOL_BLEND_KEY)
  const n = stored == null ? NaN : parseInt(stored, 10)
  return Number.isFinite(n) ? n : DEFAULT_ETHANOL_BLEND
}
