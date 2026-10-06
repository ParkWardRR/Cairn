# Repository split: plan and status

> **Executed on 2026-10-05.** Phases 0 to 7 are done, production runs from the new repositories, and the old repository is preserved read-only. What remains is recorded as issues, listed below. The plan that follows the status block is kept **verbatim** (revision 4, including the owner's opening notes and inline answers) as the record of what was decided and why.

## Outcome by phase

| Phase | Outcome |
|---|---|
| 0. Freeze, baseline, rules | Tag `split-baseline`; local backup mirror; path map with a coverage check over every tracked path; extraction tooling with verification (final tree, 14 sampled historical trees, modes, authors); trusted-code policy on the runner (fork approval for every external contributor, same-repo guard, no `pull_request_target`); history scan found only labelled public test keys |
| 1. Boundary preparation | `contracts/` created and resolved through `$CAIRN_CONTRACTS`; no `../` path crosses a future boundary; route inventory; tag `monorepo-final` on a green CI run |
| 2. Rehearse | All three extractions verified and built locally |
| 3. Publish | [cairn-vehicle-server](https://github.com/ParkWardRR/cairn-vehicle-server), [cairn-vehicle-web-dashboard](https://github.com/ParkWardRR/cairn-vehicle-web-dashboard), [cairn-esp32-device-firmware](https://github.com/ParkWardRR/cairn-esp32-device-firmware): history preserved, one commit per post-extraction rewrite, `MIGRATION.md` provenance, CI green on a runner per repository, issues transferred, web staged acceptance suite (27 tests over the demo store) |
| 4. Contracts | `contracts.yml`, `docs.yml`; first release tag `contracts-v0.1.0` (protected); every component pinned by tag **and** commit |
| 5. Cross-repo checks | Required interop job in the server repo against a pinned firmware; scheduled diagnostic run in the firmware repo; eight drills (deliberately breaking changes) each failed only the right job; all runners busy at once peaked at 3.4 of 7.5 GB under one memory cap; fork-PR settings verified (a live test needs a second account) |
| 6. Production cutover | Cold snapshot with a **tested restore** (booted on spare ports and compared with live), server then web deployed from clean HEAD, every read route verified over HTTPS, data byte-identical |
| 7. Archive, front door, renames | [cairn-original-monorepo-archive](https://github.com/ParkWardRR/cairn-original-monorepo-archive) created from `monorepo-final` and archived; front door reduced and rewritten; `Cairn` renamed `cairn-driving-log-selfhosted`, the iOS repository `cairn-ios-companion-app` |
| 8. Retire the local monorepo | Partly: new working copies, memory and runner registration done; moving the old checkout is the owner's call (see the owner issue) |
| 9. Contract hardening | Not started by design; tracked in an issue |

## Deviations from the plan

- `MIGRATION.md` is the first commit **after** the extracted history, not literally the first commit (a root commit before filtered history would rewrite every hash).
- The contracts pin was `monorepo-final` until the first release tag existed.
- The plan lists four "reasons to use it" where phase 7 says five; the README adds "private by design" as the fifth.
- The server crash-recovery script uses only server code, so it is a plain server CI job, not interop.
- `deploy-v3.sh` gained `--build-only`; the web deploy scripts take the host from the environment or a gitignored `deploy.env`.

## Follow-up work

Each issue is written so an agent can pick it up cold.

| Where | Issues |
|---|---|
| Front door | [Theme 1 History](https://github.com/ParkWardRR/cairn-driving-log-selfhosted/issues/9), [Theme 2 Vehicle insight](https://github.com/ParkWardRR/cairn-driving-log-selfhosted/issues/10), [Theme 3 Sharing](https://github.com/ParkWardRR/cairn-driving-log-selfhosted/issues/11), [Theme 4 Approachability](https://github.com/ParkWardRR/cairn-driving-log-selfhosted/issues/12), [Theme 5 Data control](https://github.com/ParkWardRR/cairn-driving-log-selfhosted/issues/13), [Phase 9](https://github.com/ParkWardRR/cairn-driving-log-selfhosted/issues/14), [pin table](https://github.com/ParkWardRR/cairn-driving-log-selfhosted/issues/15), [owner cleanup](https://github.com/ParkWardRR/cairn-driving-log-selfhosted/issues/16), [history hostnames decision](https://github.com/ParkWardRR/cairn-driving-log-selfhosted/issues/17), [split tooling](https://github.com/ParkWardRR/cairn-driving-log-selfhosted/issues/18), plus the older [#1](https://github.com/ParkWardRR/cairn-driving-log-selfhosted/issues/1) and [#7](https://github.com/ParkWardRR/cairn-driving-log-selfhosted/issues/7) |
| Server | [issues 10 to 20](https://github.com/ParkWardRR/cairn-vehicle-server/issues) (health identity, schema, vectors, history views, tune record, data control, snapshot tooling, v2 to v3 migration, optional S3, first real trip) |
| Web | [issues 1 to 12](https://github.com/ParkWardRR/cairn-vehicle-web-dashboard/issues) (SQL passthrough security, error semantics, the product themes, deploy checks, schema CI, fsq, snapshot) |
| Firmware | [issues 4 to 12](https://github.com/ParkWardRR/cairn-esp32-device-firmware/issues) (first over-the-air offload, BLE passkey and session auth, Wi-Fi residue, flash encryption, C-writer cross-check, multi-phone, current draw, OTA, emulator relay mode) |
| iOS | [issues 16 to 24](https://github.com/ParkWardRR/cairn-ios-companion-app/issues) (rename follow-ups, vector CI, sync/v1, multi-phone, standalone mode, approachability, share sheet, data control, CarPlay) |

---

# Split the Cairn monorepo (revision 4)

keep this repo name for ios cairn-companion-ios-esp32-obd2-gps-ble
and i do want v2 -> v3 datamigration.


server is optinal. Should be able to be run via ios app + dongle. lets reduce ios complexity if possibel and do the heavy lifting on the sverer. 

Consdier mutli car and multi phones (on different and same car) 

carplay future - For a full CarPlay app running from your own iPhone in your car, you also need Apple to approve the relevant CarPlay entitlement; “personal use only” does not bypass that requirement

For now - If you only want a small status display or a few interactions, an iPhone app with a widget may be sufficient instead of a full CarPlay app. Apple says small widgets and Live Activities can appear automatically in CarPlay without changes; touchscreen vehicles also support widget interaction. That is a separate route from building a category-specific CarPlay app.

Btw install Rust Garage on the server as minio/s3 alterantive on the server. 

## Goal

Three new repositories and a front door that also holds the shared contracts, next to
the existing iOS repo. Each can be built, tested, released and deployed alone; what they
share is explicit, versioned, machine-checked and pinned by commit.

| Role | Repository | Change |
| --- | --- | --- |
| Front door and contracts | `cairn-driving-log-selfhosted` | **rename** the existing `Cairn`; keeps system docs, adds `contracts/` |
| Server | `cairn-vehicle-server` | new extraction |
| Web dashboard | `cairn-vehicle-web-dashboard` | new extraction |
| Firmware and emulator | `cairn-esp32-device-firmware` | new extraction |
| iOS | `cairn-ios-companion-app` | **rename** the existing `cairn-companion-ios-esp32-obd2-gps-ble` |
| Archive | `cairn-original-monorepo-archive` | read-only mirror of the monorepo |

Five active repositories and one archive. No standalone contracts repository: the
protocol collection is called **Cairn Vehicle Data Protocols** in its README and lives
in the directory `contracts/`.

## Product direction

What the project is for. This is the context the split serves; it does not change any
phase of the split.

| Area | Your direction |
| --- | --- |
| Driving history | GPS records, annual statistics, revisiting interesting routes |
| Sharing | Share selected trips with others |
| Vehicle insight | Monitor engine health and how a tune is behaving |
| Audience | You first, then other automotive enthusiasts; approachable enough for less technical drivers |
| Reasons to use it | Useful automatic capture, detailed vehicle information, control of data, and adaptability |

### What exists today, and the gap

Checked against the code on 2026-10-05, so the roadmap starts from fact.

| Area | Built | Gap |
| --- | --- | --- |
| Driving history | GPS capture and trip storage; trip detail with the route, a replay scrubber, stops and named places; a heatmap; a learned-places history. Dashboard totals for all time, month, week and today | **No per-year/month/week/6 month/quarter/user csutom date range statistics.** No way to bookmark, tag or search a route, so "revisit an interesting route" means scrolling the trip list |
| Sharing | None per trip. The only exports are the whole-store Parquet snapshot and the saved-places JSON | Everything: a way to pick trips, a format, and a privacy model for what leaves the house (see below) |
| Vehicle insight | Pages for Boost & Power, Fuel & Tune (ethanol blend, fuel flow, trim health), Engine Analytics, Economy, Drive Behavior and speedometer calibration; per-vehicle scoping; device health | **No stock/after-tune comparison and no baselines or trends over time.** The pages show what is happening, not whether it is normal or has changed since the tune |
| Audience | Built for the owner: the page names and metrics are expert ("Deep-dive telemetry viewer") | No plain-language tier needed. No "is my car healthy?" summary a non-technical driver could read. Setup docs assume a homelab |
| Reasons to use it | Automatic capture (dongle, with the phone as the relay); detailed OBD data; self-hosted with no cloud; export and import for saved places; multiple vehicles | Control of data is only partly there: every store that holds user data (trips, places, annotations) needs export, import and backup, not just places. The GitHub description still says "No phone", which stopped being true when the phone became the dongle's uplink |

### What this means for the split

None of this is in phases 0 to 8. The table says where each area will land and which
boundary it touches, so the split does not make any of it harder.

| Direction | Lands in | Boundary it touches |
| --- | --- | --- |
| Driving history (annual stats, route bookmarks) | web (views, pages); server (new store views such as a yearly summary) | New store views are an **additive** change to `contracts/store/v1`, so the web contract test covers them from day one |
| Sharing | web (choose trips, export, preview); server (anything that issues or checks a share token); iOS (share sheet) | A new **`contracts/share/v1`**: a portable trip format with redaction metadata. Reserve the directory now as `draft`. The threat model gets a sharing section **before** any design |
| Vehicle insight | server (derived health and tune metrics, per-vehicle baselines); web (health and tune views); firmware (only if more PIDs are needed) | Needs a **tune record**: when the tune changed, set by the user, stored per vehicle, so "since the tune" has a meaning. Fits the existing per-vehicle model |
| Audience | the front door README (the plain-language entry point); web (simple view by default, detail on request); iOS (the approachable surface) | Each repo's README opens with one plain paragraph on who it is for and where to start |
| Reasons to use it | the front door README states them; each is an acceptance test (below) | Export, import and backup for every user-data store; no mandatory cloud |

### The split must not foreclose

- **Trip identity must be stable and portable.** Sharing selected trips needs an id that
  survives export and import. Today it is the boot id; keep whatever the extraction
  leaves, and do not change it during the split.
- **Per-vehicle scoping stays** (`vehicle_id` through the store and the API). Insight and
  sharing are both per vehicle.
- **The store contract stays additive.** New views get added; existing ones are not
  repurposed.
- **Every store keeps an export path.** The split must not move user data somewhere an
  export cannot reach.
- **The web server layer keeps working with no cloud.** Sharing starts as a file the
  owner sends, not a hosted service.
- **Privacy zones are a first-class idea** (a start, end or place that is blanked before
  anything is shared), so the place engine and saved places must stay reachable from the
  export path.

### Reasons to use it, as acceptance tests

| Reason | A release is not done until |
| --- | --- |
| Useful automatic capture | a drive is captured and visible without opening any app |
| Detailed vehicle information | the OBD detail is browsable per trip and per vehicle |
| Control of data | every store holding user data can be exported, imported and backed up, and nothing requires an account or a cloud service |
| Adaptability | a new vehicle, a new place kind or a new metric can be added by configuration or a documented extension point, not by editing core code |

### Proposed order for the product work (yours to change)

After the split, seeded into the front door's `ROADMAP.md` as five themes, each with an
owning repo and the contract it touches:

1. **History**: annual statistics and route bookmarks. Smallest, web plus store views,
   and it exercises the new contract flow end to end.
2. **Vehicle insight**: the tune record, baselines and a health summary. The biggest
   differentiator for you, and it needs the tune record before anything else.
3. **Sharing**: needs the privacy design and `share/v1` first, so it follows the first two.
4. **Approachability**: a plain-language pass on the web and the READMEs, run alongside the
   others rather than at the end, since it is mostly copy and defaults.
5. **Data control**: export, import and backup for trips and annotations, as each store
   gains user data. Render nice shareable high res images. 
   6. GPU accleraiton? 

---

## Non-goals

- No behaviour changes in the split itself. That promise is only checkable if the split
  is mechanical, so new protocol work is deferred to phase 9. Document any changes as repo issues.
- No new hosting. The self-hosted runner and the VM stay. If needed you can install Garage on the server as minio/s3 alterantive.

## Decisions

| # | Decision | Status |
| --- | --- | --- |
| 1 | iOS repo and the front door | existing companion repo stays the iOS repo; `Cairn` stays live as the front door | decided |
| 2 | Names | the six above | decided (second round) |
| 3 | History | keep it, extracted by path | decided |
| 4 | Shared specs and vectors | **in the front door's `contracts/`**, not a separate repo | decided (second round; supersedes the earlier "fifth repo" choice) |
| 5 | Extraction tool | plain git, as a tested script (see "Extraction"). `git filter-repo` is what git's own docs recommend but it is a Python script; using it for a one-off is a deliberate exception you would have to approve | **open** |
| 6 | Renaming `Cairn` | renaming keeps the issues, stars and history, and GitHub redirects the old URL, **but only while no other repo takes the name `Cairn`**, and your earlier instruction was to keep `…/Cairn/issues`. The rename is the last step of phase 7 and can be skipped without changing anything else | **open** |

### Consequences of merging contracts into the front door

- Protocol releases share history with documentation edits. Accepted, because it removes
  a repository to administer. The separation is by **release rules**, not repositories:
  - contract releases are tagged `contracts-vX.Y.Z` and only when contract content changes
  - documentation edits merge normally with no release
  - contract CI and docs CI are separate workflows
- Issues stay in the front door when they are about the system or a protocol;
  implementation issues move to the component repo.

---

## Live inventory (checked 2026-10-05; recount at the freeze)

| Item | Value |
| --- | --- |
| Monorepo | 153 commits, 8.7 MB; tracked files: server 128, ui 115, firmware 92, fixtures 78, docs 31, emulator 25, deploy 17, research_notes 5 |
| `Cairn` open issues | **7**: #1 publish app-sync-protocol with vectors, #2 authenticated sync API, #3 revocation and admin identity, #4 snapshot behind auth, #5 bundle relay from an authenticated app, #6 dongle BLE bundle offload (firmware), #7 retire the device mTLS listener, certificates and network OTA |
| iOS open issues | **13** (#1 to #7, #9 to #14), including #14 BLE bundle offload and #12 golden vectors |
| Root of `Cairn` `main` | `.github .gitignore INSTALL.md LICENSE README.md ROADMAP.md deploy docs emulator firmware fixtures reports research_notes server tests ui`. The legacy stack (`gleam`, `zig`, `odin`, `rust`, `plugins`, `mojo`, `moonbit`) is **already gone** from `main`; it survives only in history |
| Go files importing the module path | 59 now. Never hard-code this; recount from the frozen checkout |
| Web API routes | 37 files under `ui/server/api` |
| Actions | enabled, all actions allowed, default token read-only, no secrets or variables, **fork-PR approval policy `first_time_contributors`**, workflow runs on `pull_request`, no rulesets, no branch protection |
| Runner | one online runner `cairn-vm` (`self-hosted, Linux, X64, cairn`), run as **a rootless Podman container** (`container-github-runner.service`) as user `alfa` |
| VM | 7.4 GB RAM (about 5.8 GB available), 16 cores, cgroup v2, systemd 257; `cairn-tsdb` has `MemoryMax=3G`, `cairn-server` and `cairn-ui` have none |
| Hostnames baked into tracked files | 5 files (`ROADMAP.md`, `deploy/backup-places.sh`, `deploy/caddy/Caddyfile`, `deploy/deploy-ui.sh`, `docs/deploying.md`) |
| Git-ignored local files that must not travel | `firmware/cairn-v2/include/secrets.h`, `deploy/certs/`, `firmware/freematics-base/src/`, `.claude/`, `.nimbalyst/`, `nimbalyst-local/` |
| iOS repo links into the monorepo | `docs/ble-protocol.md`, `docs/firmware-changes.md`, `docs/roadmap.md`, `HANDOFF-FIRMWARE.md`, pointing at `ParkWardRR/Cairn` and `firmware/cairn-v2/` |
| Tags | `v0.1.0` ("tools release", the old stack) |

---

## Path disposition

Every tracked path gets an explicit destination or `archive-only`. The exhaustive map is
a **phase 0 artifact**, `split/paths.tsv` (glob, destination repo, destination path), with
a coverage check that fails if any tracked path matches no rule. The directory-level
shape:

| Source | Destination |
| --- | --- |
| `server/` (Go) | `cairn-vehicle-server` at the root: `cmd/`, `internal/`, `format/`, `go.mod` |
| `server/cmd/cairn-fsq` | `cairn-vehicle-web-dashboard` under `tools/cairn-fsq/` as its **own small Go module**. It imports only the standard library and the DuckDB driver (verified), no server internals, and its only consumer is the web layer. Language is not an ownership boundary |
| `deploy/` systemd for `cairn-server`, `cairn-tsdb`, `migrations/`, `deploy-v3.sh`, `server.env.example` | server |
| `deploy/` `deploy-ui.sh`, `cairn-ui.service`, `cairn-ui-places.conf`, `ui.env.example`, `caddy/`, `backup-places.sh` | web |
| `ui/` | web, at the root |
| `firmware/cairn-v2/` | firmware, at the root (PlatformIO project) |
| `emulator/` | firmware, `emulator/` |
| `research_notes/`, `reports/` | firmware, `research/` |
| `fixtures/format-v3`, `fixtures/enroll-v1` | front door, `contracts/format/v3/vectors`, `contracts/enrolment/v1/vectors` |
| `docs/` (per the table below) | split by owner |
| `.github/workflows/ci.yml` | split by job (see CI) |
| `tests/check-runners.sh` | copied into every repo that has CI |
| `LICENSE` | copied into every repo |
| `.gitignore` | split per repo |
| `INSTALL.md`, `ROADMAP.md`, `README.md` | `INSTALL.md` split by component; README and ROADMAP stay with the front door |
| history of paths that no longer exist (the legacy stack, `deploy/migrations/v2`, old compose files, ...) | `archive-only`: the path filters drop them; they remain in the archive and in the front door's history |

### Where each doc goes

| Doc | Home |
| --- | --- |
| `bundle-format-v3.md` | front door `contracts/format/v3/` |
| `app-sync-protocol.md` | `contracts/sync/v1/` (draft until implemented; see phase 9) |
| `device-provisioning.md` | wire parts in `contracts/enrolment/v1/`; the operator how-to in server |
| `ble-companion-protocol.md` | `contracts/ble/v1/`; the iOS repo's `docs/ble-protocol.md` becomes a link |
| `architecture.md`, `threat-model.md`, `trust-model-v3.md`, `guarantee-audit.md`, `ROADMAP.md` | front door `docs/` |
| `places.md`, `screenshots/` | web |
| `deploying.md`, `tailscale-deployment.md`, `retention-and-backup.md` | server |
| the nine hardware and firmware-testing docs (`esp32-hardening`, `flashing-and-testing`, `freematics-emulation-spec`, `hardware-roundtrip`, `manual-transmission-probe`, `ota`, `rtos-evaluation`, `v2-firmware-testing`, `v2-hardware-mapping-audit`) | firmware `docs/` |

### Where open issues go

| Issue | Destination | Why |
| --- | --- | --- |
| `Cairn` #1 publish `app-sync-protocol.md` with vectors | stays (front door) | protocol ownership |
| #2 authenticated sync API | server | implementation |
| #3 revocation and admin identity | server | implementation |
| #4 snapshot behind authentication | server (web gets a follow-up for the passthrough) | implementation |
| #5 bundle relay from an authenticated app | server; its wire format is tracked against `contracts/sync/v1` | implementation, with a protocol dependency |
| #6 dongle BLE offload | firmware | implementation |
| #7 retire the device mTLS listener, certificates and network OTA | stays as the tracking issue; child issues in server (listener, certificates) and firmware (OTA) | spans repos |
| iOS #9, #12, #14 | stay in iOS; cross-linked to `contracts/ble/v1` and `contracts/sync/v1` | the relay and offload work depends on both contracts |

Transfers use `gh issue transfer` after the destination repo exists; check labels after.

---

## Contracts

### Layout

```
cairn-driving-log-selfhosted/
├── README.md  LICENSE  ROADMAP.md
├── docs/            architecture, threat-model, trust-model-v3, guarantee-audit
├── contracts/       "Cairn Vehicle Data Protocols"
│   ├── README.md  CHANGELOG.md
│   ├── format/v3/      spec + vectors/ (positive and negative)
│   ├── enrolment/v1/   wire spec + vectors/
│   ├── sync/v1/        draft until implemented
│   ├── ble/v1/         spec + vectors/
│   ├── share/v1/       reserved, draft: portable trip export with redaction (see Product direction)
│   └── store/v1/       schema.json, http-api.md, local-api.md
├── scripts/         check-contracts, check-links, pin-dashboard
└── .github/workflows/  contracts.yml, docs.yml
```

`CAIRN_CONTRACTS` always means **the directory that contains `format/`, `sync/`, `store/`**,
that is, the `contracts/` directory itself, never the checkout root.

### Releases

- **Semver for the collection**, tagged `contracts-vX.Y.Z`, only when contract content
  changes. A README section defines what is breaking for the collection.
- **Protocol directories are versioned independently** of the collection tag
  (`format/v3`, `enrolment/v1`, ...). A breaking protocol change adds a new directory.
- **Retention**: an old protocol directory stays for as long as any supported consumer
  still uses it, not for "one release". Firmware is the long pole (devices in cars update
  slowly), so a firmware-facing format is kept at least until no supported firmware
  produces it. Each protocol states its support window in its README.
- **Tags are protected, and publishing is manual.** No CI job holds permission to tag or
  publish a contract. A maintainer tags.

### How a repo consumes them

A machine-readable lock, `contracts.lock`, replaces the informal one-line pin:

```json
{
  "contracts": {
    "repo": "https://github.com/ParkWardRR/cairn-driving-log-selfhosted",
    "tag": "contracts-v1.0.0",
    "commit": "<resolved sha>",
    "protocols": { "format": "v3", "enrolment": "v1" }
  }
}
```

- **Fetch** shallow-clones the tag into `.contracts/` (gitignored) and verifies **both**
  the tag and the resolved commit SHA. Contract root is `.contracts/contracts/`.
- **Local override**: `CAIRN_CONTRACTS=../cairn-driving-log-selfhosted/contracts` wins
  over the fetch, so a contract and its implementation can change together on a laptop.
  It prints the resolved SHA and whether the tree is dirty.
- **Release CI rejects** any override and any dirty contract tree.
- The **web** lock additionally records `server.lock`: the exact server release its CI ran
  against, separate from its **required capabilities** (a compatible schema range, not
  "live version at least N").
- The front door's **pin dashboard is generated** from each repo's lockfile by
  `scripts/pin-dashboard`; nobody edits the table by hand.

### The change loop (contract-first, with candidates)

```text
Draft contracts PR + implementation branch
-> generate candidate vectors and schema
-> review the normative spec and expected outputs
-> candidate conformance checks pass
-> merge and tag contracts (maintainer)
-> bump consumer pins and release implementations
```

For a **byte-level format change** at least one **independent** consumer must validate the
candidate vectors before tagging. A generator reproducing its own output proves
determinism, not correctness. The generators (`mkvectors`, `dump-schema`) stay in the
server because they use its packages; they write into `$CAIRN_CONTRACTS`, and server CI
regenerates into a temp directory and diffs against the pinned tag.

### Fixtures

Deterministic and offline. Each protocol directory includes **negative vectors**:
malformed input, authentication failures, unsupported versions. The warning that the
test keys are public stays in the vectors' README.

### C5: the web layer's contract with the store

Tables `position`, `obd`, `boost`, `imu`, `status`, `gap`, `transition`; views
`v_drive_summary`, `v_telemetry`, `v_vehicles`, `v_trim_map`, `v_pulls`,
`v_speed_agreement`, `v_boost_curve`, `v_reproducibility`; the `vehicle_id` column;
endpoints `/query`, `/healthz`, `/snapshot`, the local vehicles API. This is the
contract that already caused an outage, so it gets a real test, in this direction:

1. start the **actual server release** from `server.lock` (the synthetic-data build of the
   store, `cairn-tsdb-demo`) and let it create its **native** schema;
2. dump that native schema and **compare it with the pinned `store/v1/schema.json`**
   (a compatible range: required tables, views and columns present with compatible
   types; extras allowed). Loading the expected schema into the test server instead
   would hide a release that does not actually produce it;
3. only then run the web route inventory against it.

`/healthz` reports a **build identity and a store contract identity** (for example
`store/v1.3`); a separate capabilities response is used if the detail gets large. The
deploy script reads them and refuses to ship a web build whose required capabilities the
live store does not report. A newer server that removed a view still fails, which a
minimum version number alone would not catch. (All of this is phase 9; until then the
web CI uses the route inventory against the demo store.)

---

## Route inventory (web acceptance)

"Every route returns 200" is not an acceptance criterion. `tests/routes.json` lists all
37 routes with: method, auth, inputs, expected status, and response assertions. It is
generated from `ui/server/api` in phase 1 so no route is missed, then filled in by hand.
Each route is exercised under:

| Scenario | Examples |
| --- | --- |
| representative data | trips list, a trip's route, stops, fuel, timeline, places, dashboard stats |
| empty store | the same routes against a store with no trips return empty, not errors |
| bad input | unknown trip id is 404; a non-JSON mutation is 415; an empty place name is 400 |
| missing capability | vehicles with no local API configured degrades to ids only; snapshot with the store unavailable |
| access | mutations require `application/json`; `places/saved` import and export round-trip; no route leaks the saved-places file path |

Response assertions check shape and key values, not just status.

---

## CI after the split

| Today (`ci.yml`) | Goes to |
| --- | --- |
| `runner-policy` and `tests/check-runners.sh` | every repo with CI, extended to enforce the trusted-code guard below |
| `build-server`, `decode-pipeline` (real PostGIS), `vector-determinism` | server (determinism diffs against the **pinned** contracts) |
| `format-conformance` (Rust), `firmware-conformance` (host C, plain and ASan/UBSan), `firmware-build` (PlatformIO, two configurations plus OTA) | firmware |
| `fault-matrix` (emulator against a running server) | **interop** (below) |
| new | web: vitest, `nuxt build`, route inventory against a staged demo store |
| new | front door: `contracts.yml` (specs, vectors, schemas, manifests, deterministic outputs) and `docs.yml` (links) |

**Interop.** A pinned run is a **required** check in the server repo (a pinned emulator
release against the server under test). A scheduled run in the firmware repo against the
latest server release is **diagnostic only** and never replaces the pinned gate. Later,
add firmware-PR interop for changes that touch wire behaviour.

**Branch and tag protection.** Require component CI before merge on `main`, and protect
release tags (`contracts-v*`, `v*`) with rulesets. Today there is no protection at all.

---

## Runner policy

### Trusted code (before any new runner is registered)

The repos are public and the runner shares a VM with the live system, so this comes
first. It applies to today's runner too: the current fork-PR policy is only
`first_time_contributors`, and `ci.yml` runs on `pull_request`.

1. Set **fork pull request workflows to require approval for all outside collaborators**.
2. Self-hosted jobs run only for `push` to protected branches and for `pull_request`
   **from branches of the same repository**: a job guard
   `github.event.pull_request.head.repo.full_name == github.repository`, enforced by the
   extended `check-runners.sh`.
3. Never use `pull_request_target` or `workflow_run` on a self-hosted runner.
4. **Ephemeral runners**: a fresh container per job, so one job cannot persist anything
   for the next.
5. Runner containers get no host mounts of `/var/lib/cairn*` or `/etc/cairn`, no container
   socket, and no secrets (there are none in Actions today; keep it that way).
6. The default workflow token stays read-only (it is today).
7. Longer term, consider moving CI off the machine that serves production.

### Memory

Per-runner limits do not cap four runners together. The runners are Podman containers,
so:

- put **all** CI runners in one systemd slice, `cairn-ci.slice`, with an aggregate budget
  of **`MemoryHigh=3G`, `MemoryMax=4G`** to start (pressure control first, hard ceiling
  last), plus a lower per-runner limit inside it
- protect production with **`MemoryLow`** on `cairn-tsdb` (and a `MemoryMax` on
  `cairn-server` and `cairn-ui`, which have none) so pressure reclaims from CI first
- lower `CPUWeight` and `IOWeight` for the slice
- keep the shared heavy-step lock (`flock`) as well; it is useful but is not a substitute
  for an aggregate limit

These numbers are a **starting estimate, not a validated capacity**. Phase 0 measures
the real peak of each repo's CI and production headroom, and phase 5 runs everything at
once to confirm.

---

## Extraction

**Rehearse on throwaway clones; the monorepo is not touched until the archive step; no
force-push anywhere.**

The extraction is a **committed, tested script**, not an inline recipe. It lives in the
front door under `split/` with its own fixture repository that includes renamed and moved
files, an executable bit, a symlink and a merge commit, and a test that runs the script
on it.

Plain git is the default (decision 5). `git filter-branch` is the risky one of git's own
history-rewriters, which is why it is wrapped in tests; `git filter-repo` is safer but is
Python. Either way the verification below is what makes the result trustworthy.

### Verification (per destination repo)

| Check | Rule |
| --- | --- |
| content | every destination file equals its source at the final snapshot, applying the documented path moves |
| modes | executable bits and symlinks preserved |
| authors | the set of authors in the destination is a subset of the source's, and none is lost for retained paths |
| history | sampled historical versions of several files match the source at those commits |
| commit counts | **diagnostic only**; pruned and merge commits make equality the wrong test |
| coverage | the path map covers every tracked path |
| provenance | the repo's first commit is `MIGRATION.md`: source monorepo SHA, path mapping, tool and version |
| tags | only tags that make sense carry over. The old `v0.1.0` tools release stays with the front door and the archive, and is not copied into any component as a component release |

### Rewrites (one commit each, after extraction)

- Go module and its imports, recounted at the freeze; then `go build`, `go vet`, `go test`.
- Vector paths: every `../fixtures/...` in tests, `mkvectors` and the emulator, and the
  firmware Makefile and `prov_test.c`, become `$CAIRN_CONTRACTS/...`.
- Docs links that cross a boundary become absolute links to the owning repo.
- The 5 hostname files take a host from an argument or a gitignored env file.
- Topics and descriptions are replaced on every repo (the current topics are stale).

### Hygiene

Scan each extracted history for keys, tokens and real addresses before publishing. The
monorepo is already public and the last scan found only labelled public test keys, but
history is cheap to re-check.

---

## Phases

Each phase ends with something verifiable. **Production is touched only in phase 6.**

### Phase 0: Freeze, baseline and rules

- [ ] The other session lands or drops its in-flight work; `main` clean
- [ ] **Tag `split-baseline`** on the commit everything starts from
- [ ] Local backup mirror of the baseline
- [ ] Path disposition map `split/paths.tsv` and its coverage check
- [ ] Extraction script written and tested against the fixture repo
- [ ] Trusted-code policy applied to the **existing** repo and runner (fork approval,
      same-repo guard, no `pull_request_target`)
- [ ] Measure the peak memory of each CI job and production headroom
- [ ] Refresh the live inventory above (issues, routes, Go file count)
- [ ] History scan for secrets and hostnames
- [ ] Inventory of git-ignored local files to carry by hand
- [ ] Copy the Claude memory directory to the keys of the new working directories
- [ ] Answer decisions 5 and 6

**Exit:** a clean tree, a baseline tag, a measured resource budget, a passing extraction
test and a complete path map.

### Phase 1: Boundary preparation (mechanical only)

Behaviour-preserving moves inside the monorepo, so extraction is mechanical.

- [ ] Create `contracts/` in the layout above; move the existing vectors and the four
      protocol docs into it; mark anything not yet implemented `draft`
- [ ] Point server, firmware and emulator at `$CAIRN_CONTRACTS` (default `./contracts`)
- [ ] Remove every remaining `../` path that crosses a future repo boundary
- [ ] Generate the route inventory skeleton `tests/routes.json` (not yet filled)
- [ ] A staging harness for the web layer: the web server on spare ports against the demo
      store, so nothing needs production
- [ ] Resource slice and per-runner limits prepared, not yet enforcing on production

**Exit:** the full existing CI is green. **Then tag `monorepo-final`** on that commit;
extraction and the archive are made from this tag, not from the baseline.

### Phase 2: Rehearse the extraction

- [ ] Run the script for each repo on throwaway clones from `monorepo-final`
- [ ] Run the verification table; fix the script, not the output
- [ ] Build and test each result locally (Go, Rust, PlatformIO host suites, Nuxt)

**Exit:** all three extractions pass verification and build locally; nothing published.

### Phase 3: Publish the component repos

Done in order, each independently verifiable. No production deployment in any of them.

**3a. `cairn-vehicle-server`**
- [ ] Create the repo (description, topics, labels, rulesets); runner registered under
      the runner policy
- [ ] Extract; module rename; vector paths; `contracts.lock`; `MIGRATION.md`
- [ ] CI: build, vet, gofmt, tests, decode pipeline on PostGIS, determinism against the
      pinned contracts, runner policy
- [ ] Transfer issues #2 to #5 (see the routing table)
- [ ] `deploy-v3.sh` **build-only** from the new repo against the VM

**3b. `cairn-vehicle-web-dashboard`**
- [ ] Create the repo; extract `ui/` and the web deploy files; `tools/cairn-fsq` as its own module
- [ ] `contracts.lock` and `server.lock`; vitest; `nuxt build`
- [ ] The route inventory filled in and run against the **staged** instance (spare ports,
      demo store, synthetic data) for every scenario in the table
- [ ] `deploy-ui.sh` builds from the repo's own `HEAD`; hostnames out of the scripts

**3c. `cairn-esp32-device-firmware`**
- [ ] Create the repo; extract the PlatformIO project, `emulator/`, `research/`
- [ ] `contracts.lock`; host C suites (plain and ASan/UBSan); format conformance;
      PlatformIO builds, one job at a time
- [ ] `secrets.h.example` stays; the real `secrets.h` copied by hand and ignored
- [ ] Transfer issue #6

**Exit (each):** CI green on the runner, and for the web, the staged route inventory passes.

### Phase 4: Contracts in the front door

- [ ] `contracts.yml` and `docs.yml`; contract validity, manifest and link checks
- [ ] First tag `contracts-v0.1.0` (maintainer-created); the README states the release rules
- [ ] Route the open issues per the routing table
- [ ] Repoint each component's `contracts.lock` at the published tag and commit SHA

**Exit:** all three component repos build against the tag, not against a local path.

### Phase 5: Cross-repo checks

- [ ] Pinned interop as a required check in the server repo; scheduled latest-release
      interop in the firmware repo (diagnostic)
- [ ] All repos' CI at once with the slice enforcing; confirm production headroom
- [ ] **The drill**: a deliberately contract-breaking change on a branch must fail the
      right job in the right repo
- [ ] Confirm a fork PR cannot run on the runner without approval

**Exit:** the drill fails where it should, memory stays within budget, the fork test holds.

### Phase 6: Production cutover (the only phase that changes production)

- [ ] **Rollback snapshot**: unit files, environment files, migrations, source paths, build
      identities (binary hashes and versions), the current web build; all writable stores
      (`/var/lib/cairn` keystore, counters, vehicles, devices, clients; `/var/lib/cairn-ui`
      saved and learned places and the lookup cache). **Restore is tested**, not assumed
- [ ] No destructive migration during cutover
- [ ] Deploy the server from `cairn-vehicle-server`, then the web from
      `cairn-vehicle-web-dashboard`
- [ ] Verify services, the route inventory against production (read-only requests), ingest
      and the store

**Exit:** production runs entirely from the new repos with the rollback snapshot verified.

### Phase 7: Archive, front door and renames

- [ ] Create `cairn-original-monorepo-archive` from `monorepo-final`; archive it
- [ ] Reduce the front door: remove extracted implementation code, keeping docs,
      `contracts/` and their CI; add the README map and the generated pin dashboard
- [ ] Write the front door README for its two audiences: a plain-language opening
      (what it does, why you would use it, where to start) for less technical drivers,
      then the map of repos for builders. State the five reasons to use it
- [ ] Replace the repository description, which still says "No phone" (no longer true:
      the phone is the dongle's uplink), and the stale topics
- [ ] Seed `ROADMAP.md` with the five product themes from "Product direction", each with
      its owning repo and the contract it touches
- [ ] Add a sharing section to `threat-model.md` before any sharing design begins
- [ ] Rewrite links: commit-pinned and final-tag links stay valid; old
      `blob/main/...` links to removed files get rewritten or a stub
- [ ] Repoint the iOS repo's four links to the firmware repo and `contracts/`; move the
      accepted, implemented baseline BLE and sync golden vectors into `contracts/`
      (future protocol vectors stay `draft`; no competing authoritative copies)
- [ ] Rename `Cairn` to `cairn-driving-log-selfhosted` and the iOS repo to
      `cairn-ios-companion-app`, **last** (decision 6); never create another repo named
      `Cairn`; update README badges and workflow URLs

**Exit:** the monorepo is preserved read-only; the iOS app has no behaviour change, only
documentation and contract-test wiring.

### Phase 8: Retire the local monorepo

- [ ] Move `~/antigravity/Cairn` aside; clone the new repos into place
- [ ] Remove the old runner registration
- [ ] Update the Claude memory index and notes to the new names

### Phase 9: Contract hardening (a separate milestone, after the split)

New behaviour, deliberately not part of the split, and each item may be released on its own:

- [ ] `sync/v1` vectors, including negative ones; keep it `draft` until a server and the
      iOS app both implement it, and require independent validation before tagging
- [ ] `dump-schema` and `store/v1/schema.json`; the native-schema comparison from C5
- [ ] `/healthz` build identity and store contract identity; the capabilities response
- [ ] The web deploy refuses a build whose required capabilities the live store lacks
- [ ] Negative vectors for the existing protocols
- [ ] `scripts/pin-dashboard` generating the README table from lockfiles

**Exit:** each contract is implemented by at least one producer and one independent consumer.

---

## Rollback

- The monorepo and its baseline and final tags, and a local mirror, exist until phase 8.
- No force-push and no history rewrite on any existing remote.
- Preparation is not "adds only": it changes the monorepo, transfers issues and updates
  metadata. What is true is that **production is untouched until phase 6**, and phase 6
  starts from a tested restore point.
- If a phase fails its exit check, stop there. Published component repos can be
  archived without affecting production.

## Risks

| Risk | Mitigation |
| --- | --- |
| Another session edits the monorepo during the split | Phase 0 freeze; this has already caused an outage and a swept-up commit |
| An untrusted fork PR runs code on the VM that hosts production | Trusted-code policy first; ephemeral runners; no mounts; fork approval for all; same-repo guard |
| CI overload takes production down | One aggregate slice with `MemoryHigh` and `MemoryMax`, `MemoryLow` on production, measured peaks |
| Protocol drift between repos | Lockfiles with tag and SHA, native-schema comparison, pinned interop, the drill |
| A generator "agreeing with itself" is mistaken for correctness | Independent consumer validation before tagging a byte-level change |
| Rewriting history loses something | Tested script, verification table, throwaway rehearsals, provenance file |
| Old links break | Commit-pinned and tag links stay; `blob/main` links rewritten or stubbed; renames last |
| Renaming `Cairn` conflicts with keeping its Issues URL | Decision 6; rename last and reversible |
| Public-repo hygiene | History scan; hostnames out of scripts; public-key warning in vectors |
| Python dependence (PlatformIO in firmware CI, optionally `filter-repo`) | Plain-git extraction by default; PlatformIO already runs in CI today |
| Claude memory is keyed by working directory | Copy to the new keys in phase 0 |
| Product ambitions leak into the split and make it unverifiable | Product work is roadmap after the split; the split only keeps its boundaries open (see "The split must not foreclose") |
| Sharing is designed before the privacy model | The threat-model sharing section is a phase 7 deliverable and gates any `share/v1` work |
| Effort | Roughly 8 to 10 working sessions for the split: phases 1, 3 and 9 are the largest. The product themes are separate and not counted here |

---

## Review disposition

How each point of the two reviews was handled, including the ones I checked and did not
simply accept.

### First review

| Point | Disposition |
| --- | --- |
| `monorepo-final` tagged before the prep work changes the repo | **Accepted.** `split-baseline` in phase 0; `monorepo-final` after phase 1 passes CI; archive from the final tag |
| "Nothing live changes until phase 7" contradicted a deploy in phase 4 | **Accepted.** Web acceptance is a staged instance on spare ports with synthetic data; production only in phase 6 |
| Contract hardening is more than moving files | **Accepted.** Moved to phase 9, after cutover; contracts not yet implemented stay `draft` |
| "Every route returns 200" is too weak | **Accepted.** Route inventory with methods, inputs, statuses, assertions and scenarios |
| Per-runner `MemoryMax` does not cap the aggregate | **Accepted.** Shared slice with `MemoryHigh` and `MemoryMax`, plus `MemoryLow` on production. About 4 GB is a starting estimate to be measured |
| Public repos running workflows on the production VM | **Accepted, and confirmed live**: fork-PR approval is `first_time_contributors` and `ci.yml` runs on `pull_request`. Applies to today's runner too, so phase 0 fixes the existing repo first |
| One-line pin is too informal | **Accepted.** Machine-readable lock with tag **and** commit SHA, verified on fetch; override prints SHA and dirty state; release CI rejects both |
| Server dependency and runtime capability conflated | **Accepted.** `server.lock` for the CI release, separate required capabilities |
| Protocol identity independent of collection version | **Accepted.** Directory versions independent of `contracts-vX.Y.Z` |
| Minimum schema number is not compatibility | **Accepted.** Compatible-range comparison; `/healthz` build and store contract identity; capabilities response |
| C5 test direction | **Accepted.** Start the real release, dump its native schema, compare with the pinned contract, then run web requests |
| Contract-first loop contradiction | **Accepted.** Publication is the boundary; candidate flow with independent validation for byte-level changes |
| Offline deterministic fixtures with negative vectors | **Accepted** |
| "One release" retention is too short | **Accepted.** Retain while supported consumers use it; firmware-facing formats longest |
| Pin table maintained by hand | **Accepted.** Generated from lockfiles |
| Reconsider `filter-branch` | **Partly.** Kept plain git because of your no-Python preference, but wrapped in a tested script with a verification table. `git filter-repo` is offered as an explicit exception (decision 5) |
| Path inventory with a disposition for every path | **Accepted.** `split/paths.tsv` with a coverage check |
| Extraction verification beyond commit counts | **Accepted.** Content, modes, symlinks, authors, sampled history; counts diagnostic only |
| Which tags survive | **Accepted.** `v0.1.0` stays with the front door and archive only |
| Provenance in each repo | **Accepted.** `MIGRATION.md` as the first commit |
| Rollback beyond `.prev` | **Accepted.** Phase 6 snapshot of units, env, migrations, source paths, build identities, stores; tested restore; no destructive migration |
| Protection rules and manual contract publishing | **Accepted.** Rulesets for `main` and release tags; no CI publishing |
| Soften "every old link resolves" and "only adds" | **Accepted** and reworded |
| Refresh the live inventory | **Done, with two exceptions below** |
| Root still has `gleam`, `mojo`, `moonbit`, `odin`, `rust`, `zig`, `plugins` | **Not reproduced.** The GitHub `main` root listing and the local tree have none of them; they are gone from `main` and live only in history. Marked `archive-only` |
| Go imports: 60 files | **Not reproduced.** I count 59 now. The plan no longer hard-codes a count |
| Cairn open issues: 7; iOS open issues: 13 | **Confirmed**; both are newer than my first look. The routing table covers all of them |
| Answers to the open questions | Adopted: semver; pinned interop required plus scheduled diagnostic; `cairn-fsq` to web (I verified it imports no server internals); move accepted baseline vectors now; iOS "no app behaviour change" |

### Second review (names and merging contracts)

| Point | Disposition |
| --- | --- |
| Merge contracts into the front door | **Accepted** (supersedes the earlier "fifth repo" choice) |
| Final names | **Accepted** as listed |
| Rename `Cairn` and the iOS repo | **Accepted, flagged.** Renames are last and reversible; decision 6 notes the tension with keeping the `Cairn/issues` URL |
| `CAIRN_CONTRACTS` is the `contracts/` directory | **Accepted** |
| Tags `contracts-vX.Y.Z`; contracts and docs CI separate | **Accepted** |
| Record repo URL, tag and resolved SHA in consumers | **Accepted** (the lockfile) |
| Protocol issues stay in the front door; implementation issues transfer | **Accepted** (routing table) |

## Open questions

- **Decision 5**: plain git, or approve `git filter-repo` as a one-off Python exception? exeption is fine
- **Decision 6**: rename `Cairn` as proposed, or keep the name so `…/Cairn/issues` stays canonical? up to you
- No need to support: What is the minimum support window for firmware-facing formats? The plan says "until no. 
  supported firmware produces it"; a concrete number (for example 12 months after the
  replacement ships) would make it testable.
- Proceed: Should the scheduled interop job also run on firmware PRs now, or only once a wire-touching
  change is in flight?
- both: **Annual statistics**: per-year totals (distance, time, fuel, top routes), or something
  more like a yearly review? The first is a store view; the second is its own feature.
- no just explortable images or gpx files. no website hosting. mayeb support for s3 static site later. Our webserver is never public facing: **Sharing**: a file the owner sends (GPX or GeoJSON plus stats) as the first step, or an
  expiring link to a page the server hosts? The first needs no hosting and no new
  attack surface; the second is what most people expect. What is blanked by default
  (home, start and end, speed)?
- not on IOS: **Tune record**: entered by hand, or detected from a change in behaviour and confirmed by
  the user? Detection is the harder and more useful version.
- yes - the iOS app simple, the web detailed-  **Two audiences in one app**: a simple view by default with an expert switch, or two
  separate surfaces (the iOS app simple, the web detailed)?


Future goals when we have GPU:

Large-scale route similarity/clustering	Potentially useful	Batched distance calculations could become a GPU candidate with a large archive, but only after profiling establishes that numerical computation is the bottleneck.
Learned engine-health or driving-pattern models	Potentially useful	Training or batched inference would introduce a different workload; this is a possible future feature, not a demonstrated current requirement.
Local-language-model trip summaries	Useful for the model, not Cairn itself	A separately hosted model could summarize derived statistics without changing the capture or receipt pipeline.