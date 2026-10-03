const config = useRuntimeConfig()

interface TsdbResult {
  columns: string[]
  rows: any[][]
  elapsed_us: number
  truncated: boolean
}

export async function queryTsdb(sql: string): Promise<TsdbResult> {
  const res = await $fetch<TsdbResult>(`${config.tsdbUrl}/query`, {
    method: 'POST',
    body: sql,
    headers: { 'Content-Type': 'text/plain' },
  })
  return res
}

export function tsdbRowsToObjects(result: TsdbResult): Record<string, any>[] {
  return result.rows.map((row) => {
    const obj: Record<string, any> = {}
    result.columns.forEach((col, i) => {
      obj[col] = row[i]
    })
    return obj
  })
}

export async function queryTsdbObjects(sql: string): Promise<Record<string, any>[]> {
  const result = await queryTsdb(sql)
  return tsdbRowsToObjects(result)
}

export async function tsdbHealth(): Promise<{ ok: boolean; status?: any }> {
  try {
    const status = await $fetch(`${config.tsdbUrl}/healthz`)
    return { ok: true, status }
  } catch {
    return { ok: false }
  }
}
