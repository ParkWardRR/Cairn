export default defineEventHandler(async (event) => {
  const query = getQuery(event)
  const lat = parseFloat(String(query.lat))
  const lon = parseFloat(String(query.lon))

  if (isNaN(lat) || isNaN(lon)) {
    throw createError({ statusCode: 400, statusMessage: 'lat and lon query params required' })
  }

  const name = await reverseGeocode(lat, lon)
  return { name }
})
