# Device enrolment v1

[`spec.md`](spec.md): the sealed enrolment blob (225 bytes), its fingerprint and the USB
console protocol. [`vectors/vectors.json`](vectors/vectors.json) pins the blob byte for byte;
the firmware's C and the server's Go both reproduce it.

- **Status:** stable. Exercised on real hardware.
- **Support window:** for as long as any firmware that enrols this way is supported.
- No network credential travels over this channel: the dongle has none.
