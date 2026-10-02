# Guarantee audit

Every guarantee this project documents, and where it is actually verified.

The standing rule is that a documented guarantee has a corresponding test row.
This file is the check on that rule, and the reason it exists is that prose and
tests drift in one direction: a claim is easy to write and a row is work, so
without an explicit mapping the documentation slowly becomes aspirational. The
sections at the end list what is *not* covered, which is the part worth reading.

Where to find the suites:

All of these run in CI. Two jobs exist specifically to protect claims this file
makes: `firmware-build` compiles all three ESP32 configurations, which the host
suites do not do, and `vector-determinism` regenerates the vectors and fails on
any diff — so "the vectors are deterministic" is enforced rather than asserted.

| Suite | Command | Count |
|---|---|---|
| Format conformance (Go) | `go test ./format/` | 25 vectors |
| Format conformance (Rust) | `cargo run -- conformance --vectors ../fixtures/format-v2` | 22 vectors, 3 skipped |
| Format conformance (C) | `make -C firmware/cairn-v2/test/host conformance` | 25 vectors |
| Firmware storage matrix (C) | `make -C firmware/cairn-v2/test/host faults` | 28 rows |
| Fault-injection matrix (Rust) | `cargo run -- matrix` | 9 rows |
| Server unit and integration | `go test ./...` | — |
| Decode pipeline (needs PostGIS) | `CAIRN_TEST_DSN=… go test ./internal/store/` | 11 rows |
| Firmware target build | `pio run` in `firmware/cairn-v2` | 3 configurations |
| Vector determinism | regenerate, then `git diff --exit-code -- fixtures/` | — |

## The four invariants

| Invariant | Verified by |
|---|---|
| **1.** A sealed bundle is never mutated | C matrix: *a sealed bundle is never overwritten* (stages a colliding id and confirms the original manifest is untouched). `cairn_fs_rename` refuses an existing target in both backends |
| **2.** No byte is deleted without a locally verified signed receipt | C matrix, five rows: untrusted key, wrong content root, malformed receipt, absent/placeholder key, and the positive case. Rust matrix: `prune_requires_receipt`, `reboot_between_receipt_and_prune` |
| **3.** Ordering truth is `(boot_id, seq)`, never wall-clock UTC | Conformance vector `clock-jump` in all three implementations. Rust matrix: `clock_jump_preserves_ordering`. C matrix: *one chain spans segment rotation*, *the journal chain is independent* |
| **4.** Honest incompleteness beats fabricated continuity | Conformance vectors `torn-tail-mid-frame`, `torn-tail-mid-header`, `bad-frame-crc`, `chain-break-spliced`, `seq-gap`. C matrix: three recovery rows asserting the *exact* discarded byte count. Store test: `TestGapsAreRecordedAndNotBridged` |

## Format

| Claim | Verified by |
|---|---|
| Three implementations agree byte-for-byte | The same 25 vectors run against Go and C. Rust runs 22 and **reports the 3 it skips**: it implements frames, manifests and receipts, not record payloads or OTA descriptors. A silent skip would look exactly like a pass, which is the drift the vectors exist to prevent |
| A policy snapshot encodes identically across implementations | Vector `policy-snapshot` commits the payload; Go and C both decode it *and* re-encode to the same bytes. Mutation-checked: changing one encoded value fails the vector |
| Trip events decode with position and detail intact, unknown types included | Vector `trip-event-types` covers all eight defined types plus an undefined one, which must be named rather than discarded |
| Degraded bitmaps survive, including reserved bits | Vector `health-bitmap` covers none, one, several at once, and the reserved bit |
| An OTA descriptor verifies against the pinned update key, and a tampered one does not | Vectors `update-descriptor-valid` and `update-descriptor-bad-signature` |
| The manifest signature covers exactly `manifest.cbor`, with nothing to strip | Vector `manifest-valid` checks the re-encoded digest; C additionally reproduces the Go signature byte-for-byte, which only a deterministic signer can do |
| Deterministic CBOR: a decoder rejects what it would not have produced | `TestDecoderRejectsNonMinimalIntegers`, `TestDecoderRejectsIndefiniteLength`, `TestManifestTrailingBytesRejected`; C `cairn_manifest_decode` re-encodes and compares |
| Merkle: an odd node is promoted, never duplicated | `TestMerkleOddLeavesPromoteNotDuplicate`, vector `merkle-odd-leaves` |
| Content root is order-independent and name-sensitive | `TestContentRootIsOrderIndependent`, `TestContentRootRejectsDuplicateNames`, vector `content-root-member-order` |
| An unknown record type is skipped and counted, never an error | Vector `unknown-record-type` |
| Degraded state is a bitmap; unknown bits are preserved | `TestHealthStatePreservesUnknownBits` and three siblings; C matrix: *degraded bitmap round-trips through the card* |
| Policy snapshot is deterministic and strictly decoded | C matrix: *snapshot encoding is deterministic*; Go `TestParsePolicySnapshotFromFirmwareBytes` reads bytes captured from the C encoder |

