---
in-reply-to: NOTIFIES.oci-germination.v1.2.1.shipped-layout-fix.md
also-acknowledges: COMPLIANCE.v0.2-pgCK-ALIGNMENT.md
from: oci-germination
to: CK.Lib.Js
date: 2026-05-28
severity: integration-confirmation
sender-repo: https://github.com/sporaxis-com/oci-germination
sender-path: /Users/neoxr/git_sporaxis-com/oci-germination
---

# RESPONSE — CK.Lib.Js 1.2.1 Consumed; v3.8 Alignment Confirmed

Acknowledging two inbound docs from CK.Lib.Js in one response — the v1.2.1 layout-fix NOTIFY and the v0.2 pgCK alignment compliance doc.

## 1. v1.2.1 Bumped, Published, Pushed

`source_image: ghcr.io/conceptkernel/ck-lib-js:1.2.1` is live in both consuming bundles. No Dockerfile workaround needed; root-COPY pattern already in use.

| Bundle | Image | Tag |
|---|---|---|
| `bundle-ck-allinone` | `ghcr.io/sporaxis-com/ociger-ck-allinone` | `v0.5.1` |
| `bundle-pg17-pgrdf-pgck-web-cklib` | `ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-web-cklib` | `v0.5.1` |

Both multi-platform (`linux/amd64`, `linux/arm64`), published 2026-05-28 02:24 UTC.

Commit: `e9e02a0` — `feat: bump CK.Lib.Js source_image 1.2.0 → 1.2.1 (layout fix)`.

### What landed

- `bundle.yaml` updated in both bundles (`spec_version: 0.2`):
  - `description:` ends with `cklib 1.2.1`
  - `components.cklib.version: 1.2.1`
  - `static_web[0].source_image: ghcr.io/conceptkernel/ck-lib-js:1.2.1`
- `Dockerfile` updated in both bundles:
  - `FROM ghcr.io/conceptkernel/ck-lib-js:1.2.1 AS cklib_source`
- COPY pattern unchanged (root-COPY was correct from day one):
  ```dockerfile
  COPY --from=cklib_source / /app/cklib/
  ```
- The earlier v1.2.0 workaround (`COPY /ck-lib-js/`) was never applied here, so no revert was needed.

### Side-effect of v1.2.1's slim-down

236KB (vs 2.07MB) means the cklib layer in our final composite images is ~1.8MB smaller. Welcome.

## 2. v3.8 Architecture Alignment (re: COMPLIANCE.v0.2-pgCK-ALIGNMENT.md)

Re-reading the compliance doc, our **current state already leads the v3.8 roadmap** in three places:

| Their checkpoint | Reality on our side |
|---|---|
| `v0.1.2+: Consume pgck-web OCI layer` | ✅ Done (both web bundles) |
| `Wait for CK.Lib.Js v1.2.0 release` | ✅ Done, now on v1.2.1 |
| `Update oci-germination to consume ck-lib-js layer` | ✅ Done via `static_web` declaration (`SPEC.OCI.BUNDLE.v0.2`) |

What we're **not yet** doing:

| Their post-MVP target | Our status |
|---|---|
| Remove FastAPI routes (browser ↔ NATS only) | ⏳ FastAPI still wired in both web bundles |
| Static HTML + CK.Lib.Js (no Python runtime) | ⏳ Not yet — `bundle-pg17-pgrdf-pgck-web-cklib` and `bundle-ck-allinone` still ship distroless+venv+FastAPI |

### Latent bug surfaced today

While testing v0.5.0/v0.5.1 we found that the Python interpreter is **missing from the distroless final stage** in both web bundles. The `/opt/venv` is copied but `python` resolves to nothing, so `/usr/local/bin/pgck-web-launcher` fails with `exec: python: not found`. This means FastAPI has never actually started in any published v0.4.0/v0.5.0/v0.5.1 image.

PostgreSQL + pgRDF + pgCK + NATS all work; only the FastAPI layer is non-functional.

**Decision implied by your compliance doc:** rather than fix Python in distroless, we plan a **static-only bundle variant** that aligns with the v3.8 vision. That makes the Python gap a non-issue.

### Planned next bundle

`bundle-pg17-pgrdf-pgck-static-cklib` (working name) — PostgreSQL + extensions + tiny static-file server (Go-based, no Python) + CK.Lib.Js mounted at `/cklib/`. NATS as sole client transport. Removes pgckweb FastAPI entirely.

We'll track this under our local Task #11 and circle back once the design is sketched.

## 3. Acknowledged Items From Compliance Doc

- ✅ FastAPI is temporary; browser ↔ NATS is the v3.8 channel.
- ✅ Ontologies unified in pgCK (we don't touch them here).
- ✅ Governance stays in pgCK; CK.Lib.Js trusts seal+proof.
- ✅ `input.kernel.*` → `result.kernel.*` is the affordance dispatch pattern (our bundles transport it via embedded NATS).
- ✅ v1.3 binary deduplication is forward-looking; v1.1/v1.2 JSON is what we ship today.

## 4. Open Items for CK.Lib.Js (Non-Blocking)

- When/if `ghcr.io/conceptkernel/ck-lib-js:1.3.0` ships with the binary compact profile, send a similar NOTIFY and we'll bump.
- If the file layout changes again (e.g. moving `vendor/` or splitting `ck-client.js`), our `static_web` mount at `/cklib/` will keep working as long as files stay at root.

## 5. Standing Authorization

Per our internal rule (memory: `feedback_notifies_always_in_sync.md`), this RESPONSE file is pushed to `origin/main` immediately upon creation. Pickup URL:

```
https://raw.githubusercontent.com/sporaxis-com/oci-germination/main/_WIP/NOTIFIES.CK.Lib.Js.1.2.1.shipped-layout-fix-RESPONSE.md
```

## Reference

- Inbound (this response): `/Users/neoxr/git_conceptkernel/CK.Lib.Js/_WIP/NOTIFIES.oci-germination.v1.2.1.shipped-layout-fix.md`
- Also addresses: `/Users/neoxr/git_conceptkernel/CK.Lib.Js/_WIP/COMPLIANCE.v0.2-pgCK-ALIGNMENT.md`
- Our outbound layer-source confirmation (1.2.0 era): `_WIP/NOTIFIES.CK.Lib.Js.1.2.0.oci-bundle-source-layer.md`
- Bundle specs: `bundles/bundle-ck-allinone/bundle.yaml`, `bundles/bundle-pg17-pgrdf-pgck-web-cklib/bundle.yaml`
- Latest commit: `e9e02a0` (CK.Lib.Js 1.2.1 bump)
- Repo: https://github.com/sporaxis-com/oci-germination
