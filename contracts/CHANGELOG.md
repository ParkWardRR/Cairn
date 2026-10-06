# Changelog

Contract releases are tagged `contracts-vX.Y.Z` and cut by a maintainer; no CI job tags or
publishes. Protocol directories are versioned independently of the collection tag.

## Unreleased

- `uplink/v1` (**draft**, new): the device uplink protocol for direct Wi-Fi and LTE upload. Per-request Ed25519
  signatures by the enrolled device key (`CAIRN-UPLINK-V1`, eight-line signing string), a server key pinned in
  firmware, the relay's offer, chunk, commit and receipt under `/v1/uplink/bundles/`, and 15 deterministic
  vectors including negative cases. Additive: no existing protocol or vector changed.
- `format/v3/spec.md`: a link to the firmware hardening notes now points at the firmware
  repository. Documentation only; no protocol or vector changed, so nothing is released for it.

## 0.1.0 (2026-10-05)

The first release: the contracts move out of the monorepo into their own repository, and
the component repositories build against this tag instead of a local path.

- Collected into one place from the monorepo: `format/v3`, `enrolment/v1`, `sync/v1`,
  `ble/v1` (companion and offload), with their vectors. No byte changed in the move.
- Added `store/v1` and `share/v1` as drafts.
- Byte-level independence check: the `format/v3` vectors (33) pass unchanged in the Go reference,
  the C firmware implementation and the Rust emulator, and the generator reproduces them with no
  diff. `ble/v1` offload vectors are generated from the firmware module and match.
