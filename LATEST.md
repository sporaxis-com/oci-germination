# oci-germination — Latest Releases

Snapshot of the most recent published version of every OCI container in this repo. Regenerated when a new release ships.

All images:
- multi-platform (`linux/amd64` + `linux/arm64`)
- public, anonymous pull (no GHCR auth required)
- linked to `sporaxis-com/oci-germination` (visible at https://github.com/orgs/sporaxis-com/packages?repo_name=oci-germination)
- versioned per `SEMANTIC-VERSIONING.md` (2-number git tag + distance, or explicit 3-number tag)

---

## ck-allinone — CKP Development Default ⭐

PostgreSQL 17 + pgRDF + pgCK + NATS (4222) + NATS WSS (9222) + pgckweb (FastAPI) + CK.Lib.Js mounted at `/cklib/`. Supervisor-orchestrated, scratch base. Recommended default for Concept Kernel development.

| | |
|---|---|
| **Version** | `v0.6.2` |
| **Pull URI** | `ghcr.io/sporaxis-com/ociger-ck-allinone:v0.6.2` |
| **Also tagged** | `latest` |
| **Index digest** | `sha256:0d37e27d1fb8104f72f19ddf247fb8696cec5c5e44e8b2a8a83a4a63d72b5b04` |
| **Created (UTC)** | 2026-05-28 15:59:17 |
| **Platforms** | linux/amd64, linux/arm64 |
| **GHCR view** | https://github.com/orgs/sporaxis-com/packages/container/package/ociger-ck-allinone |

```bash
docker pull ghcr.io/sporaxis-com/ociger-ck-allinone:v0.6.2
```

⚠️ FastAPI process never starts in this bundle (latent Python-in-distroless gap). PostgreSQL + pgRDF + pgCK + NATS work fully. For a working web layer use `ociger-pg17-pgrdf-pgck-static-cklib` (below).

---

## pg17-pgrdf-pgck-static-cklib — Static-only web variant (v3.8-aligned)

PostgreSQL 17 + pgRDF + pgCK + NATS (4222) + NATS WSS (9222) + Go static-server + CK.Lib.Js at `/cklib/`. **No Python, no FastAPI.** Browser ↔ kernel via NATS WSS; HTTP serves only static assets. Aligns with CK.Lib.Js v3.8 architecture.

| | |
|---|---|
| **Version** | `v0.6.2` |
| **Pull URI** | `ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-static-cklib:v0.6.2` |
| **Also tagged** | `latest` |
| **Index digest** | `sha256:c4a5ecc2487f7314bc01d20b438070e564be50835139dc1abc5edc2bdc1143e6` |
| **Created (UTC)** | 2026-05-28 15:58:06 |
| **Platforms** | linux/amd64, linux/arm64 |
| **GHCR view** | https://github.com/orgs/sporaxis-com/packages/container/package/ociger-pg17-pgrdf-pgck-static-cklib |

```bash
docker pull ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-static-cklib:v0.6.2
```

Smoke-verified end-to-end: `/healthz` → `ok`, `/cklib/ck-client.js` serves CK.Lib.Js 1.3.0, `pgrdf.parse_turtle()` returns 1 (PgAtomic init OK), `has_python=none`.

---

## pg17-pgrdf-pgck-web-cklib — Standard web variant (FastAPI + cklib, no NATS)

PostgreSQL 17 + pgRDF + pgCK + pgckweb (FastAPI) + CK.Lib.Js at `/cklib/`. Distroless base. No NATS.

| | |
|---|---|
| **Version** | `v0.6.2` |
| **Pull URI** | `ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-web-cklib:v0.6.2` |
| **Also tagged** | `latest` |
| **Index digest** | `sha256:1a09707254de07a870cc15a69fe707b5ce3e0241244aaa683fdd01a16f1d6566` |
| **Created (UTC)** | 2026-05-28 15:59:23 |
| **Platforms** | linux/amd64, linux/arm64 |
| **GHCR view** | https://github.com/orgs/sporaxis-com/packages/container/package/ociger-pg17-pgrdf-pgck-web-cklib |

```bash
docker pull ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-web-cklib:v0.6.2
```

⚠️ Same FastAPI latent gap as ck-allinone. Use static-cklib for a working web layer.

---

## pg17-pgrdf-pgck-nats-micro — Triple-bundle + NATS, scratch base

PostgreSQL 17 + pgRDF + pgCK + NATS (4222) + NATS WSS (9222). Scratch final image. The canonical base for ck-allinone and static-cklib.

| | |
|---|---|
| **Version** | `v0.1.2` |
| **Pull URI** | `ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-nats-micro:v0.1.2` |
| **Also tagged** | `latest` |
| **Index digest** | `sha256:864fe4030c09241af1d0617933b42159b9857bd6d29706f7d92beb914189b511` |
| **Created (UTC)** | 2026-05-27 17:02:57 |
| **Size** | ~104.5 / 41.3 MiB (uncompressed / compressed) |
| **Platforms** | linux/amd64, linux/arm64 |
| **GHCR view** | https://github.com/orgs/sporaxis-com/packages/container/package/ociger-pg17-pgrdf-pgck-nats-micro |

```bash
docker pull ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-nats-micro:v0.1.2
```

---

## pg17-pgrdf-pgck-nats — Triple-bundle + NATS, distroless base

PostgreSQL 17 + pgRDF + pgCK + NATS (4222) + NATS WSS (9222). Distroless final image. Larger than `-micro` but with shell + libc available.

| | |
|---|---|
| **Version** | `v0.1.2` |
| **Pull URI** | `ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-nats:v0.1.2` |
| **Also tagged** | `latest` |
| **Index digest** | `sha256:9da4affd7059475c61a18fc240a44836f22577760d25f8c74a5a9fa9fc87344f` |
| **Created (UTC)** | 2026-05-27 17:02:49 |
| **Size** | ~170.1 / 65.1 MiB (uncompressed / compressed) |
| **Platforms** | linux/amd64, linux/arm64 |
| **GHCR view** | https://github.com/orgs/sporaxis-com/packages/container/package/ociger-pg17-pgrdf-pgck-nats |

```bash
docker pull ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-nats:v0.1.2
```

---

## pg17-pgrdf-pgck — pgRDF + pgCK, no NATS

PostgreSQL 17 + pgRDF + pgCK preloaded by default (`shared_preload_libraries=pgrdf,pgck`). Distroless base.

| | |
|---|---|
| **Version** | `v0.1.2` |
| **Pull URI** | `ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck:v0.1.2` |
| **Also tagged** | `latest` |
| **Index digest** | `sha256:09ca1dcdb1c9f0a1fa67295e52c7966be6e03689b95fbe2d1c5de8abef2e3162` |
| **Created (UTC)** | 2026-05-27 17:02:34 |
| **Size** | ~151.7 / 57.8 MiB (uncompressed / compressed) |
| **Platforms** | linux/amd64, linux/arm64 |
| **GHCR view** | https://github.com/orgs/sporaxis-com/packages/container/package/ociger-pg17-pgrdf-pgck |

```bash
docker pull ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck:v0.1.2
```

---

## pg17-pgrdf — pgRDF only

PostgreSQL 17 + pgRDF 0.5.1. Distroless base.

| | |
|---|---|
| **Version** | `v0.1.2` |
| **Pull URI** | `ghcr.io/sporaxis-com/ociger-pg17-pgrdf:v0.1.2` |
| **Also tagged** | `latest` |
| **Index digest** | `sha256:657c0fc273ac6bba00bf72c7080a90170e74b3c05da530e04ddfe5f3afa96115` |
| **Created (UTC)** | 2026-05-27 17:02:15 |
| **Platforms** | linux/amd64, linux/arm64 |
| **GHCR view** | https://github.com/orgs/sporaxis-com/packages/container/package/ociger-pg17-pgrdf |

```bash
docker pull ghcr.io/sporaxis-com/ociger-pg17-pgrdf:v0.1.2
```

---

## core-pg17-nats — PostgreSQL 17 + NATS, distroless

PostgreSQL 17 + NATS (4222) + NATS WSS (9222). No extensions. Distroless final image.

| | |
|---|---|
| **Version** | `v0.1.1` |
| **Pull URI** | `ghcr.io/sporaxis-com/ociger-core-pg17-nats:v0.1.1` |
| **Also tagged** | `latest` |
| **Index digest** | `sha256:8732386137c50770ca67ca74d8066ec7948b7294c1c1bdb40d055be88f6481ea` |
| **Created (UTC)** | 2026-05-26 04:54:22 |
| **Size** | ~157.9 / 60.3 MiB (uncompressed / compressed) |
| **Platforms** | linux/amd64, linux/arm64 |
| **GHCR view** | https://github.com/orgs/sporaxis-com/packages/container/package/ociger-core-pg17-nats |

```bash
docker pull ghcr.io/sporaxis-com/ociger-core-pg17-nats:v0.1.1
```

---

## core-pg17-nats-micro — PostgreSQL 17 + NATS, scratch

Same as above but on a scratch final image. Smaller.

| | |
|---|---|
| **Version** | `v0.1.1` |
| **Pull URI** | `ghcr.io/sporaxis-com/ociger-core-pg17-nats-micro:v0.1.1` |
| **Also tagged** | `latest` |
| **Index digest** | `sha256:d5b45d0abe70f7df63d159e331c2baefc6a78dd2098e8907cb5251070640e552` |
| **Created (UTC)** | 2026-05-26 04:54:39 |
| **Size** | ~92.2 / 36.4 MiB (uncompressed / compressed) |
| **Platforms** | linux/amd64, linux/arm64 |
| **GHCR view** | https://github.com/orgs/sporaxis-com/packages/container/package/ociger-core-pg17-nats-micro |

```bash
docker pull ghcr.io/sporaxis-com/ociger-core-pg17-nats-micro:v0.1.1
```

---

## core-pg17-micro — PostgreSQL 17 only, scratch

PostgreSQL 17 on a scratch final image. Smallest postgres bundle in this repo.

| | |
|---|---|
| **Version** | `v0.1.1` |
| **Pull URI** | `ghcr.io/sporaxis-com/ociger-core-pg17-micro:v0.1.1` |
| **Also tagged** | `latest` |
| **Index digest** | `sha256:730abc0ca4104fbb9da37228ca9e941bd59d03e997ca5ae73ac06636ad0ba92c` |
| **Created (UTC)** | 2026-05-26 04:54:04 |
| **Size** | ~73.8 / 29.2 MiB (uncompressed / compressed) |
| **Platforms** | linux/amd64, linux/arm64 |
| **GHCR view** | https://github.com/orgs/sporaxis-com/packages/container/package/ociger-core-pg17-micro |

```bash
docker pull ghcr.io/sporaxis-com/ociger-core-pg17-micro:v0.1.1
```

---

## core-pg17-min — PostgreSQL 17 only, distroless

PostgreSQL 17 on a distroless base image. Minimal but not scratch.

| | |
|---|---|
| **Version** | `core-pg17-v0.1.2` |
| **Pull URI** | `ghcr.io/sporaxis-com/ociger-core-pg17-min:core-pg17-v0.1.2` |
| **Also tagged** | `latest` |
| **Index digest** | `sha256:bd11e0bc6b39577e1e8e946dc28fcf3a7cea24524472a97c68c408a80ef4e3ac` |
| **Created (UTC)** | 2026-05-25 18:16:47 |
| **Size** | ~139.5 / 53.1 MiB (uncompressed / compressed) |
| **Platforms** | linux/amd64, linux/arm64 |
| **GHCR view** | https://github.com/orgs/sporaxis-com/packages/container/package/ociger-core-pg17-min |

```bash
docker pull ghcr.io/sporaxis-com/ociger-core-pg17-min:core-pg17-v0.1.2
```

---

## Compact index

| Image | Latest | Created | Composition |
|---|---|---|---|
| `ociger-ck-allinone` | `v0.6.2` | 2026-05-28 | pg17 + pgrdf + pgck + nats + nats-wss + pgckweb + cklib (⚠ FastAPI gap) |
| `ociger-pg17-pgrdf-pgck-static-cklib` | `v0.6.2` | 2026-05-28 | pg17 + pgrdf + pgck + nats + nats-wss + static-server + cklib |
| `ociger-pg17-pgrdf-pgck-web-cklib` | `v0.6.2` | 2026-05-28 | pg17 + pgrdf + pgck + pgckweb + cklib (⚠ FastAPI gap) |
| `ociger-pg17-pgrdf-pgck-nats-micro` | `v0.1.2` | 2026-05-27 | pg17 + pgrdf + pgck + nats + nats-wss (scratch base) |
| `ociger-pg17-pgrdf-pgck-nats` | `v0.1.2` | 2026-05-27 | pg17 + pgrdf + pgck + nats + nats-wss (distroless base) |
| `ociger-pg17-pgrdf-pgck` | `v0.1.2` | 2026-05-27 | pg17 + pgrdf + pgck (distroless base) |
| `ociger-pg17-pgrdf` | `v0.1.2` | 2026-05-27 | pg17 + pgrdf (distroless base) |
| `ociger-core-pg17-nats-micro` | `v0.1.1` | 2026-05-26 | pg17 + nats + nats-wss (scratch base) |
| `ociger-core-pg17-nats` | `v0.1.1` | 2026-05-26 | pg17 + nats + nats-wss (distroless base) |
| `ociger-core-pg17-micro` | `v0.1.1` | 2026-05-26 | pg17 (scratch base) |
| `ociger-core-pg17-min` | `core-pg17-v0.1.2` | 2026-05-25 | pg17 (distroless base) |
