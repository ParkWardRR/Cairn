# Places

The trip page marks where the car stopped, and the Places page gathers those
stops, and every trip start and end, into places you can name and correct.

## Where the data lives

Everything below is in `/var/lib/cairn-ui/` on the server (`NUXT_PLACES_DATA_DIR`).

| File | What | Safe to delete? |
| --- | --- | --- |
| `saved-places.sqlite` | Places you named, places the system learned, and spots you rejected | **No.** Nothing else holds these |
| `saved-places.json` | The same, rewritten after every change | No: it is how the database is restored |
| `backups/saved-places-*.json` | A copy kept at most hourly, newest 30 | Yes, but they are your history |
| `places.sqlite` | Lookup cache and the visit history | Yes. Names are fetched again; visits are rebuilt from the trips still in cairn-tsdb |

If `saved-places.sqlite` is lost, the next start restores it from
`saved-places.json`. For a copy that survives the server, run
`deploy/backup-places.sh` from a laptop (it only reads), or use Export on the
Places page. Import merges a file back in and skips places already there.

These files hold real locations. They are not in the repository and must not be.

## How a place gets its name

1. **A place you saved or learned** covers every visit inside its circle, past
   and future, and is used before anything else. Nothing about it is sent to a
   lookup service.
2. Otherwise the name comes from the sources below, cached after the first lookup:
   OpenStreetMap (Overpass), Geoapify (needs a key) and a local Foursquare
   extract (`cairn-fsq`). A place both Foursquare and OSM agree on scores higher.

## Learning

When trips are ingested the server records each trip's stops, start and end in a
visit history. A spot that appears on **3 or more trips**, has a specific name
(a business or area, not a street or a bare address) and a confidence of at least
0.5 is saved automatically as **learned**, and shows a Learned badge.

- Edit a learned place, or press Confirm, and it becomes yours.
- Remove a place and that spot is remembered as rejected, so it is not learned
  again.

## What runs when trips arrive

```
bundle -> intake -> CAS -> outbox -> decode -> cairn-tsdb
                                                   |
                          (UI server, every minute, only when trips changed)
                                                   v
             record visits -> name places -> learn repeat places
```

The ingest is idempotent: it replaces a trip's visits as a whole, and trips that
later disappear from cairn-tsdb keep the visits already recorded.

## Kinds and icons

Every place has a kind: Home, Work, Gym, Fuel, Health, School, Travel, Groceries,
Food, Leisure, Shopping, Friends, or Other. The kind sets the icon on the map
markers, the place list and the trip's stop list, and the catalogue lives in one
file, `ui/shared/utils/placeKinds.ts`, shared by the app and the server.

Picking a kind in the editor is a hint the engine uses:

- it ranks the nearby names so the ones that look like that kind come first
  (choose Gym and the fitness centres move to the front);
- it fills in a name when there is none (Home, Work);
- it is stored with the place, so learned and saved places keep their icon;
- for places nobody has tagged, the kind is worked out from the category the
  lookup returned (`supermarket` is Groceries, `fitness centre` is Gym).

The engine also suggests Home: the spot trips most often start from and end at,
on at least 3 trips. It only suggests, and stops once a Home exists.
