# oci-germination — latest published artifacts

Eleven OCI bundles ship from this repo: four `core-pg17-*` infrastructure images, four `pg17-pgrdf-*` extension bundles, and three web-bearing variants for CKP v3.8 development. All multi-arch (`linux/amd64` + `linux/arm64`), anonymous public pull. This file tracks the head of each. See [Repo packages view](https://github.com/orgs/sporaxis-com/packages?repo_name=oci-germination) for the full version matrix.

## ociger-ck-allinone — `v0.6.2`

CKP v3.8 development default: PostgreSQL 17 + pgRDF + pgCK + NATS (4222) + NATS WSS (9222) + pgckweb (FastAPI) + CK.Lib.Js mounted at `/cklib/`. Supervisor-orchestrated, scratch base. ⚠️ FastAPI process latent gap — Postgres/pgRDF/pgCK/NATS work; FastAPI dead. Use `static-cklib` (below) for a working web layer.

| arch  | Platform digest                                                            | Created (UTC)       |
|-------|----------------------------------------------------------------------------|---------------------|
| amd64 | `sha256:7ed89efdd82149b4f1cb1a70d5e0598306d979e2cb6d37775d4c87368bb5d57a`  | 2026-05-28 15:59:17 |
| arm64 | `sha256:1db8a6102e819b0a47350f31f9479c327f5096165b69d832822fabf63fef8d87`  | 2026-05-28 15:59:17 |

|                    |                                                                          |
|--------------------|--------------------------------------------------------------------------|
| Pull URI           | `ghcr.io/sporaxis-com/ociger-ck-allinone:v0.6.2`                         |
| Also tagged        | `latest`                                                                 |
| Index digest       | `sha256:0d37e27d1fb8104f72f19ddf247fb8696cec5c5e44e8b2a8a83a4a63d72b5b04`|
| Source bundle      | [`bundles/bundle-ck-allinone/`](./bundles/bundle-ck-allinone/)           |
| Repo packages view | https://github.com/orgs/sporaxis-com/packages/container/package/ociger-ck-allinone |

## ociger-pg17-pgrdf-pgck-static-cklib — `v0.6.2`

CKP v3.8-aligned web bundle: PostgreSQL 17 + pgRDF + pgCK + NATS + NATS WSS + Go static-server + CK.Lib.Js at `/cklib/`. No Python, no FastAPI. Browser ↔ kernel via NATS WSS; HTTP serves only static assets. Smoke-verified end-to-end.

| arch  | Platform digest                                                            | Created (UTC)       |
|-------|----------------------------------------------------------------------------|---------------------|
| amd64 | `sha256:2d39c43904213aa0dd800f4bc5550455a8a9606f4554cc39e206735963a181ed`  | 2026-05-28 15:58:06 |
| arm64 | `sha256:4b86849d8d84b31c79fc0a31e5080b1c61cb4e2ca31fe2b411903f7df5d9f616`  | 2026-05-28 15:58:06 |

|                    |                                                                          |
|--------------------|--------------------------------------------------------------------------|
| Pull URI           | `ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-static-cklib:v0.6.2`        |
| Also tagged        | `latest`                                                                 |
| Index digest       | `sha256:c4a5ecc2487f7314bc01d20b438070e564be50835139dc1abc5edc2bdc1143e6`|
| Source bundle      | [`bundles/bundle-pg17-pgrdf-pgck-static-cklib/`](./bundles/bundle-pg17-pgrdf-pgck-static-cklib/) |
| Repo packages view | https://github.com/orgs/sporaxis-com/packages/container/package/ociger-pg17-pgrdf-pgck-static-cklib |

## ociger-pg17-pgrdf-pgck-web-cklib — `v0.6.2`

Standard web bundle (no NATS): PostgreSQL 17 + pgRDF + pgCK + pgckweb (FastAPI) + CK.Lib.Js at `/cklib/`. Distroless base. ⚠️ Same FastAPI latent gap as ck-allinone. Use `static-cklib` for a working web layer.

| arch  | Platform digest                                                            | Created (UTC)       |
|-------|----------------------------------------------------------------------------|---------------------|
| amd64 | `sha256:a7a8c4a7b545beda2b16d8e3632777c27b85ba3a4d2f7caffc670f4644935e72`  | 2026-05-28 15:59:23 |
| arm64 | `sha256:89ec2db7835452f62383bbe2a501fe178c2e1788ec4993d5bbadc97045502224`  | 2026-05-28 15:59:23 |

|                    |                                                                          |
|--------------------|--------------------------------------------------------------------------|
| Pull URI           | `ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-web-cklib:v0.6.2`           |
| Also tagged        | `latest`                                                                 |
| Index digest       | `sha256:1a09707254de07a870cc15a69fe707b5ce3e0241244aaa683fdd01a16f1d6566`|
| Source bundle      | [`bundles/bundle-pg17-pgrdf-pgck-web-cklib/`](./bundles/bundle-pg17-pgrdf-pgck-web-cklib/) |
| Repo packages view | https://github.com/orgs/sporaxis-com/packages/container/package/ociger-pg17-pgrdf-pgck-web-cklib |

## ociger-pg17-pgrdf-pgck-nats-micro — `v0.1.2`

PostgreSQL 17 + pgRDF + pgCK + NATS (4222) + NATS WSS (9222). Scratch base. Canonical base for ck-allinone and static-cklib.

| arch  | Platform digest                                                            | Created (UTC)       |
|-------|----------------------------------------------------------------------------|---------------------|
| amd64 | `sha256:66ee152db3550fa8daf13a1b54a3131142c21937e356523e45ac5934d22bdc3b`  | 2026-05-27 17:02:57 |
| arm64 | `sha256:0304f9fb041d60a94572fcd552ce486867ccf434dd27de79a18cb713ba209eb1`  | 2026-05-27 17:02:57 |

|                    |                                                                          |
|--------------------|--------------------------------------------------------------------------|
| Pull URI           | `ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-nats-micro:v0.1.2`          |
| Also tagged        | `latest`                                                                 |
| Index digest       | `sha256:864fe4030c09241af1d0617933b42159b9857bd6d29706f7d92beb914189b511`|
| Source bundle      | [`bundles/bundle-pg17-pgrdf-pgck-nats-micro/`](./bundles/bundle-pg17-pgrdf-pgck-nats-micro/) |
| Repo packages view | https://github.com/orgs/sporaxis-com/packages/container/package/ociger-pg17-pgrdf-pgck-nats-micro |

## ociger-pg17-pgrdf-pgck-nats — `v0.1.2`

PostgreSQL 17 + pgRDF + pgCK + NATS + NATS WSS. Distroless base (shell + libc available).

| arch  | Platform digest                                                            | Created (UTC)       |
|-------|----------------------------------------------------------------------------|---------------------|
| amd64 | `sha256:a092c93b3883a25b19dfbf0b4187d6ef9cadb5854a71bc45268527a6e4720e65`  | 2026-05-27 17:02:49 |
| arm64 | `sha256:062ed7bb7d6cf17d8347d7ae211d7c1a9b0c5b38e3cd20bc3c6d51114af3e2a7`  | 2026-05-27 17:02:49 |

|                    |                                                                          |
|--------------------|--------------------------------------------------------------------------|
| Pull URI           | `ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-nats:v0.1.2`                |
| Also tagged        | `latest`                                                                 |
| Index digest       | `sha256:9da4affd7059475c61a18fc240a44836f22577760d25f8c74a5a9fa9fc87344f`|
| Source bundle      | [`bundles/bundle-pg17-pgrdf-pgck-nats/`](./bundles/bundle-pg17-pgrdf-pgck-nats/) |
| Repo packages view | https://github.com/orgs/sporaxis-com/packages/container/package/ociger-pg17-pgrdf-pgck-nats |

## ociger-pg17-pgrdf-pgck — `v0.1.2`

PostgreSQL 17 + pgRDF + pgCK preloaded by default (`shared_preload_libraries=pgrdf,pgck`). No NATS. Distroless base.

| arch  | Platform digest                                                            | Created (UTC)       |
|-------|----------------------------------------------------------------------------|---------------------|
| amd64 | `sha256:730befed00155461d3f77aeec3db3ae374399c80e4d6ecfd269020916d346818`  | 2026-05-27 17:02:34 |
| arm64 | `sha256:88050a9da292efcb75b81f9c45fdf059669ccc9d3138e4b30dec876a86e3ff14`  | 2026-05-27 17:02:34 |

|                    |                                                                          |
|--------------------|--------------------------------------------------------------------------|
| Pull URI           | `ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck:v0.1.2`                     |
| Also tagged        | `latest`                                                                 |
| Index digest       | `sha256:09ca1dcdb1c9f0a1fa67295e52c7966be6e03689b95fbe2d1c5de8abef2e3162`|
| Source bundle      | [`bundles/bundle-pg17-pgrdf-pgck/`](./bundles/bundle-pg17-pgrdf-pgck/)    |
| Repo packages view | https://github.com/orgs/sporaxis-com/packages/container/package/ociger-pg17-pgrdf-pgck |

## ociger-pg17-pgrdf — `v0.1.2`

PostgreSQL 17 + pgRDF 0.5.1. No pgCK. Distroless base.

| arch  | Platform digest                                                            | Created (UTC)       |
|-------|----------------------------------------------------------------------------|---------------------|
| amd64 | `sha256:7261cbb23a633bf73611334a3946d7e9612b77e8c8857ffdfbdb2afbaa92df16`  | 2026-05-27 17:02:15 |
| arm64 | `sha256:a2d9eeb5dc3a2b5d339ebc9c796f9eaf7807291213506759f34b8409ef32c03e`  | 2026-05-27 17:02:15 |

|                    |                                                                          |
|--------------------|--------------------------------------------------------------------------|
| Pull URI           | `ghcr.io/sporaxis-com/ociger-pg17-pgrdf:v0.1.2`                          |
| Also tagged        | `latest`                                                                 |
| Index digest       | `sha256:657c0fc273ac6bba00bf72c7080a90170e74b3c05da530e04ddfe5f3afa96115`|
| Source bundle      | [`bundles/bundle-pg17-pgrdf/`](./bundles/bundle-pg17-pgrdf/)             |
| Repo packages view | https://github.com/orgs/sporaxis-com/packages/container/package/ociger-pg17-pgrdf |

## ociger-core-pg17-nats-micro — `v0.1.1`

PostgreSQL 17 + NATS + NATS WSS. No extensions. Scratch base.

| arch  | Platform digest                                                            | Created (UTC)       |
|-------|----------------------------------------------------------------------------|---------------------|
| amd64 | `sha256:433dc848f636a953da7a977cad1c5ca95bb12252bdfe88d223b6a802a2f53c11`  | 2026-05-26 04:54:39 |
| arm64 | `sha256:8440a518a6c4dae703cd8434f80dafe8a7e184da03aaa7dc9b058b3191279be5`  | 2026-05-26 04:54:39 |

|                    |                                                                          |
|--------------------|--------------------------------------------------------------------------|
| Pull URI           | `ghcr.io/sporaxis-com/ociger-core-pg17-nats-micro:v0.1.1`                |
| Also tagged        | `latest`                                                                 |
| Index digest       | `sha256:d5b45d0abe70f7df63d159e331c2baefc6a78dd2098e8907cb5251070640e552`|
| Source bundle      | [`bundles/core-pg17-nats-micro/`](./bundles/core-pg17-nats-micro/)        |
| Repo packages view | https://github.com/orgs/sporaxis-com/packages/container/package/ociger-core-pg17-nats-micro |

## ociger-core-pg17-nats — `v0.1.1`

PostgreSQL 17 + NATS + NATS WSS. No extensions. Distroless base.

| arch  | Platform digest                                                            | Created (UTC)       |
|-------|----------------------------------------------------------------------------|---------------------|
| amd64 | `sha256:bd6f7feb19d75af12a9616d70bbb6b8113507fc652026ea069b9944b8a59001a`  | 2026-05-26 04:54:22 |
| arm64 | `sha256:1cb2e4d5fa0a57cb1d2c49ab0f57ad83cc156f1e7ca1881a1b3dca65c8f10fa8`  | 2026-05-26 04:54:22 |

|                    |                                                                          |
|--------------------|--------------------------------------------------------------------------|
| Pull URI           | `ghcr.io/sporaxis-com/ociger-core-pg17-nats:v0.1.1`                      |
| Also tagged        | `latest`                                                                 |
| Index digest       | `sha256:8732386137c50770ca67ca74d8066ec7948b7294c1c1bdb40d055be88f6481ea`|
| Source bundle      | [`bundles/core-pg17-nats/`](./bundles/core-pg17-nats/)                    |
| Repo packages view | https://github.com/orgs/sporaxis-com/packages/container/package/ociger-core-pg17-nats |

## ociger-core-pg17-micro — `v0.1.1`

PostgreSQL 17 only. No extensions, no NATS. Scratch base. Smallest postgres bundle in this repo.

| arch  | Platform digest                                                            | Created (UTC)       |
|-------|----------------------------------------------------------------------------|---------------------|
| amd64 | `sha256:a6ceb42fb410838869c0969b6f35978803e47ab0904fa834777953f31dc503dd`  | 2026-05-26 04:54:04 |
| arm64 | `sha256:7cb75e2818f0be317922b5c7440658d69f2a15706808f48d6eb20c3a5c092350`  | 2026-05-26 04:54:04 |

|                    |                                                                          |
|--------------------|--------------------------------------------------------------------------|
| Pull URI           | `ghcr.io/sporaxis-com/ociger-core-pg17-micro:v0.1.1`                     |
| Also tagged        | `latest`                                                                 |
| Index digest       | `sha256:730abc0ca4104fbb9da37228ca9e941bd59d03e997ca5ae73ac06636ad0ba92c`|
| Source bundle      | [`bundles/core-pg17-micro/`](./bundles/core-pg17-micro/)                  |
| Repo packages view | https://github.com/orgs/sporaxis-com/packages/container/package/ociger-core-pg17-micro |

## ociger-core-pg17-min — `core-pg17-v0.1.2`

PostgreSQL 17 only. Distroless base (shell + libc available).

| arch  | Platform digest                                                            | Created (UTC)       |
|-------|----------------------------------------------------------------------------|---------------------|
| amd64 | `sha256:dbc46866349ac9a30b1148cda677e250010817d853a47e8bc9b8ecb356fb84ba`  | 2026-05-25 18:16:47 |
| arm64 | `sha256:3d83944a8290d44c9e28d99f2e660e2c158171b190a8e386bd56884b1ef33974`  | 2026-05-25 18:16:47 |

|                    |                                                                          |
|--------------------|--------------------------------------------------------------------------|
| Pull URI           | `ghcr.io/sporaxis-com/ociger-core-pg17-min:core-pg17-v0.1.2`             |
| Also tagged        | `latest`                                                                 |
| Index digest       | `sha256:bd11e0bc6b39577e1e8e946dc28fcf3a7cea24524472a97c68c408a80ef4e3ac`|
| Source bundle      | [`bundles/core-pg17/`](./bundles/core-pg17/)                              |
| Repo packages view | https://github.com/orgs/sporaxis-com/packages/container/package/ociger-core-pg17-min |

## Pin policy

- `latest` tracks the **most recent published tag** on each bundle's production image name.
- Tagged versions are immutable on GHCR.
- All bundles are multi-arch manifest lists — pulling by the multi-arch `latest` or `vX.Y.Z` tag resolves to the right platform automatically.
- See [`SEMANTIC-VERSIONING.md`](./SEMANTIC-VERSIONING.md) for the versioning contract and [`SPEC.OCIGERMI.TRACKS.DEVEL.v1.0.md`](./SPEC.OCIGERMI.TRACKS.DEVEL.v1.0.md) for the production/devel track distinction.
