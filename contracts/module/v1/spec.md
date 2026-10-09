# module/v1 — modules

**Draft.** See [README.md](README.md) for the status and the gate it has not yet met.

Cairn's core is **GPS logging management**: capture a drive, seal it, get it off the card
against a verified receipt, decode it, keep it. A **module** is *interpretation* — what a
reading means — and it carries its dongle, server, web and app parts in one package.

## 1. The line

**Core. Never a module.**

- `format/v3`: frames, segments, manifest, Merkle, receipts, update descriptors. Record
  layouts are core; a module never defines a byte.
- Identity and authority: devices, clients, vehicles, assignments, counters, keystore,
  revocation.
- Transport and custody: BLE offload, relay, uplink, intake, CAS, outbox, ledger,
  receipts, receipt-gated prune.
- Decode of raw fields into the base tables of [`store/v1`](../../store/v1/).
- Trip identity and shape: `v_boot_start`, `v_drive_summary`, `v_trip_summary`,
  `v_telemetry`, `v_vehicles`, `v_reproducibility`; route, stops, timeline.
- Generic analysis machinery: `v_metric_samples`, `v_health_stats`, `v_tune_effect` —
  but **parameterised by module-contributed metrics** instead of a literal metric list.
- The shell: auth, nav, vehicle selector, trip list, device page, export and import.

**Module.** Boost, fuel economy, fuel mixture and trims, driving style, speedometer check,
place kinds, and whatever comes next.

Three borderline calls, settled here so they are not re-argued per module:

- **Stop detection is core; what the stop *was* is a module.** Knowing the car stopped is
  part of the trip's shape. Deciding it was a gym is interpretation.
- **Tune records are core; tune interpretation is a module.** The vehicle registry is the
  one place a tune is written. "A tune is not a fault" is a rule about trims.
- **A base table is core; a derived column on it may be a module's** (§4).

## 2. What a module may not do

A module never: sees plaintext bundle bytes, holds or derives a key, influences whether a
receipt verifies, influences a prune, adds a network listener, or writes to the store.

**This is structural, not a convention.** Both schemas set `additionalProperties: false`
throughout and there is no field through which any of it could be asked for. A module
cannot be given a capability the manifest cannot spell.

### Not the retired plugin system

Arbitrary user plugins were retired on 2026-10-05 and remain deferred. The difference:

| Retired plugins | Modules |
|---|---|
| Shipped executable code (WASM) to a host | Declarations only: manifests, SQL, source compiled in |
| Loaded at run time by the server | Validated at startup; firmware and UI parts generated and compiled |
| Third-party, arbitrary | First-party, in one audited repository, released by tag |
| Could touch the raw path | Cannot — §2 |
| Invisible in the store digest | Folded into store identity and reproducibility (§7) |

**No new interpreter is added on any host.** Derived values are SQL expressions evaluated
by DuckDB, which the server already runs. The only expression VM involved anywhere is the
one the firmware already has for `engine/v1` formulas.

## 3. Identity

A module is a directory. [`module.schema.json`](module.schema.json) states the manifest's
structure. What a schema cannot state, and a validator must therefore enforce:

- **`id` equals the directory name.** Otherwise a module can be selected under a name
  that is not in it.
- **Exactly one module owns each derived column** (§4).
- **Derivation order has no cycle.** A module whose `requires.store` names another
  module's derived column is ordered after it; a cycle is an error.
- **Every `requires.store` entry exists** in [`store/v1`](../../store/v1/)'s `schema.json`,
  or is a column some module in the set derives.
- **A metric's `source_view` is one this module declares**, and `source_column` exists in
  it.
- **Every view a module creates is named `v_<id>_*`**, or is a grandfathered name listed
  in `store/v1`.
- **Every declared query parameter appears as `$name` in its SQL, and every `$name` in the
  SQL is declared.** Both directions, so a query can neither ignore an argument nor read
  an unbound one.
- **A query is one `SELECT`** with no semicolon outside a string literal.
- **A stub claims nothing** beyond identity and `sources`.

`status` and `sources` carry the same meanings as in [`engine/v1`](../../engine/v1/): a
`stub` names the thing and claims nothing, `derived` was extracted from code that already
worked, `verified` was checked against the real car, and every claim has a source.

A module may be **nothing but a manifest** — no metric, no engine field, no derivation.
That is a legitimate module (a page over data the core already holds), and it is the first
vector a validator should be held to.

## 4. Derived columns

A module may define a column on a core table. The value is a **SQL scalar expression over
the columns of that row's own table**, evaluated by DuckDB during load.

- **Exactly one module owns a column.** Two owners would make the value depend on load
  order.
- A derivation **cannot write**, cannot read another table, and cannot aggregate. Anything
  that needs more than one row is a view.

### An absent module: introduced against grandfathered

A derived column is one of two kinds, and they behave differently when no module provides
the definition.

| | **Introduced** by a module | **Grandfathered** |
|---|---|---|
| Declared by `store/v1`? | no — the module adds it | yes |
| With the module present | the module's expression | the module's expression |
| With the module absent | the column exists and is **all-null** | **the core's definition stands** |

An introduced column being all-null is exactly what the store already says about a car
that never reported the input, and it is what lets `store/v1` stay honest under any module
set.

A grandfathered column must **not** go null, because something already depends on it. The
core computed it before any module existed, and nulling it when an operator happens to run
without a module set would break every deployment and every query that reads it. So the
core's definition stands until a module takes over, and the module's expression then
replaces it — identically, which is what the proof below is for.

This is the only place a module's absence is not simply "that interpretation is missing",
and it is the price of letting interpretation move out of a core that already shipped.

### The two grandfathered columns

