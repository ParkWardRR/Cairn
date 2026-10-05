export default defineEventHandler(async (event) => {
  const v = await vehicleScope(event)
  const rows = await queryTsdbObjects(`
    SELECT * FROM v_drive_summary${v.where()} ORDER BY boot_id DESC
  `)

  return { driveSummary: rows }
})