## Device storage

| Claim | Verified by |
|---|---|
| A torn tail is truncated to the last valid frame, with an exact byte count | C matrix, mid-frame and mid-header. The mid-frame row pins the subtle part: losing 20 bytes of a 60-byte frame discards the 40-byte *surviving prefix*, not the 20 that never arrived |
| Corruption isolates to one record | C matrix: *corrupt frame isolates to one record* — four preceding frames survive and the state is `SALVAGED`, distinct from a clean tail recovery |
| An interrupted seal completes idempotently | C matrix: *interrupted seal completes idempotently*, including that a second pass changes nothing |
| A live capture is not mistaken for an interrupted seal | C matrix: *a live capture is not mistaken for a seal* |
| A short write does not advance the chain | `cairn_capture_append` returns before advancing; the resulting torn tail is covered by the recovery rows |
| Pre-roll holds records without writing, then flushes in order flagged `PRETRIP` | C matrix, three rows, including that the flag does not leak past confirmation |
| Adaptive sampling is never slower than nominal during a trip | C matrix: *adaptive rates are never slower during a trip*, checked for every dynamics level |

## Server

| Claim | Verified by |
|---|---|
| The manifest signature is checked against the *enrolled* key before anything is trusted | `internal/intake` tests; `TestOfferRejectsUnenrolledDevice` and siblings |
| A validly signed but self-contradictory manifest is rejected | `internal/intake` consistency tests |
| Receipts are signed and persisted *before* being returned | `internal/receipts`: `TestIssueAndVerify`, `TestIssueIsIdempotentOnContentRoot` |
| An ephemeral signing key is refused outside dev mode | `TestOpenRefusesEphemeralKeyByDefault`, `TestSigningKeyPersistsAcrossRestart` |
| The missing-chunk set is derived from the store, not tracked | Rust matrix: `network_loss_per_chunk` resumes with no progress state |
| A duplicate upload is stored once | Rust matrix: `duplicate_upload_stored_once` |
| A corrupt chunk in transit is rejected | Rust matrix: `corrupt_chunk_in_transit` |
| A lost receipt is recoverable by retrying commit | Rust matrix: `receipt_lost_in_transit` |
| Quotas contain a misbehaving device | `TestOfferRejectsOverQuota`, `TestQuotaAccumulatesAcrossBundles`, `TestZeroQuotaImposesNoLimit` |
| Revocation takes effect without a restart | `TestRevocationTakesEffectWithoutRestart` — **written by this audit**; the behaviour existed and had no row |
| mTLS binds the transport identity to the manifest's claimed device | `TestClientIdentityBinding`, `TestNoClientCertificateIsRefused` — **written by this audit**. A comment in the test harness pointed at `TestClientIdentityBinding` as though it existed; it did not |
| Ingest has no database dependency | `TestSyncSucceedsWithNoWorkerOrDatabase` |
| Decode is idempotent | `TestReDecodeIsIdempotent` |
| Decode is reproducible | `TestDecodeIsReproducible` |
| A decode failure leaves raw data and the receipt intact | `TestDecodeFailureLeavesRawAndReceiptIntact` |
| A decoder upgrade re-derives from raw with no device involvement | `TestDecoderUpgradeReDerivesFromRaw` |
| A reused bundle id with different content is reported, not swallowed | `TestBundleIDConflictIsReported` |
| The vectors are generated deterministically | CI job `vector-determinism`. Mutation-checked: editing one threshold in the generator without regenerating fails it |
| The firmware compiles for its target | CI job `firmware-build`, all three configurations — capture, self-test, and OTA-enabled, since the install path is behind `CAIRN_OTA_AVAILABLE` and is otherwise never compiled |
| Standby never strands unsent data | C matrix: *standby never strands unsent data*. Each gate checked on its own — a blocker that only works in combination is one that gets removed by accident. Mutation-checked: removing the pending-bundle gate fails it |
| Waking favours the earliest reliable signal | C matrix: *waking favours the earliest reliable signal*. Engine voltage beats motion because the rail rises before the vehicle moves, which is what lets the pre-roll cover the start of a drive |
| Every ledger refusal carries a reason | `internal/ledger`: `TestRejectionsRequireAReason` |
| The ledger covers the whole lifecycle, in causal order | Verified end-to-end: `offered` → `committed` → `receipt_issued` → `decode_queued`, with `decode_succeeded` written after the outbox ack and `decode_failed` only when a job is *parked* — a retry is not an outcome |
| Trip events are attributed from evidence, or admit they are not | C matrix: *trip events round-trip through the card*. `HARSH_MOTION` exists precisely because braking and cornering are indistinguishable without orientation or a speed signal |

