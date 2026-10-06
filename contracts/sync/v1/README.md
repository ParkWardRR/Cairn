# App sync v1

[`spec.md`](spec.md): the phone's API to the server. Per-request signing, invitation
enrolment, push/pull/ack, the vehicle-scoped snapshot, trip summaries, and (section 13) the
**bundle relay** by which the phone uploads a dongle's bundles. Vectors: signing and
enrolment proofs, and whole request/response exchanges (signed and bearer push, pull, ack,
vehicle scoping, the bundle relay, and the negatives: bad signature, replay, wrong scope,
expired token, unknown vehicle, clock skew, revocation) in [`vectors/`](vectors/); the cases
are listed in [`vectors/README.md`](vectors/README.md).

- **Status: draft.** The server implements all of it and tests it; the iOS app does not yet.
  Per the release rules a contract stays `draft` until a server and the app both implement it
  and an independent consumer has validated the vectors.
- **Support window:** to be set when it leaves draft.
- ECDSA is randomised, so in `vectors.json` signatures are pinned as "must verify"; the
  deterministic parts (signing string, body hash, header layout) are pinned as "must equal".
  `exchanges.json` signs deterministically (RFC 6979) so the whole file is reproducible.
- `exchanges.json` is generated and checked by the server; **no independent consumer has
  validated it yet.** The iOS app must pass it before this contract leaves `draft`.
