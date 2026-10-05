// Merges an export back in. Places already present (same name, within 10 m) are
// skipped, so importing the same file twice changes nothing.
export default defineEventHandler(async (event) => {
  requireJson(event)
  const body = await readBody(event)
  try {
    return getPlaceResolver().saved.importAll(body)
  } catch (e) {
    asHttpError(e)
  }
})
