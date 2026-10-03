export default defineEventHandler(async (event) => {
  const body = await readBody(event)
  const sql = typeof body === 'string' ? body : body?.sql

  if (!sql || typeof sql !== 'string') {
    throw createError({ statusCode: 400, statusMessage: 'SQL query required in body (string or { sql })' })
  }

  const trimmed = sql.trim()
  if (!/^select\b/i.test(trimmed)) {
    throw createError({ statusCode: 403, statusMessage: 'Only SELECT statements are allowed' })
  }

  const result = await queryTsdb(trimmed)
  return result
})
