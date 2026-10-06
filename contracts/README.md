# Cairn Vehicle Data Protocols

The wire formats and vectors that the parts of Cairn agree on: the dongle's firmware, the
server, the web dashboard and the phone app. Each can be built, tested and released alone;
what they share is here, versioned, and checked by machine.

| Protocol | What it is | Producers / consumers | Status |
|---|---|---|---|
| [`format/v3`](format/v3/) | Sealed trip bundles: encrypted segments, manifest, receipt, update descriptor | firmware writes; server and emulator read | **stable**, 33 vectors reproduced by three implementations |
| [`enrolment/v1`](enrolment/v1/) | The sealed enrolment blob and the USB console protocol | firmware produces; server verifies | **stable**, vectors reproduced by two implementations |
| [`sync/v1`](sync/v1/) | The phone's API to the server: signing, enrolment, push/pull, snapshot, bundle relay | server implements; app consumes | **draft**: the server implements it, the iOS app does not yet |
| [`ble/v1`](ble/v1/) | The dongle's BLE service: phone GPS, status, and bundle offload | firmware serves; app consumes | **stable** for the companion; **offload** is implemented on the firmware and a Go reference client, not yet on the phone |
| [`store/v1`](store/v1/) | The analytical store the web dashboard queries | server produces; web consumes | **draft**: surface described, machine-checked schema to come |
| [`uplink/v1`](uplink/v1/) | How a dongle with Wi-Fi or LTE uploads bundles itself: device-signed requests, a pinned server key, the relay's offer/chunk/commit/receipt | reserved for the firmware; server | **draft**: spec and vectors only, nothing implements it yet |
| [`share/v1`](share/v1/) | A portable trip export with redaction | reserved | **reserved, draft**: no design until the threat model has a sharing section |

## Versioning and releases

- The collection is versioned with semver and tagged `contracts-vX.Y.Z`, **only when
  contract content changes**. Documentation edits elsewhere do not release anything.
- Each protocol directory is versioned **independently** of the collection tag
  (`format/v3`, `enrolment/v1`, ...). A breaking change to a protocol adds a new directory
  (`format/v4`); it never edits an old one in place.
- **Breaking, for the collection:** removing or renumbering a field, record type or
  endpoint; changing a byte layout, a canonical form or a signing string; tightening what a
  consumer must accept. **Not breaking:** adding an optional field, a record type old
  readers skip, an endpoint, a vector, or clarifying prose.
- **Retention:** an old protocol directory stays for as long as any supported consumer
  still uses it. Firmware is the long pole (devices in cars update slowly), so a
  firmware-facing format stays at least until no supported firmware produces it. Each
  protocol's README states its support window.
- **Tags are protected and publishing is manual.** No CI job holds permission to tag or
  publish a contract; a maintainer does.

## Using them from another repository

A repository records exactly which contracts it was built against in `contracts.lock`
(the tag **and** the resolved commit), and fetches them into `.contracts/`. Everything that
needs the specs or vectors reads the directory named by **`CAIRN_CONTRACTS`**, which is
always *this* `contracts/` directory (the one that contains `format/`, `sync/`, `store/`),
never a checkout root. Unset, the tools look for `./contracts` (this repository) and then
`./.contracts/contracts` (a fetched copy). To change a contract and its implementation
together on one machine, point `CAIRN_CONTRACTS` at a local checkout; release builds
refuse an override.

## How a contract changes

```text
draft the contract change + the implementation branch
-> generate candidate vectors
-> review the spec and the expected outputs
-> candidate conformance checks pass
-> merge, then a maintainer tags contracts-vX.Y.Z
-> consumers bump their pin and release
```

For a byte-level change, **at least one independent consumer** must validate the candidate
vectors before tagging. A generator reproducing its own output proves determinism, not
correctness.

## Vectors

Deterministic and offline. They include negative cases (malformed input, authentication
failures, unsupported versions) because a consumer that only passes valid input has not
been tested. **Every key in them is a public test key committed to a public repository.
They protect nothing, and must never be used for anything real.**
