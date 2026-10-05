// Runs the place ingest and learner in the background: shortly after start, then
// every minute. Each run is a cheap count query unless trips have changed.
export default defineNitroPlugin(() => {
  if (process.env.VITEST) return

  const tick = async () => {
    try {
      const r = await ingestPlaces()
      const learned = learnPlaces(getPlaceResolver())
      if (!r.skipped || learned) {
        console.log(`[places] ingest: ${r.skipped ? 'trips unchanged' : `${r.trips} trips, ${r.visits} visits recorded`}${learned ? `, learned ${learned} place${learned === 1 ? '' : 's'}` : ''}`)
      }
    } catch (e: any) {
      console.error('[places] ingest failed:', e?.message ?? e)
    }
  }

  setTimeout(tick, 15_000)
  setInterval(tick, 60_000)
})
