# tools

Small programs and scripts this repository still uses. Each has a CI job that vets and tests
it (`.github/workflows/ci.yml`).

| Path | What it is | Used by |
|---|---|---|
| `cmd/contract-check`, `contractcheck/` | Checks every protocol version under `contracts/` is complete and valid | `contracts.yml` |
| `cmd/engine-check`, `enginelang/` | A third, independent implementation of the `engine/v1` formula language, compiler and bytecode evaluator, run against `contracts/engine/v1/vectors/expr.txt`. Deliberately has no `--update`: a checker that can rewrite its own vectors is not a check | `contracts.yml` |
| `cmd/link-check`, `links/` | Fails if a relative link in any markdown file does not resolve | `docs.yml` |
| `cmd/pin-dashboard`, `pindash/` | Regenerates the README's table of pins from each repository's lock files | `pins.yml` |
| `cmd/uplink-vectors`, `uplinkvectors/` | Generates and checks `contracts/uplink/v1/vectors`, and is the reference for its request-signing rules | `ci.yml` (the test fails if the checked-in vectors drift) |
| `cmd/ble-vectors`, `blevectors/` | Generates and checks `contracts/ble/v1/vectors/device-info`, and is the reference encoder, decoder and instruction check for `device-info.md` and `checkin.md` | `ci.yml` |
| `runner/` | Installs one self-hosted CI runner as a rootless Podman container under a shared memory cap | by hand, on the CI host |
| `publish/create-repo.sh` | Creates a public repository with the project's standard settings | by hand |
| `cutover/` | A cold snapshot of the whole stack and a restore test that boots it | by hand; **moving** to `deploy/` in cairn-vehicle-server, see [its issue 17](https://github.com/ParkWardRR/cairn-vehicle-server/issues/17), and then deleted here |

Run Go tools from this directory, for example `go run ./cmd/contract-check ../contracts`.

## What was removed

The tooling that split the original single repository into the component repositories (the path
map and its coverage check, the history extraction, the rewrites and the README and CI
templates) did its job on 2026-10-05 and is gone. It remains in this repository's history, at
the commit before the one that removed it, and in the archive repository.
