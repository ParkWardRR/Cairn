# module/v1 vectors

Deterministic and offline. They contain **no keys of any kind** — a module is a
description of an interpretation, never a credential — so there is no public-test-key
warning to carry here.

## Written in JSON, read as YAML

A module manifest is a YAML file. These vectors are **JSON**, because JSON is a subset of
YAML: one file is valid input to both a YAML-based implementation and a JSON one, and the
checker in this repository needs no YAML parser. A module in `cairn-modules` will be
written as YAML, which is pleasanter to edit; the vectors are JSON so that every
implementation can run them.

The one exception is `invalid/bad-duplicate-key.yaml`, which **must** be YAML (see
"Coverage" below).

## Layout

```text
vectors/
├── valid-boost.json              a full module: requires, derives, metrics, views, queries, ui, ios
├── valid-fuel-mixture.json       several metrics, per-boot sampling, a grandfathered derivation
├── valid-places.json             ui and queries only: no metric, no engine field, no derivation
├── valid-manifest-only.json      a stub: identity and sources, nothing else
├── invalid/*.json                23 manifests that must be rejected
├── invalid/bad-duplicate-key.yaml  the one YAML-only case
└── queries/
    ├── valid-boost-queries.json  three named queries with bound parameters
    └── invalid/*.json            11 query files that must be rejected
```

A valid vector is a **bare document**. An invalid one is wrapped, so the expectation
travels with it:

```json
{
  "expect_error": "text the rejection must contain",
  "note": "why this case exists, when it is not obvious",
  "directory": "turbo",
  "manifest": { }
}
```

- `expect_error` is matched by **substring**, so an implementation may word its errors
  however it likes as long as it names the thing that is wrong.
- `directory` is the module's directory name, present only where the case is about the
  `id`-equals-directory rule — a flat vector file has no directory of its own.
- `queries_file` replaces `manifest` for a query-file case, and may carry `module` to
  name the owning manifest.

## `valid-manifest-only.json` is the first one to pass

A module may legitimately be **nothing but a manifest**: no metric, no engine field, no
derivation, not even a page. A validator that only accepts the full `boost` shape has
hard-coded an assumption that the `places` module already breaks, and that any
"just a page over data we already have" module would break again.

## Coverage

The invalid manifests cover each rule in [`../spec.md` §3](../spec.md#3-identity) plus the
schema-level enums:

| Group | Cases |
|---|---|
| Identity | `bad-schema`, `bad-id-uppercase`, `bad-id-underscore`, `bad-id-not-directory`, `bad-status`, `bad-no-name`, `bad-no-sources`, `bad-unknown-field` |
| A stub claims nothing | `bad-stub-claims-metric`, `bad-stub-claims-derive` |
| Vocabulary | `bad-engine-field` (a derived metric is not a capture field), `bad-store-column-unknown` |
| Derived columns | `bad-column-ref`, `bad-derive-type`, `bad-duplicate-derive` |
| Metrics | `bad-metric-sample`, `bad-metric-view-name`, `bad-metric-view-undeclared`, `bad-trip-insight-agg` |
| Paths and UI | `bad-view-path-escape`, `bad-route`, `bad-vehicle-scope`, `bad-nav-group` |

The query cases cover both parameter directions (`bad-undeclared-param`,
`bad-unused-param`), the read-only rule (`bad-not-select`, `bad-semicolon`), naming
(`bad-query-name`, `bad-param-name`, `bad-duplicate-name`), typing (`bad-param-type`) and
ownership (`bad-module-mismatch`).

### Not covered by vectors

Three rules cannot be expressed as a single-document vector, and are unit-tested in
`tools/modulecheck` instead:

- **Two modules owning one derived column** — needs two manifests.
- **A cycle in derivation order** — needs two manifests that read each other's columns.
  There is a companion test that a legitimate *chain* is still allowed, because
  `fuel-economy` genuinely needs the lambda `fuel-mixture` derives.
- **Two modules sharing an id.**

### The YAML case

`invalid/bad-duplicate-key.yaml` is the only vector that must be YAML. A duplicate
mapping key cannot be written in JSON, and many YAML parsers accept one silently, keeping
the last value — so a reviewer can see `status: verified` while the loader reads
`status: stub`. A parser that does not reject it is not safe to load a manifest with. The
Go checker reads the JSON vectors and skips it; any YAML-based implementation must reject
it.
