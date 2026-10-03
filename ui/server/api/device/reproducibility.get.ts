export default defineEventHandler(async () => {
  const rows = await queryTsdbObjects(`
    SELECT * FROM v_reproducibility ORDER BY content_root
  `)

  return { reproducibility: rows }
})
