export default defineEventHandler((event) => {
  const id = Number(getRouterParam(event, 'id'))
  if (!Number.isInteger(id)) throw createError({ statusCode: 400, statusMessage: 'bad id' })
  if (!getPlaceResolver().saved.remove(id)) throw createError({ statusCode: 404, statusMessage: 'No such saved place' })
  return { ok: true }
})
