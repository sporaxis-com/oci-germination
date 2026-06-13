# oci-germination — Sporaxis-Com's Concept Kernel v3.9 container distribution

This repo ships the OCI container artifacts that make a **CKP v3.9 ([Critical Isolation](https://github.com/styk-tv/pgCK)) runtime** trivial to stand up: PostgreSQL 17 with `pgRDF` (the parallel-ingest RDF graph engine) and `pgCK` (governed-write + concept-kernel runtime) preloaded, NATS for the WebSocket transport browsers talk to, and CK.Lib.Js mounted at `/cklib/` so a page can become a concept-kernel client in one line.

The repo is a **layer-addition shop, not a compile shop**: pg base + extensions come pre-composed from upstream, this side just stacks `s6-overlay`, `busybox httpd`, and CK.Lib.Js on top to produce a marketplace-shaped image.

> **New here?** → **[GETTING-STARTED.md](./GETTING-STARTED.md)** is the adopter on-ramp: one `docker run`, the hello-kernel journey, and how a game / experiment / software project maps onto the substrate. The journey is runnable — [`examples/hello-kernel/`](./examples/hello-kernel/).

---

## Why concept kernels (light read)

A **Concept Kernel** is a small self-describing, self-governing being that owns a slice of meaning. It carries its own ontology (what types can exist here), its own affordances (what verbs can be invoked), its own instances (the data that lives under it), and — when it climbs the capability tiers — its own sealing and proof. In [BFO](https://basic-formal-ontology.org/) terms a kernel sits as a `BFO:0000040`-class material entity for the purposes of stable identity and part-of relations: it endures, it has parts (ontology / tool / data), and it can be addressed independently. In [PROV-O](https://www.w3.org/TR/prov-o/) terms a kernel is a `prov:Agent` whose affordances generate `prov:Activity` invocations that mutate `prov:Entity` instances under it, each activity producing a sealed, auditable record. The CKP v3.9 line makes this concrete: kernels live in the pgRDF graph, declare their shape in SHACL, expose verbs over NATS-WSS, and carry an append-only proof chain. The discipline lets you describe a real-world or game-world domain as a federation of sovereign-by-design beings rather than a sprawl of tables and endpoints. This repo ships the runtime container that hosts it.

### A gaming example (one sentence)

The design target: a `Space.Ship` is a kernel that declares the `Ship` class, the SHACL shape that says a ship has a `name` and a `hull_integrity`, and an affordance `Space.Ship.dock`; players invoke `ckp://Action#Space.Ship.dock` over WSS, the runtime resolves the URN, validates the input against the shape, mutates the addressed instance, and broadcasts the event — no REST endpoint, no per-type table, no bespoke server code. A sibling `Match` kernel composes several `Space.Ship` instances + a `Session` kernel into a live multiplayer round, governed end-to-end.

What the **published image does today** is the same machinery one capability short of that target: governed, sealed, proof-chained kernels with tasks + goals, and consensus-gated type evolution — first-class custom classes like `Ship` are the next pgCK milestone. [`GETTING-STARTED.md`](./GETTING-STARTED.md) §2 draws that boundary precisely, and [`examples/hello-kernel/`](./examples/hello-kernel/) runs the part that works today end to end.

---

## What ships from here

Three production bundles + one benchmark sibling + four core bundles (pg-only). All multi-arch (`linux/amd64`, `linux/arm64`), all SLSA Build Provenance v1 attested via GitHub Actions, all published to GHCR. Current heads:

| Bundle | Current tag | Last release | Role |
|---|---|---|---|
| **`ociger-ck-allinone`** | `v0.7.19` | 2026-06-13 | Marketplace-minimal CKP v3.9 Critical Isolation Alpha runtime — pg17 + pgRDF + pgCK + NATS + cklib, supervised by `s6-overlay`, web by `busybox httpd`, with the in-tree `ociger-pgck-relay` dispatch bridge wired to `ckp.dispatch`. **No Python, no FastAPI, scratch base.** Default for prod. |
| `ociger-pg17-pgrdf-pgck-nats-micro` | `v0.1.13` | 2026-06-13 | The shared base for `ck-allinone` — pg17 + pgRDF + pgCK + NATS in a scratch + selectively-copied bookworm layout. Tracks the upstream pgRDF / pgCK release cadence directly. |
| `ociger-pg17-pgrdf-pgck-static-cklib` | `v0.6.7` | 2026-05-31 | Same composition family but with the in-tree `ociger-static-server` Go binary instead of busybox httpd — kept for downstreams that already pin to it. **Frozen at the 2026-05-31 component pins**; next cut will roll forward when the static-cklib track is needed. |
| `ociger-pg17-pgrdf-pgck-nats` | `v0.1.7` | 2026-05-31 | Same as `-nats-micro` with distroless final (instead of scratch). |
| `ociger-pg17-pgrdf-pgck` | `v0.1.7` | 2026-05-31 | pg17 + pgRDF + pgCK, no NATS. |
| `ociger-pg17-pgrdf` | `v0.1.7` | 2026-05-31 | pg17 + pgRDF only. |
| `ociger-core-pg17-{nats,nats-micro,micro,min}` | `v0.1.2` / `v0.1.3` | 2026-05-28 | pg17 cores without extensions. |
| `ociger-pgck-bench` | `v0.1.1` | 2026-05-31 | **Sibling devel / benchmark image** (Python + FastAPI + pgck-web). Runs **outside** ck-allinone on the same NATS network; never used in prod. The only sanctioned home for Python in this group. |
| `ociger-pg17-pgrdf-pgck-web-cklib` | `v0.6.5` | 2026-05-29 | **Retired surface**. Superseded by `ck-allinone`; the `static-cklib` variant continues for downstreams that need the in-tree static server. Not advertised in `LATEST.md`. |

Component versions baked into the `v0.7.19` / `v0.1.13` heads (the actively maintained pair):

| Component | Version | Source |
|---|---|---|
| PostgreSQL | 17.10 (Debian bookworm build) | upstream |
| pgRDF | `0.6.3` — parallel bulk loader (`load_turtle(…, bulk_load => true)`, shipped 0.6.2); LUBM-benchmarked ingest at tens-of-millions-of-triples scale | [styk-tv/pgRDF](https://github.com/styk-tv/pgRDF), SLSA-attested |
| pgCK | `0.4.13` (CKP v3.9 Critical Isolation enforced — CI-A role floor + CI-B sealed registry + CI-C plan compiler + CI-D governance + CI-E typed reads) | [styk-tv/pgCK](https://github.com/styk-tv/pgCK), SLSA-attested |
| NATS server | `2.14.1` (core on `:4222` + WSS bridge on `:9222`) | upstream |
| CK.Lib.Js | `1.5.0` (dispatch-only client) | [ConceptKernel/CK.Lib.Js](https://github.com/ConceptKernel/CK.Lib.Js), SLSA-attested |
| s6-overlay | `3.2.3.0` | upstream |
| busybox | `1.36.1-musl` | upstream |
| `ociger-pg-launcher` (in-tree) | rolls with bundle | this repo |
| `ociger-pgck-relay` (in-tree dispatch bridge) | rolls with bundle (v0.7.11 generation) | this repo |

All non-frozen bundles now share **one** set of component pins via [`versions.yaml`](./versions.yaml) (the single source of truth — pgRDF 0.6.3, pgCK 0.4.13, cklib 1.5.0); the `scripts/check-versions.sh` drift gate fails CI if any Dockerfile disagrees. The pure-extension siblings (`pg17-pgrdf`, `-pgck`, `-pgck-nats`, `pg17-bookworm-pgrdf`) carry those pins in source and roll their *published* images forward when their per-bundle tags are next cut. `pg17-pgrdf-pgck-static-cklib` stays frozen on its older pins (reason recorded in `versions.yaml`).

See [`LATEST.md`](./LATEST.md) for the auto-rendered attestation-verified head of each bundle.

---

## Inside `ck-allinone:v0.7.19` — layer composition

The numbers below are illustrative — they were last measured against `v0.7.0` (the size profile is close enough to the current cut that the relative shape still holds; v0.7.11+ added the `pgx`-equipped relay binary, which moves the bundle from ~125 MiB to ~128 MiB). For exact byte counts on a specific tag, run `docker manifest inspect ghcr.io/sporaxis-com/ociger-ck-allinone:v0.7.19`.

Layer composition (compressed, amd64; 9 layers total, 5 inherited from `pg_base`, 4 added in this repo):

| # | MB (gz) | Source / role |
|---|---:|---|
| 1 | 35.23 | pg_base — pg17 binary + extensions + ICU/glibc/TLS/Kerberos libs (mixed: fleet + upstream) |
| 2 | 1.59 | pg_base — additional shared libs |
| 3 | 1.60 | pg_base — more shared libs |
| 4 | 6.52 | pg_base — `nats-server`, `ociger-pg-launcher`, `ociger-supervisor` |
| 5 | 0.00 | pg_base — `/etc/{passwd,group,nats/,…}` |
| 6 | 2.68 | Delta — s6-overlay (PID 1 supervisor) |
| 7 | 0.73 | Delta — `/bin/busybox` (httpd applet on `:8000`) |
| 8 | 0.06 | Delta — CK.Lib.Js → `/app/cklib/` |
| 9 | 0.00 | Delta — `/etc/s6-overlay/s6-rc.d/*` service definitions |
| **Σ** | **48.42** | |

The bulk of the image is **upstream baggage** (libicudata.so.72 alone is 30 MB uncompressed for Unicode collation data). The **fleet code** — the part this group authors — totals **~14 MB** uncompressed within layer 1, split:

### Fleet extensions by repo (within layer 1, uncompressed)

| File | pgRDF | pgCK |
|---|---:|---:|
| `<ext>.so` (Rust/pgrx-compiled) | **12.90 MB** | 0.80 MB |
| `<ext>--<ver>.sql` (PL/pgSQL governance) | 15.44 KB | **42.05 KB** |
| `<ext>.control` | 1.03 KB | 0.37 KB |
| **Subtotal** | **12.92 MB** | **0.84 MB** |

pgRDF's `.so` is 16× pgCK's (oxigraph + RDF stack statically linked). pgCK's SQL is 2.7× pgRDF's (PL/pgSQL governance code lives in SQL, not Rust). Together they are ~16 % of the layer; the other 84 % is the upstream pg17 binary, ICU character data, glibc, TLS, etc.

Net contribution added in this repo on top of `pg_base`: **~3.5 MB compressed** (s6-overlay + busybox + cklib + service definitions).

---

## Run

Pull and run the marketplace-minimal image:

```bash
docker run --rm -d \
  --name ck-allinone \
  -e PGDATA=/var/lib/postgresql/data \
  -v "$PWD/ck-allinone-data:/var/lib/postgresql/data" \
  -p 15432:5432 -p 18000:8000 -p 14222:4222 -p 19222:9222 \
  ghcr.io/sporaxis-com/ociger-ck-allinone:v0.7.19
```

Verify the runtime:

```bash
# Postgres + extensions
docker run --rm --network host -e PGPASSWORD= postgres:17-bookworm psql \
  -h 127.0.0.1 -p 15432 -U postgres -d postgres \
  -c "CREATE EXTENSION pgcrypto;
      CREATE EXTENSION pgrdf;
      CREATE EXTENSION pgck CASCADE;
      SELECT pgrdf.version(), pgck_version();"

# busybox httpd serving cklib
curl -I http://127.0.0.1:18000/cklib/ck-client.js   # → 200

# NATS reachable
nc 127.0.0.1 14222 < /dev/null                       # → INFO {...}
nc -zv 127.0.0.1 19222                               # WSS listening
```

Verify the attestation chain (Sigstore Rekor + GitHub Fulcio OIDC):

```bash
gh attestation verify oci://ghcr.io/sporaxis-com/ociger-ck-allinone:v0.7.19 \
  --repo sporaxis-com/oci-germination
```

For browser-side smoke (NATS-WSS + cklib), point any modern browser at `http://127.0.0.1:18000/cklib/` and open dev tools — the page imports `ck-page.js` which initialises a CKP client.

---

## Sibling: `ociger-pgck-bench` (devel / benchmark only)

Python + FastAPI + pgck-web — runs **alongside** ck-allinone on the same docker network, never inside it. Labelled `ck.bundle.role=bench` + `ck.bundle.never-prod=true` on the manifest:

```bash
docker network create ckp-net
docker run --rm -d --name ck-allinone --network ckp-net \
  ghcr.io/sporaxis-com/ociger-ck-allinone:v0.7.19
docker run --rm -d --name pgck-bench --network ckp-net -p 18001:8000 \
  ghcr.io/sporaxis-com/ociger-pgck-bench:v0.1.0
```

`pgck-bench` defaults to connecting to `ck-allinone` over the shared network (`PG=ck-allinone:5432`, `NATS=ws://ck-allinone:9222`). Override env vars to target other ck-allinone instances. Sibling, not sidecar: prod images stay Python-free.

---

## Repository layout

```
bundles/                     OCI bundle Dockerfiles + bundle.yaml + s6 service trees
cmd/                         ociger-pg-launcher, ociger-supervisor, ociger-static-server, ociger-gen,
                             ociger-pgck-relay
internal/supervisor/         supervisor profile selection (used by static-cklib only; ck-allinone uses s6)
scripts/                     build-* and smoke-* per bundle
.github/workflows/           release pipelines (one per bundle line; all attest via SLSA v1)
LATEST.md                    auto-rendered head per bundle (attestation-gated; no manual edits)
PROVENANCE.md                publishing rules + attestation chain
CONTRIBUTING.CI.md           tag/release flow
```

---

## Discipline

Headlines of the fleet packaging contract:

- **No manual GHCR pushes.** Every published image carries a verifiable SLSA Build Provenance v1 attestation produced by the `Build OCI Bundles` workflow (or its siblings). See [`PROVENANCE.md`](./PROVENANCE.md).
- **`LATEST.md` is auto-rendered.** It refreshes only after `gh attestation verify` accepts the digest. Manual edits are reverted.
- **This repo never compiles upstream code.** pg17, pgRDF, pgCK, NATS, CK.Lib.Js, pgck-web, s6-overlay, busybox all arrive pre-built. The only binaries this repo produces are tiny in-tree Go tools.
- **Prod images are Python-free.** The CI gate refuses any prod-track image carrying python / uvicorn / fastapi / venv. `ociger-pgck-bench` is the sole sanctioned home for Python and runs as a sibling to prod images, never embedded.
- **Prod Shape A composition = Delta.** Scratch final base, supervised by s6-overlay, web served by busybox httpd or equivalent statically-linked binary. No apt-install to satisfy upstream-binary NEEDED lists.
- **Every image carries `ck.bundle.role` and `ck.bundle.never-prod` manifest labels** so tooling can refuse a `never-prod=true` image into a prod environment.

---

## References

- BFO: <https://basic-formal-ontology.org/>
- PROV-O: <https://www.w3.org/TR/prov-o/>
- pgRDF: <https://github.com/styk-tv/pgRDF>
- pgCK extension + pgck-web: <https://github.com/styk-tv/pgCK>
- CK.Lib.Js: <https://github.com/ConceptKernel/CK.Lib.Js>
- Attestation policy: [`PROVENANCE.md`](./PROVENANCE.md)
- Tag / release flow: [`CONTRIBUTING.CI.md`](./CONTRIBUTING.CI.md)
