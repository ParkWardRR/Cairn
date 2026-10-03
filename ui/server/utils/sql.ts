export function sqlString(val: string): string {
  return `'${val.replace(/'/g, "''")}'`
}

export function sqlInt(val: string): number {
  const n = parseInt(val, 10)
  if (isNaN(n)) throw createError({ statusCode: 400, statusMessage: 'Invalid integer parameter' })
  return n
}
