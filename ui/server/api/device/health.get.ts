export default defineEventHandler(async (event) => {
  const query = getQuery(event)
  const limit = Math.min(Math.max(parseInt(String(query.limit ?? '100'), 10) || 100, 1), 10000)

  const rows = await queryTsdbObjects(`
    SELECT * FROM status ORDER BY observed_at DESC LIMIT ${limit}
  `)

  return { health: rows }
})
