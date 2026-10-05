export default defineEventHandler(async (event) => {
  const v = await vehicleScope(event)
  const rows = await queryTsdbObjects(`
    SELECT * FROM v_reproducibility${v.where()} ORDER BY content_root
  `)

  return { reproducibility: rows }
})
