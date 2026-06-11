# Vendored pgCK ontology fixtures

These TTL files are vendored from [styk-tv/pgCK](https://github.com/styk-tv/pgCK) at the `v0.4.1` source tree (commit-matching the extension we ship). pgCK's `ckp.import_module(module text, project text, root text)` function reads the file at `<root>/<module>.ttl` to seed the per-kernel SHACL shapes during `task.create` / `goal.create` bootstrapping.

pgCK 0.4.1's installed extension package does not ship these fixtures — `import_module` will error with `could not open file "/ontology/<module>.ttl"` without them. Until pgCK distributes the fixtures with the extension itself, the bundle vendors them so the dispatch path works out of the box.

## Files

| File | Bytes | Purpose |
|---|---:|---|
| `core.ttl` | 8123 | CKP v3.8 core vocabulary |
| `task.ttl` | 1851 | Task module — read by `ckp.import_module('task', …)` for `task.create` |
| `goal.ttl` | 1376 | Goal module — read by `ckp.import_module('goal', …)` for `kernel.create` |
| `affordance.ttl` | 984 | Affordance vocabulary |
| `delegation.ttl` | 685 | Delegation vocabulary |
| `delivery.ttl` | 770 | Delivery vocabulary |
| `proof.ttl` | 906 | Proof / sealing vocabulary |
| `validate.ttl` | 507 | Validation vocabulary |

## Provenance

Source: `https://github.com/styk-tv/pgCK/tree/main/ontology/`. License: MIT (matches pgCK). Pin: tracks the pgCK release the bundle is built against; updated in lockstep with `bundles/bundle-pg17-pgrdf-pgck-nats-micro/bundle.yaml`'s `pgck.version`.

## Upstream conversation

A NOTIFY has been filed asking pgCK to either (a) ship the fixtures with the extension package, or (b) document a stable runtime path the consumer is expected to populate. When pgCK adopts either, this vendor copy retires.

## In the image

The Dockerfile copies these files to `/ontology/` so `ckp.import_module(module, project)` resolves with no further configuration. The path matches pgCK 0.4.1's hard-coded expectation.
