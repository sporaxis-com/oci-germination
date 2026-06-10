# Ontology Fixtures

This directory contains pinned ontology test fixtures vendored into this repository for reproducible container and extension tests.

These files belong to their respective upstream owners and authors. `oci-germination` does not claim ownership of the ontology content. The files are copied here only so tests can run against stable local TTL inputs without depending on live network fetches.

Current policy:

- Keep this folder small.
- Vendor only ontology files that are actively used by tests.
- Prefer authoritative shipped runtime ontology files over broader draft modeling sets.
- Do not fetch ontology files from live URLs during tests; load the TTL files that exist in this folder.

Import source used for the current snapshot:

- Upstream: `https://github.com/styk-tv/pgCK/blob/main/ontology/core.ttl`
- Imported on: `2026-05-25`

## Fixture Table

| Local file | Upstream owner | Upstream repository | Upstream source path | Canonical ontology namespace | License | SHA-256 | Size (bytes) | Intended test use |
| --- | --- | --- | --- | --- | --- | --- | ---: | --- |
| `ckp-v3.8-core.ttl` | Peter Styk / Concept Kernel | `styk-tv/pgCK` | `ontology/core.ttl` | `https://conceptkernel.org/ontology/v3.8/core#` | MIT | `2f49a475aca6d6afe29a69a2de5aa3d0a755fa1631d9b68d72d440548c779a4f` | 4151 | Minimal `3.8` load / validate / aggregation test fixture for future `pg17 + pgrdf + pgck` bundle tests |

## Notes

- `ckp-v3.8-core.ttl` is the current authoritative shipped runtime ontology file from `pgCK`.
- The split `ontology/*.ttl` files in the upstream `pgCK` repo are not vendored yet; this first iteration intentionally keeps the fixture surface minimal.
- If this file is re-vendored later, update the table row with the new source metadata and checksum in the same commit as the file change.
