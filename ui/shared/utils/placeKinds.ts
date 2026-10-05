// The kinds of place people go to, each with its own icon. Shared by the app and
// the server, so the same words mean the same thing on both sides.
//
// A kind is a hint the engine uses: it ranks the names offered for a spot, fills
// in a default name, labels learned places, and is returned with every place so
// anything downstream can tell a gym from a grocery store.

export type PlaceKindId =
  | 'home' | 'work' | 'gym' | 'fuel' | 'parking' | 'civic' | 'worship' | 'school'
  | 'health' | 'pets' | 'beauty' | 'auto' | 'services' | 'travel' | 'groceries'
  | 'food' | 'nightlife' | 'outdoors' | 'leisure' | 'shopping' | 'friends' | 'other'

export interface PlaceKind {
  id: PlaceKindId
  label: string
  // Name suggested when the user picks this kind for a spot with no name yet.
  defaultName?: string
  // SVG path data on a 24x24 grid, drawn as an outline.
  paths: string[]
  // Words that mark a lookup category (or a name) as this kind. Matched whole.
  keywords: string[]
}

// Order matters: the first kind whose keyword matches wins, so the specific
// kinds come before the broad ones ("pet store" is pets, not shopping; "wine
// bar" is nightlife, not shopping; "post office" is services, not work). Home,
// Work and Friends are mostly picked by hand, so they are tried late: "funeral
// home" is services.
export const PLACE_KINDS: PlaceKind[] = [
  {
    id: 'gym', label: 'Gym',
    paths: ['M6.5 6.5v11M17.5 6.5v11M3.5 9v6M20.5 9v6M6.5 12h11'],
    keywords: ['gym', 'fitness', 'health club', 'sports centre', 'sports center', 'yoga', 'pilates', 'crossfit', 'climbing', 'swimming', 'swimming pool', 'boxing', 'martial arts', 'workout', 'sports club', 'tennis', 'pickleball'],
  },
  {
    id: 'fuel', label: 'Fuel',
    paths: ['M4 21V5a2 2 0 012-2h6a2 2 0 012 2v16M3 21h13M6.5 8h5M14 9h2.5A1.5 1.5 0 0118 10.5v6a1.5 1.5 0 003 0V8l-3-3'],
    keywords: ['fuel', 'gas station', 'service station', 'petrol', 'gas', 'charging station', 'ev charging', 'charger'],
  },
  {
    id: 'parking', label: 'Parking',
    paths: ['M5 3h14a2 2 0 012 2v14a2 2 0 01-2 2H5a2 2 0 01-2-2V5a2 2 0 012-2zM9 17V7h4a3 3 0 010 6H9'],
    keywords: ['parking', 'parking lot', 'parking garage', 'parking structure', 'car park', 'park and ride', 'cars', 'multi storey'],
  },
  {
    id: 'civic', label: 'Civic',
    paths: ['M3 3v1.5M3 21v-6m0 0l2.77-.693a9 9 0 016.208.682l.108.054a9 9 0 006.086.71l3.114-.732a48.524 48.524 0 01-.005-10.499l-3.11.732a9 9 0 01-6.085-.711l-.108-.054a9 9 0 00-6.208-.682L3 4.5M3 15V4.5'],
    keywords: ['government', 'town hall', 'townhall', 'city hall', 'courthouse', 'court', 'police', 'fire station', 'dmv', 'embassy', 'consulate', 'public service', 'military', 'prison', 'library', 'civic', 'municipal', 'council', 'customs', 'ranger station'],
  },
  {
    id: 'services', label: 'Services',
    paths: ['M12 21v-8.25M15.75 21v-8.25M8.25 21v-8.25M3 9l9-6 9 6m-1.5 12V10.332A48.36 48.36 0 0012 9.75c-2.551 0-5.056.2-7.5.582V21M3 21h18M12 6.75h.008v.008H12V6.75z'],
    keywords: ['bank', 'atm', 'credit union', 'post office', 'shipping', 'mailbox', 'insurance', 'lawyer', 'attorney', 'accountant', 'tax', 'estate agent', 'real estate', 'notary', 'laundry', 'laundromat', 'dry cleaning', 'tailor', 'locksmith', 'storage', 'self storage', 'copy shop', 'copyshop', 'printing', 'photo', 'travel agency', 'funeral', 'community centre', 'community center', 'social facility', 'charity', 'recycling'],
  },
  {
    id: 'worship', label: 'Worship',
    paths: ['M12 3v4M10 5h4M12 7l-5 4v10h10V11l-5-4zM10 21v-5a2 2 0 014 0v5'],
    keywords: ['church', 'mosque', 'synagogue', 'temple', 'place of worship', 'chapel', 'religion', 'cathedral', 'shrine', 'gurdwara', 'worship', 'monastery', 'religious'],
  },
  {
    id: 'school', label: 'School',
    paths: ['M4.26 10.147a60.436 60.436 0 00-.491 6.347A48.627 48.627 0 0112 20.904a48.627 48.627 0 018.232-4.41 60.46 60.46 0 00-.491-6.347m-15.482 0a50.57 50.57 0 00-2.658-.813A59.905 59.905 0 0112 3.493a59.902 59.902 0 0110.399 5.84c-.896.248-1.783.52-2.658.814m-15.482 0A50.697 50.697 0 0112 13.489a50.702 50.702 0 017.74-3.342M6.75 15a.75.75 0 100-1.5.75.75 0 000 1.5zm0 0v-3.675A55.378 55.378 0 0112 8.443m-7.007 11.55A5.981 5.981 0 006.75 15.75v-1.5'],
    keywords: ['school', 'university', 'college', 'kindergarten', 'education', 'campus', 'daycare', 'childcare', 'preschool', 'classroom', 'academy', 'tutoring', 'training', 'learning', 'language school'],
  },
  {
    id: 'health', label: 'Health',
    paths: ['M9 3h6v6h6v6h-6v6H9v-6H3V9h6z'],
    keywords: ['hospital', 'clinic', 'doctor', 'doctors', 'dentist', 'dental', 'pharmacy', 'chemist', 'healthcare', 'medical', 'optician', 'optometrist', 'urgent care', 'chiropractor', 'therapist', 'physiotherapist', 'physical therapy', 'drugstore', 'laboratory', 'pediatrics', 'dermatology', 'surgery'],
  },
  {
    id: 'pets', label: 'Pets',
    paths: ['M7 11a1.6 1.6 0 100-3.2A1.6 1.6 0 007 11zM17 11a1.6 1.6 0 100-3.2A1.6 1.6 0 0017 11zM10 8a1.6 1.6 0 100-3.2A1.6 1.6 0 0010 8zM14 8a1.6 1.6 0 100-3.2A1.6 1.6 0 0014 8zM12 12c-3 0-5.5 3-5.5 5.2 0 1.6 1.4 2.3 2.7 2.3 1 0 1.8-.5 2.8-.5s1.8.5 2.8.5c1.3 0 2.7-.7 2.7-2.3C17.5 15 15 12 12 12z'],
    keywords: ['pet', 'pets', 'veterinary', 'vet', 'animal', 'kennel', 'dog park', 'dog grooming', 'pet store', 'animal shelter', 'groomer', 'aquarium store'],
  },
  {
    id: 'beauty', label: 'Beauty',
    paths: ['M6 9a3 3 0 100-6 3 3 0 000 6zM6 21a3 3 0 100-6 3 3 0 000 6zM8.1 7.9L20 18M8.1 16.1L20 6'],
    keywords: ['beauty', 'salon', 'hairdresser', 'hair', 'barber', 'barbershop', 'spa', 'massage', 'nails', 'nail', 'tattoo', 'waxing', 'cosmetics', 'skincare', 'wellness'],
  },
  {
    id: 'travel', label: 'Travel',
    paths: ['M6 12L3.269 3.126A59.768 59.768 0 0121.485 12 59.77 59.77 0 013.27 20.876L5.999 12zm0 0h7.5'],
    keywords: ['airport', 'aerodrome', 'terminal', 'train station', 'bus station', 'bus stop', 'subway', 'metro', 'light rail', 'station', 'hotel', 'motel', 'hostel', 'resort', 'ferry', 'car rental', 'bicycle rental', 'bike rental', 'taxi', 'travel', 'lodging', 'accommodation', 'guest house', 'bed and breakfast', 'rest area'],
  },
  {
    id: 'auto', label: 'Auto',
    paths: ['M21.75 6.75a4.5 4.5 0 01-4.884 4.484c-1.076-.091-2.264.071-2.95.904l-7.152 8.684a2.548 2.548 0 11-3.586-3.586l8.684-7.152c.833-.686.995-1.874.904-2.95a4.5 4.5 0 016.336-4.486l-3.276 3.276a3.004 3.004 0 002.25 2.25l3.276-3.276c.256.565.398 1.192.398 1.852z'],
    keywords: ['car repair', 'auto repair', 'mechanic', 'tire', 'tires', 'tyre', 'tyres', 'car dealer', 'car dealership', 'auto parts', 'car parts', 'body shop', 'smog', 'oil change', 'car wash', 'automotive', 'motorcycle', 'car', 'vehicle', 'tow', 'dealership'],
  },
  {
    id: 'groceries', label: 'Groceries',
    paths: ['M2.25 3h1.386c.51 0 .955.343 1.087.835l.383 1.437M7.5 14.25a3 3 0 00-3 3h15.75m-12.75-3h11.218c1.121-2.3 2.1-4.684 2.924-7.138a60.114 60.114 0 00-16.536-1.84M7.5 14.25L5.106 5.272M6 20.25a.75.75 0 11-1.5 0 .75.75 0 011.5 0zm12.75 0a.75.75 0 11-1.5 0 .75.75 0 011.5 0z'],
    keywords: ['supermarket', 'grocery', 'groceries', 'market', 'greengrocer', 'convenience', 'butcher', 'farmers market', 'food store', 'deli', 'organic', 'food market', 'seafood', 'fishmonger', 'produce'],
  },
  {
    id: 'food', label: 'Food',
    paths: ['M7 3v7a2 2 0 002 2v9M7 3v5M11 3v5M11 8a2 2 0 01-2 2M17 3c-1.7 1.6-2.5 4-2.5 7 0 1.6.9 2.6 2.5 2.6V21'],
    keywords: ['restaurant', 'cafe', 'coffee', 'fast food', 'food', 'bakery', 'pizza', 'pizzeria', 'diner', 'donut', 'doughnut', 'ice cream', 'bbq', 'bistro', 'taco', 'burger', 'sandwich', 'juice', 'tea', 'dining', 'eatery', 'noodle', 'sushi', 'salad', 'grill', 'steakhouse', 'bagel', 'brunch', 'dessert', 'smoothie', 'food court', 'takeaway', 'chocolate', 'candy', 'confectionery'],
  },
  {
    id: 'nightlife', label: 'Nightlife',
    paths: ['M5 4h14l-7 8-7-8zM12 12v8M8 20h8'],
    keywords: ['bar', 'pub', 'nightclub', 'night club', 'brewery', 'taproom', 'lounge', 'wine bar', 'cocktail', 'tavern', 'distillery', 'karaoke', 'hookah', 'biergarten', 'winery'],
  },
  {
    id: 'outdoors', label: 'Outdoors',
    paths: ['M12 3l4.5 7H14l3.5 6H13v5h-2v-5H6.5L10 10H7.5L12 3z'],
    keywords: ['park', 'trailhead', 'trail', 'hiking', 'beach', 'campground', 'camp site', 'picnic', 'garden', 'botanical', 'nature', 'nature reserve', 'forest', 'lake', 'viewpoint', 'lookout', 'scenic', 'marina', 'pier', 'recreation area', 'state park', 'national park', 'playground', 'outdoors', 'dog run', 'skatepark', 'bike path'],
  },
  {
    id: 'leisure', label: 'Leisure',
    paths: ['M15.182 15.182a4.5 4.5 0 01-6.364 0M21 12a9 9 0 11-18 0 9 9 0 0118 0zM9.75 9.75c0 .414-.168.75-.375.75S9 10.164 9 9.75 9.168 9 9.375 9s.375.336.375.75zm-.375 0h.008v.015h-.008V9.75zm5.625 0c0 .414-.168.75-.375.75s-.375-.336-.375-.75.168-.75.375-.75.375.336.375.75zm-.375 0h.008v.015h-.008V9.75z'],
    keywords: ['cinema', 'movie', 'theatre', 'theater', 'museum', 'stadium', 'golf', 'bowling', 'arcade', 'amusement', 'zoo', 'entertainment', 'attraction', 'recreation', 'casino', 'gallery', 'arts', 'artwork', 'leisure', 'hobby', 'concert', 'music venue', 'comedy', 'escape room', 'theme park', 'water park', 'aquarium', 'sightseeing', 'monument', 'memorial', 'landmark', 'historic'],
  },
  {
    id: 'shopping', label: 'Shopping',
    paths: ['M15.75 10.5V6a3.75 3.75 0 10-7.5 0v4.5m11.356-1.993l1.263 12c.07.665-.45 1.243-1.119 1.243H4.25a1.125 1.125 0 01-1.12-1.243l1.264-12A1.125 1.125 0 015.513 7.5h12.974c.576 0 1.059.435 1.119 1.007zM8.625 10.5a.375.375 0 11-.75 0 .375.375 0 01.75 0zm7.5 0a.375.375 0 11-.75 0 .375.375 0 01.75 0z'],
    keywords: ['store', 'shop', 'mall', 'retail', 'clothes', 'clothing', 'electronics', 'hardware', 'doityourself', 'variety', 'department', 'mobile phone', 'furniture', 'books', 'gift', 'wholesale', 'shoes', 'jewelry', 'jewellery', 'florist', 'liquor', 'alcohol', 'wine', 'outlet', 'boutique', 'shopping', 'plaza', 'bicycle', 'bike', 'bed', 'mattress', 'toys', 'sports', 'computer', 'camera', 'stationery', 'craft', 'art supplies', 'thrift', 'antique', 'garden centre', 'garden center', 'dollar', 'general', 'kiosk'],
  },
  {
    id: 'work', label: 'Work', defaultName: 'Work',
    paths: [
      'M20.25 14.15v4.25c0 1.094-.787 2.036-1.872 2.18-2.087.277-4.216.42-6.378.42s-4.291-.143-6.378-.42c-1.085-.144-1.872-1.086-1.872-2.18v-4.25m16.5 0a2.18 2.18 0 00.75-1.661V8.706c0-1.081-.768-2.015-1.837-2.175a48.114 48.114 0 00-3.413-.387m4.5 8.006c-.194.165-.42.295-.673.38A23.978 23.978 0 0112 15.75c-2.648 0-5.195-.429-7.577-1.22a2.016 2.016 0 01-.673-.38m0 0A2.18 2.18 0 013 12.489V8.706c0-1.081.768-2.015 1.837-2.175a48.111 48.111 0 013.413-.387m7.5 0V5.25A2.25 2.25 0 0013.5 3h-3a2.25 2.25 0 00-2.25 2.25v.894m7.5 0a48.667 48.667 0 00-7.5 0',
    ],
    keywords: ['work', 'office', 'coworking', 'workplace', 'company', 'headquarters', 'corporate', 'business', 'it', 'studio', 'factory', 'industrial'],
  },
  {
    id: 'friends', label: 'Friends',
    paths: ['M15 19.128a9.38 9.38 0 002.625.372 9.337 9.337 0 004.121-.952 4.125 4.125 0 00-7.533-2.493M15 19.128v-.003c0-1.113-.285-2.16-.786-3.07M15 19.128v.106A12.318 12.318 0 018.624 21c-2.331 0-4.512-.645-6.374-1.766l-.001-.109a6.375 6.375 0 0111.964-3.07M12 6.375a3.375 3.375 0 11-6.75 0 3.375 3.375 0 016.75 0zm8.25 2.25a2.625 2.625 0 11-5.25 0 2.625 2.625 0 015.25 0z'],
    keywords: ['friend', 'friends', 'family', 'mom', 'dad', 'parents', 'relative', 'relatives', 'grandma', 'grandpa', 'sister', 'brother', 'aunt', 'uncle', 'cousin'],
  },
  {
    id: 'home', label: 'Home', defaultName: 'Home',
    paths: ['M2.25 12l8.954-8.955a1.126 1.126 0 011.591 0L21.75 12M4.5 9.75v10.125c0 .621.504 1.125 1.125 1.125H9.75v-4.875c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125V21h4.125c.621 0 1.125-.504 1.125-1.125V9.75M8.25 21h8.25'],
    keywords: ['home', 'residence', 'apartment'],
  },
  {
    // What nothing else matched. Drawn as a plain pin, deliberately: no guess.
    id: 'other', label: 'Unsorted',
    paths: ['M15 10.5a3 3 0 11-6 0 3 3 0 016 0z', 'M19.5 10.5c0 7.142-7.5 11.25-7.5 11.25S4.5 17.642 4.5 10.5a7.5 7.5 0 1115 0z'],
    keywords: [],
  },
]

