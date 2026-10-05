export default defineEventHandler(async (event) => {
  requireJson(event)
  const id = Number(getRouterParam(event, 'id'))
  if (!Number.isInteger(id)) throw createError({ statusCode: 400, statusMessage: 'bad id' })
  const body = await readBody(event)
  const { saved } = getPlaceResolver()
  try {
    // { confirm: true } accepts a learned place as it is.
    const next = body?.confirm === true ? saved.confirm(id) : saved.update(id, body)
    if (!next) throw createError({ statusCode: 404, statusMessage: 'No such saved place' })
    return { saved: next }
  } catch (e) {
    asHttpError(e)
  }
})