> The decode-pipeline rows are gated behind `CAIRN_TEST_DSN` and skip silently
> without PostGIS. CI supplies it in the `decode-pipeline` job; a local
> `go test ./...` does **not**, so a clean local run is weaker than it looks.

## OTA

| Claim | Verified by |
|---|---|
| Each precondition blocks on its own and names itself | C matrix: *every precondition blocks on its own*, including that an unknown supply voltage blocks |
| Version ordering refuses rather than guessing | C matrix: *version ordering refuses rather than guesses*, including that 10.0.0 beats 9.0.0 where string comparison would not |
| The descriptor decoder is strict | C matrix: *descriptor decoding is strict* — trailing bytes, truncation, zero-length image, wrong key |
| A tampered descriptor or signature is rejected | Verified against a real `cairn-signfw`-signed 1.1 MB image |
| The signature is verified before downloading | Code ordering in `cairn_ota_install.cpp`; **not** covered by a test |

## Not verified

The honest part. These are documented behaviours with no corresponding row.

**Needs hardware.** Nothing below can be exercised on a host.

- The OTA install path end-to-end: `esp_ota_write`, reading the slot back,
  `esp_ota_set_boot_partition`, and rollback when an image fails to mark itself
  valid. The *decisions* are tested; the flash operations are not.
- The signature-before-download ordering. It is a property of the code's
  structure, and a test would need a server that counts image requests.
- ~~The mTLS handshake on-device~~ — **verified 2026-10-01.** The device
  completed offer, chunk upload and commit over HTTPS on 8443 against the
  private CA, and the server attributed all three to `device=8777228e`, which it
  can only derive from the client certificate's CommonName. So the identity
  binding holds against mbedTLS on real hardware and not only against the Go
  test client. Heap at handshake time was not a problem: free heap was 235 KB at
  boot and the sync completed without a drop.
- GNSS date and time decoding. The civil-from-days arithmetic has never seen a
  real fix, and the driver's `date`/`time` encoding is assumed.
- `COBD::getVoltage()` units. Assumed volts; the firmware multiplies by 1000.
- The fact queue under real load, and therefore whether the drop counter ever
  fires in practice.
- **Parked current draw.** Standby powers the radio and GNSS down, puts the
  coprocessor in its low-power mode, clocks to 80 MHz and light-sleeps between
  polls. Which conditions permit standby is tested; how many milliamps result is
  not, and cannot be without a multimeter. This is the measurement most worth
  taking first on hardware.
- Whether adaptive sampling's rate changes are actually achieved, as opposed to
  requested. The frames carry their own timestamps so this is recoverable from a
  real bundle, but no run has produced one yet.

**Possible on a host, not yet done.**

- Threshold tuning against real traces. Deliberately deferred: tuning against
  guesses would be worse than the current defaults.
- No staged rollout or per-device firmware pinning; every enrolled device sees
  the same `latest`.
- `IMU_RAW_WINDOW` is defined in the format and exercised by the vectors, but
  the firmware never emits it. Raw windows are large and the summary carries the
  statistics that matter; this would be opt-in for a diagnostic build.

**Deliberately out of scope**, with the reasoning recorded where the decision
lives:

- ESP-IDF secure boot and flash encryption. Both are irreversible, which is a
  poor property for hardware already in a vehicle. Consequence: the signing seed
  is readable from an extracted chip, and a physically present attacker can
  flash over serial. The key authorizes uploads, not deletions, and the server
  can revoke it.
- `previous_bundle_root` is populated but not enforced. Detecting deleted
  *historical* bundles is a different threat model from detecting corruption
  within one bundle.
