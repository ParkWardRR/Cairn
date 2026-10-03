import { SignJWT, importPKCS8 } from 'jose'
import { readFileSync } from 'node:fs'

const config = useRuntimeConfig()

let cachedKey: CryptoKey | null = null
let cachedToken: string | null = null
let tokenExpiry = 0

async function getPrivateKey(): Promise<CryptoKey> {
  if (cachedKey) return cachedKey
  const pem = readFileSync(config.appleMapPrivateKeyPath, 'utf-8')
  cachedKey = await importPKCS8(pem, 'ES256')
  return cachedKey
}

export async function mintMapKitToken(): Promise<string> {
  const now = Math.floor(Date.now() / 1000)
  if (cachedToken && now < tokenExpiry - 60) return cachedToken

  const key = await getPrivateKey()
  const token = await new SignJWT({})
    .setProtectedHeader({ alg: 'ES256', kid: config.appleMapKeyId, typ: 'JWT' })
    .setIssuer(config.appleMapTeamId)
    .setIssuedAt(now)
    .setExpirationTime(now + 3600)
    .sign(key)

  cachedToken = token
  tokenExpiry = now + 3600
  return token
}

export async function reverseGeocode(lat: number, lon: number): Promise<string | null> {
  try {
    const token = await mintMapKitToken()
    const res = await $fetch<any>('https://maps-api.apple.com/v1/reverseGeocode', {
      params: { loc: `${lat},${lon}` },
      headers: { Authorization: `Bearer ${token}` },
    })
    const place = res?.results?.[0]
    if (!place) return null
    return place.name || place.formattedAddressLines?.[0] || null
  } catch {
    return null
  }
}
