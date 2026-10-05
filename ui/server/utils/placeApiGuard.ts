import type { H3Event } from 'h3'
import { SavedPlaceError } from './placeSaved'

// Mutating endpoints take JSON only. A cross-site form post cannot send that
// content type without a CORS preflight, which this server does not grant.
export function requireJson(event: H3Event): void {
  const ct = getRequestHeader(event, 'content-type') ?? ''
  if (!ct.toLowerCase().includes('application/json')) {
    throw createError({ statusCode: 415, statusMessage: 'Send application/json' })
  }
}

export function asHttpError(e: unknown): never {
  if (e instanceof SavedPlaceError) throw createError({ statusCode: 400, statusMessage: e.message })
  throw e
}
