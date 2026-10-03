export default defineEventHandler(async () => {
  const rows = await queryTsdbObjects(`
    SELECT * FROM bundles ORDER BY boot_id DESC
  `)

  return { bundles: rows }
})