// The order the editor offers kinds in: the everyday ones first. Unsorted is
// left out because choosing it is the same as choosing nothing.
export const PICKER_KIND_IDS: PlaceKindId[] = [
  'home', 'work', 'gym', 'leisure', 'food', 'nightlife', 'groceries', 'shopping',
  'fuel', 'parking', 'auto', 'health', 'beauty', 'pets', 'school', 'friends',
  'services', 'civic', 'worship', 'outdoors', 'travel',
]

const BY_ID = new Map(PLACE_KINDS.map(k => [k.id, k]))

export const PICKER_KINDS: PlaceKind[] = PICKER_KIND_IDS.map(id => BY_ID.get(id)!)

// Matches whole words, so "bar" does not match "barber" or "barn".
const MATCHERS = PLACE_KINDS.map(k => ({
  id: k.id,
  res: k.keywords.map(w => new RegExp(`(^|[^a-z0-9])${w.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}($|[^a-z0-9])`)),
}))

export function placeKind(id: string | null | undefined): PlaceKind {
  return BY_ID.get((id ?? 'other') as PlaceKindId) ?? BY_ID.get('other')!
}

function kindOfText(text: string | null | undefined): PlaceKindId | null {
  const t = (text ?? '').toLowerCase().trim()
  if (!t) return null
  for (const m of MATCHERS) if (m.res.some(r => r.test(t))) return m.id
  return null
}

