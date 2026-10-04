const config = useRuntimeConfig()

export default defineEventHandler(async (event) => {
  const headers: Record<string, string> = {}

  const ifNoneMatch = getRequestHeader(event, 'if-none-match')
  if (ifNoneMatch) headers['If-None-Match'] = ifNoneMatch

  const query = getQuery(event)
  const params = new URLSearchParams()
  if (query.format) params.set('format', String(query.format))
  const qs = params.toString()

  const res = await fetch(`${config.tsdbUrl}/snapshot${qs ? '?' + qs : ''}`, { headers })

  setResponseStatus(event, res.status)
  for (const key of ['content-type', 'content-length', 'etag', 'last-modified', 'cache-control']) {
    const val = res.headers.get(key)
    if (val) setResponseHeader(event, key, val)
  }

  if (res.status === 304) return null
  if (!res.ok) {
    const text = await res.text()
    throw createError({ statusCode: res.status, statusMessage: text })
  }

  return sendStream(event, res.body as ReadableStream)
})
