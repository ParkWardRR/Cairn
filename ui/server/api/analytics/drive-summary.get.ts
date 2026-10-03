export default defineEventHandler(async () => {
  const rows = await queryTsdbObjects(`
    SELECT * FROM v_drive_summary ORDER BY boot_id DESC
  `)

  return { driveSummary: rows }
})
