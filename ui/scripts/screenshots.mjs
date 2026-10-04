// Captures the README screenshots from a running UI.
//
//   go run ./server/cmd/cairn-tsdb-demo        # synthetic data on :8480
//   npx nuxt dev --port 3123                   # in ui/
//   node scripts/screenshots.mjs http://localhost:3123 ../docs/screenshots
import { chromium } from '@playwright/test'
import { mkdirSync, readFileSync, existsSync } from 'node:fs'

const base = process.argv[2] ?? 'http://localhost:3123'
const out = process.argv[3] ?? '../docs/screenshots'
const scale = Number(process.env.SCALE ?? 1)
mkdirSync(out, { recursive: true })

const browser = await chromium.launch()
const ctx = await browser.newContext({
  viewport: { width: 1440, height: 900 },
  deviceScaleFactor: scale,
  colorScheme: 'light',
  timezoneId: 'America/Los_Angeles',
  locale: 'en-US',
})
// With a CARTO key in ui/.env (NUXT_PUBLIC_CARTO_KEY, gitignored) the app asks
// CARTO for tiles as designed and they pass straight through. Without one CARTO
// answers with an "API KEY REQUIRED" watermark, so fall back to OpenStreetMap
// tiles. Either way the shots are day mode: light UI, CARTO Voyager basemap
// (the app itself only asks for dark_all). Attribution: map data (c) OpenStreetMap contributors.
const keyed = !!process.env.NUXT_PUBLIC_CARTO_KEY ||
  (existsSync('.env') && /^NUXT_PUBLIC_CARTO_KEY=\S+/m.test(readFileSync('.env', 'utf8')))
if (!keyed) console.log('no CARTO key found; using OpenStreetMap tiles')
await ctx.route('**/basemaps.cartocdn.com/**', async (route) => {
  const url = route.request().url()
  if (keyed) return route.continue({ url: url.replace('/dark_all/', '/voyager/') }) // day mode
  const m = url.match(/\/(\d+)\/(\d+)\/(\d+)(?:@2x)?\.png/)
  if (!m) return route.abort()
  const res = await ctx.request.get(`https://tile.openstreetmap.org/${m[1]}/${m[2]}/${m[3]}.png`, {
    headers: { 'User-Agent': 'cairn-readme-screenshots (github.com/ParkWardRR/Cairn)' },
  })
  await route.fulfill({ response: res })
})
const hideDevtools = '#nuxt-devtools-container,nuxt-devtools-inspect-panel,#vue-tracer-overlay{display:none!important}'
await ctx.addInitScript(({ hide }) => {
  addEventListener('DOMContentLoaded', () => {
    const s = document.createElement('style')
    s.textContent = hide
    document.head.appendChild(s)
  })
}, { hide: hideDevtools })
const page = await ctx.newPage()

const { trips } = await (await page.request.get(`${base}/api/trips?limit=200`)).json()
const long = trips.filter(t => t.duration_s > 600).sort((a, b) => b.start_time.localeCompare(a.start_time))
const featured = long[0]

const shots = [
  { name: 'dashboard', path: '/', full: true, map: true },
  { name: 'trips', path: '/trips' },
  { name: 'trip-detail', path: `/trips/${featured.boot_id}`, full: true, map: true },
  { name: 'places', path: '/places', map: true },
  { name: 'boost', path: '/boost', full: true, height: 1250 },
  { name: 'fuel', path: '/fuel', full: true },
  { name: 'behavior', path: '/behavior', full: true, height: 1000 },
  { name: 'calibration', path: '/calibration', full: true },
  { name: 'analytics', path: '/analytics', full: true },
  { name: 'system', path: '/system', full: true, height: 1000 },
]
const only = process.env.ONLY?.split(',')

for (const s of shots) {
  if (only && !only.includes(s.name)) continue
  await page.goto(base + s.path, { waitUntil: 'networkidle' })
  if (s.name === 'analytics') {
    await page.waitForTimeout(3000) // let the dev build hydrate before touching the form
    await page.selectOption('select', featured.boot_id)
  }
  await page.waitForTimeout(s.map || s.name === 'analytics' ? 4000 : 1500) // tiles and chart animations
  if (s.name === 'fuel') {
    // The page footnote names this owner's fuel blend; keep it out of the README.
    await page.evaluate(() => {
      for (const el of document.querySelectorAll('*')) {
        if (el.children.length === 0 && /ethanol/i.test(el.textContent ?? '')) (el.closest('.rounded-xl') ?? el).remove()
      }
    })
  }
  await page.screenshot({
    path: `${out}/${s.name}.png`,
    fullPage: !!s.full,
    ...(s.height ? { clip: { x: 0, y: 0, width: 1440, height: s.height } } : {}),
  })
  console.log(s.name)
}
await browser.close()