// The kind a place belongs to. The category (a kind the user picked, or what a
// lookup said) decides first; the name is only a fallback, for places named
// "Anytime Fitness" with no category.
export function kindForCategory(category: string | null | undefined, name?: string | null): PlaceKindId {
  const c = (category ?? '').trim().toLowerCase()
  // A real category is final: "social facility" at "Westside Food Bank" is not a
  // restaurant. The name is only consulted when there is nothing better.
  if (c && c !== 'address' && c !== 'street') return kindOfText(c) ?? 'other'
  return kindOfText(name) ?? 'other'
}

// The kind with this exact label (what the picker stores), if any.
export function kindFromLabel(label: string | null | undefined): PlaceKindId | null {
  const l = (label ?? '').trim().toLowerCase()
  return PLACE_KINDS.find(k => k.id === l || k.label.toLowerCase() === l)?.id ?? null
}

// Inline SVG for the icon, for places that cannot render a component (map
// markers are plain HTML).
export function placeIconSvg(kindId: string | null | undefined, opts: { size?: number; color?: string; strokeWidth?: number } = {}): string {
  const { size = 16, color = 'currentColor', strokeWidth = 1.75 } = opts
  const paths = placeKind(kindId).paths.map(d => `<path d="${d}"/>`).join('')
  return `<svg xmlns="http://www.w3.org/2000/svg" width="${size}" height="${size}" viewBox="0 0 24 24" fill="none" stroke="${color}" stroke-width="${strokeWidth}" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${paths}</svg>`
}

// Orders names offered for a spot so those matching a kind hint come first,
// keeping the original order otherwise.
export function rankByKind<T extends { kind?: string | null }>(items: T[], hint: string | null | undefined): T[] {
  if (!hint || hint === 'other') return items
  return [...items.map((x, i) => ({ x, i }))]
    .sort((a, b) => Number(b.x.kind === hint) - Number(a.x.kind === hint) || a.i - b.i)
    .map(e => e.x)
}
