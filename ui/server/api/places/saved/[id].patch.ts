export default defineEventHandler(async (event) => {
  requireJson(event)
  const id = Number(getRouterParam(event, 'id'))
  if (!Number.isInteger(id)) throw createError({ statusCode: 400, statusMessage: 'bad id' })
  const body = await readBody(event)
  try {
    const saved = getPlaceResolver().saved.update(id, body)
    if (!saved) throw createError({ statusCode: 404, statusMessage: 'No such saved place' })
    return { saved }
  } catch (e) {
    asHttpError(e)
  }
})
