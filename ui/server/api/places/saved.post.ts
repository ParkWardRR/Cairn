export default defineEventHandler(async (event) => {
  requireJson(event)
  const body = await readBody(event)
  try {
    return { saved: getPlaceResolver().saved.create(body) }
  } catch (e) {
    asHttpError(e)
  }
})
