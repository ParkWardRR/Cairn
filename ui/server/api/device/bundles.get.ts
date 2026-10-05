export default defineEventHandler(async (event) => {
  const v = await vehicleScope(event)
  const rows = await queryTsdbObjects(`
    SELECT * FROM bundles${v.where()} ORDER BY boot_id DESC
  `)

  return { bundles: rows }
})
