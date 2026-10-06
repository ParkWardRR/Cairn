# Device uplink v1

[`spec.md`](spec.md): how a dongle with Wi-Fi or LTE uploads sealed bundles to the server by
itself. Each request is signed by the device's enrolled Ed25519 key, the server's TLS key is
pinned in firmware, and the exchange is the relay's offer, chunk, commit and receipt.
[`vectors/vectors.json`](vectors/vectors.json) pins four signed requests and eleven negative
cases.

- **Status:** draft. Specified and has vectors; nothing implements it, and the threat-model
  review ([issue 23](https://github.com/ParkWardRR/cairn-driving-log-selfhosted/issues/23)) has not
  happened. The exposure of a device-facing endpoint to the internet is an owner decision.
- **Support window:** none yet; a draft may change incompatibly until it is marked stable.
