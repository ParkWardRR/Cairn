// A download of every saved and learned place, and the spots that were rejected.
// The same JSON is kept beside the database on the server after every change.
export default defineEventHandler((event) => {
  const data = getPlaceResolver().saved.exportAll()
  setResponseHeader(event, 'content-type', 'application/json')
  setResponseHeader(event, 'content-disposition', `attachment; filename="cairn-saved-places-${data.exported_at.slice(0, 10)}.json"`)
  return data
})
