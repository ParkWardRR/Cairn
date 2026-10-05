interface LocalVehicle {
  display_name: string | null
  engine_code: string | null
  archived: boolean
}

// Names live in cairn-server, not tsdb. Only these fields are copied out, so
// anything else upstream sends (a VIN included) never reaches the browser.
async function localVehicles(): Promise<Map<string, LocalVehicle>> {
  const names = new Map<string, LocalVehicle>()
  const base = useRuntimeConfig().cairnLocalUrl
  if (!base) return names
  try {
    const res = await $fetch<{ vehicles?: any[] }>(`${base.replace(/\/$/, '')}/v1/local/vehicles`, { timeout: 1500 })
    for (const v of res?.vehicles ?? []) {
      const id = typeof v?.id === 'string' ? v.id.toLowerCase() : ''
      if (!VEHICLE_ID_RE.test(id)) continue
      names.set(id, {
        display_name: typeof v.display_name === 'string' && v.display_name.trim() ? v.display_name.trim() : null,
        engine_code: typeof v.engine_code === 'string' && v.engine_code.trim() ? v.engine_code.trim() : null,
        archived: v.archived === true,
      })
    }
  } catch {
    // Names are cosmetic: an unreachable cairn-server leaves the ids to stand in.
  }
  return names
}

export default defineEventHandler(async () => {
  const [rows, names] = await Promise.all([
    queryTsdbObjects(`
      SELECT * FROM v_vehicles ORDER BY last_observed_at DESC NULLS LAST, vehicle_id
    `),
    localVehicles(),
  ])

  return {
    vehicles: rows.map((r) => {
      const local = names.get(r.vehicle_id)
      return {
        id: r.vehicle_id as string,
        name: local?.display_name ?? `Vehicle ${String(r.vehicle_id).slice(0, 8)}`,
        engine_code: local?.engine_code ?? null,
        archived: local?.archived ?? false,
        bundles: Number(r.bundles ?? 0),
        boots: Number(r.boots ?? 0),
        first_observed_at: r.first_observed_at ?? null,
        last_observed_at: r.last_observed_at ?? null,
      }
    }),
  }
})