`boost.boost_psi` and `boost.lambda_ratio` are declared by `store/v1` and computed today in
the server's decode path. A module may take over their *definition*, but the column, its
type and its values must not change — **proven, not inspected**. The reference
implementation in the server's `format` package stays as the thing the `format/v3` vectors
pin; the module's SQL is tested against it, not instead of it.

"Proven" means evaluated: the expression is run by the same engine that will run it in
production, against the reference, over every input the record can hold. Reading two
expressions and judging them equivalent is how a stored column changes silently. It has
already happened once here — an earlier draft of this contract's own `valid-boost.json`
carried `map_kpa < 255 AND baro_kpa > 0`, which looks like a tightening and is in fact a
different column: `BoostPSI()` returns a value whenever both readings are present, and
saturation is excluded by the *views* that read the column, not by the column. Every
saturated sample's boost would have become `NULL`.

## 5. Metrics

A metric is what the generic machinery needs to treat a module's readings like any
other's: a key, a label, a unit, what one observation is, and where to read it.

`sample` is a **closed enum** — `sample`, `pull`, `boot`, `trip`. It decides how
`v_metric_samples` counts, and "enough observations to judge" means nothing if a module
can invent its own unit. Boost is per-pull and the trims are per-boot, so two is already
the minimum.

A metric's `key` is also an `engine/v1` analysis `signals[].key`. That is the whole
mechanism by which a reading finds its limits, and it preserves the rule those limits
exist for:

**A metric with no matching signal is shown without a verdict.** It is never called
normal, because nothing says what normal is. It may still be judged against the car's own
earlier readings, which needs no knowledge of the engine at all.

### Contribution points

`trip_insight`, `dashboard_highlight` and `ios_gauge` exist because the shell has pages
that display module-owned metrics — a trip's insight list, the dashboard highlights, the
app's live gauge grid. Without somewhere for a module to contribute, moving its code out
of the core fails at exactly those pages, and they are the ones a reader sees first.

## 6. Named queries

A module's queries are declared, named and parameterised
([`queries.schema.json`](queries.schema.json)), and reached as `POST /q/<module>/<name>`.

Parameters are **bound, never interpolated**. The web layer today splices a
regex-validated vehicle id into SQL strings and keeps a generated inventory of 88 of them;
binding removes that surface by construction rather than by care, and the inventory stops
being something a person maintains.

`vehicle_id` and `boot_id` are their own parameter types because they are the two the
store is partitioned by, and the two most likely to arrive from a URL.

Where a query relies on an ASOF join it states `age_threshold_ms`. **An ASOF join always
finds something**, so staleness must be an explicit predicate and never a default — the
rule `store/v1`'s own views already follow, carried into anything a module asks.

## 7. Availability, identity, reproducibility

### Three gates, in order

| Gate | When | Asks | When it fails |
|---|---|---|---|
| **Engine** | build / flash | does the engine profile provide `requires.engine_fields`? | the build fails |
| **Data** | run time, per vehicle | does *this car* have rows? | the module is hidden, not an empty page |
| **Knowledge** | run time, per engine | are limits known for this engine? | readings shown, verdicts withheld |

A module **states** an engine-field requirement. It cannot make a dongle poll a PID the
engine profile does not declare; it can only be told no, at build time, on a workstation.
With one dongle and no spare, that asymmetry is the whole safety argument: the failure
mode is a build error, never a changed polling loop in a car.

### Identity

A module set changes what a derived store contains, so by the project's fifth invariant —
*anything derived is disposable and must prove it reproduces* — the module set is part of
that store's identity.

- A module hashes to SHA-256 over **its manifest and every file the manifest names** —
  `views[]`, `queries`, and any later key whose value is a path — in that order: the
  manifest first, then the named paths sorted. Each contributes its repo-relative path,
  a `0x00` byte, its bytes with CRLF read as LF, and a `0x00` byte.
- The **module set identity** is SHA-256 of `cairn.module-set/v1-draft\n` followed, for
  each module in `id` order, by `<id>\n<version>\n<hex hash>\n`.

**Only the declarations, not the directory.** A module's hash covers what can change its
output and nothing else. A `README.md`, a note, a scratch file — none of them can change
what a derivation computes or what a query returns, so none of them moves the hash. The
earlier rule hashed the whole directory, which made a prose fix look like a different
module and therefore a different store; that is a false mismatch, and invariant 5 is worth
less every time it cries wolf.

"The manifest plus every file the manifest names" needs no "except documentation"
carve-out, and it extends by itself: a key added later whose value is a path is covered the
day it is added, with nothing to remember.

A consumer reports it: the server folds it into the store contract digest and into
`v_reproducibility`, so a rebuild under a different module set is **visibly** a different
rebuild rather than a silent mismatch. The firmware compiles it in and reports it. This
mirrors what `engine/v1` already does with `cairn.engine-build/v1-draft`, for the same
reason.

## 8. Relationship to the other contracts

| Module manifest | Points at |
|---|---|
| `requires.engine_fields[]` | the `field` enum in [`engine/v1`](../../engine/v1/)'s `acquisition.schema.json`. The two enums must stay identical and a validator compares them |
| `metrics[].key` | a `signals[].key` in an `engine/v1` analysis profile |
| `requires.store[]`, `derives[].column` | tables and columns of [`store/v1`](../../store/v1/) |

A module adds no wire format. It never appears in `format/v3`, `ble/v1`, `enrolment/v1` or
`uplink/v1`, because none of those carry interpretation.

## 9. Vectors

See [`vectors/README.md`](vectors/README.md): valid manifests including the
manifest-only case, invalid manifests covering each §3 rule, and the query-file cases.
