# Modules v1 (draft)

Cairn's core is **GPS logging management**: capture a drive, seal it, get it off the card
against a verified receipt, decode it, keep it. A **module** is *interpretation* — what a
reading means — and it carries its dongle, server, web and app parts in one package.

[`spec.md`](spec.md) is the normative text. [`module.schema.json`](module.schema.json) is
the manifest; [`queries.schema.json`](queries.schema.json) is a module's named queries.

- **Status:** draft. Identified in-document as `cairn.module/v1-draft`; nothing implements
  it yet.
- **Producers:** a maintainer writes a module by hand, in
  [cairn-modules](https://github.com/ParkWardRR/cairn-modules) (created 2026-10-08; see
  [the module system plan](../../../docs/module-system-plan.md)).
- **Consumers:** the server loads manifests and applies derivations, views and queries;
  the web layer builds its nav, routes and queries from them; the app generates metric
  descriptors; the firmware's generator resolves `requires.engine_fields`.

## Why this exists

The project has carried an acceptance test since the rebuild and never met it:

> **Adaptability** — a new vehicle, a new place kind or a new metric can be added by
> configuration or a documented extension point, not by editing core code.

Today, adding a metric means editing a core view, a core decode path, a core nav array, a
core scope table and a generated inventory of 88 SQL strings. Six of the dashboard's
twelve pages are interpretation living inside the core, and `v_metric_samples`,
`v_health_stats` and `v_tune_effect` are generic machinery wrapped around a hard-coded
`VALUES ('boost_psi'), ('lambda'), ('ltft_pct'), ('stft_pct')`. That literal is what a
module contributes.

## This is not the plugin system that was retired

Arbitrary user plugins were retired on 2026-10-05 with the MoonBit/WASM system, and
remain deferred: they "would need the raw format, decoder ABI, receipt semantics and
replay tooling to be stable first". That judgement stands.

| Retired plugins | Modules |
|---|---|
| Shipped executable code (WASM) to a host | Declarations only: manifests, SQL, source compiled in |
| Loaded at run time by the server | Validated at startup; firmware and UI parts generated and compiled |
| Third-party, arbitrary | First-party, in one audited repository, released by tag |
| Could touch the raw path | Cannot — see below |
| Invisible in the store digest | Folded into store identity and reproducibility ([spec §7](spec.md#7-availability-identity-reproducibility)) |

**A module never** sees plaintext bundle bytes, holds or derives a key, influences whether
a receipt verifies, influences a prune, adds a network listener, or writes to the store.
That is **structural**: both schemas set `additionalProperties: false` throughout and there
is no field through which any of it could be asked for. A module cannot be granted a
capability the manifest cannot spell.

**No new interpreter is added on any host.** Derived values are SQL expressions evaluated
by DuckDB, which the server already runs; the only expression VM involved anywhere is the
one the firmware already has for [`engine/v1`](../../engine/v1/) formulas.

## What is checked, and by what

| Check | Where |
|---|---|
| Manifest and query-file structure | [`module.schema.json`](module.schema.json), [`queries.schema.json`](queries.schema.json) |
| The rules a schema cannot state ([spec §3](spec.md#3-identity)) | `tools/modulecheck` in this repository |
| `requires.store` entries exist | resolved against [`store/v1`](../../store/v1/)'s `schema.json` — a typo fails validation, not a page |
| `requires.engine_fields` are real capture fields | resolved against [`engine/v1`](../../engine/v1/)'s acquisition `field` enum |
| The two engine-field enums have not drifted | `CheckVocabulary`, run in CI. `engine/v1` exists because two files in two places drifted; this is the guard against repeating it with three |
| One owner per derived column, no cycle in derivation order | `CheckSet`, with unit tests — the valid vectors form an acyclic set, so these need fixtures as well as vectors |

Run it with `go run ./cmd/module-check` from `tools/`.

## The release gate

Nothing implements this yet, so it cannot leave draft. Before it can:

- [x] **`cairn-modules` exists**, with `modgen` validating and emitting the per-surface
      artefacts. **No real module yet**: the one module in it is a manifest and nothing
      else, which is deliberate at this stage but leaves the `derives`, `metrics`, `views`
      and `queries` halves of this contract exercised only by the vectors below.
- [x] **A second, independent implementation.** `tools/modulecheck` here, in Go, and
      [`modgen`](https://github.com/ParkWardRR/cairn-modules/tree/main/tools/modgen) in
      Rust, written from [`spec.md`](spec.md) rather than ported from the Go. Both pass
      all 39 vectors. Writing the second one found a real defect in these vectors:
      `invalid/bad-status.json` had encoded one implementation's exact phrasing, which a
      substring match is meant to avoid.
- [ ] **The grandfathered derivations prove byte-identical.** `boost.boost_psi` and
      `boost.lambda_ratio` are computed in the server's decode path today; a module may
      take over their definition only against the committed `format/v3` conformance
      vectors ([spec §4](spec.md#4-derived-columns)).
- [ ] **Module-set identity is folded into the store digest and `v_reproducibility`**
      ([spec §7](spec.md#7-availability-identity-reproducibility)). This must land with
      the first real module, not after: a derived store whose contents depend on an
      unrecorded module set cannot prove it reproduces, and that is invariant 5.
- [ ] **The `nuxt` and `swift` emit targets.** `modgen` has `fields` and `go`; the other
      two land with the web and app phases, when there is a consumer to shape them. Until
      then the `ui` and `ios` keys of this contract are validated but never rendered.
- [ ] **A decision on `ui.nav.icon`.** It names an icon a surface must already have, which
      couples a contract to a web asset set. It may belong in the module's own UI layer
      instead.

## Known coverage gaps

- **One vector is YAML**, `vectors/invalid/bad-duplicate-key.yaml`. A duplicate mapping
  key cannot be written in JSON, and many YAML parsers accept one silently, keeping the
  last value — so a review can see `status: verified` while the loader reads
  `status: stub`. The Go checker reads the JSON vectors and skips it; any YAML-based
  implementation must reject it.
- **A view's SQL is not parsed.** `tools/modulecheck` holds manifests, not module
  directories, so it checks that a metric's `source_view` is named for its module and
  cannot check that the SQL actually creates it. `modgen`, which will have the files,
  must.

## Support window

Not applicable while draft: a draft protocol may change in place. Once released, a module
manifest is read by the server, the web layer and the generator — all of which release
together — so it is not subject to the firmware's long retention rule unless
`requires.engine_fields` ends up compiled into a shipped image.

## Files

| File | What it is |
|---|---|
| [`spec.md`](spec.md) | Normative. The core/module line, what a module may not do, identity, derived columns, metrics, named queries, the three availability gates |
| [`module.schema.json`](module.schema.json) | JSON Schema for the manifest |
| [`queries.schema.json`](queries.schema.json) | JSON Schema for a module's named, parameterised queries |
| [`vectors/`](vectors/) | 4 valid manifests, 23 invalid, 1 valid query file, 11 invalid, and the one YAML case |
