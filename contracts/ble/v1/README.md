# BLE v1

The dongle's GATT service. [`spec.md`](spec.md): phone GPS in, status and live data out.
[`offload.md`](offload.md): how sealed bundles leave a dongle that has no network (the phone
pulls them over BLE and uploads them), including the receipt gate.

- **Vectors:** [`vectors/golden/`](vectors/golden/) (companion frames, written from an
  independent implementation) and [`vectors/offload/`](vectors/offload/) (offload frames,
  generated from the firmware's own protocol module and held there by a test).
- **Status:** the companion protocol is stable. The offload protocol is implemented by the
  firmware and a Go reference client and tested end to end; the phone app has not
  implemented it yet.
- **Support window:** for as long as firmware that serves it is supported.
