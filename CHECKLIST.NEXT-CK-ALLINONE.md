# Checklist — `ck-allinone` next version (target: ready-to-use out of the box)

**Status:** draft — open work for the next ck-allinone release (target `v0.7.3` or `v0.8.0`)
**Authored:** 2026-06-03
**Background:** a brand-new pod pulled from `ghcr.io/sporaxis-com/ociger-ck-allinone:v0.7.2`, started with no env overrides and a fresh volume, comes up with PG + NATS + WSS + busybox httpd alive but with **no extensions created, no `pgck.nats_url` set, no NATS action dispatcher subscribed**. The official smoke test in `scripts/smoke-ck-allinone.sh` confirms this is by design — step ② of the smoke runs `CREATE EXTENSION pgcrypto; pgrdf; pgck;` itself, meaning the image's smoke certifies *composition*, not *ready-to-use behaviour*.

The goal of this checklist: when a consumer runs `docker run -p 8000:8000 -p 5432:5432 … ghcr.io/sporaxis-com/ociger-ck-allinone:<next>` against a fresh volume and points a browser at `http://localhost:8000/`, they should see a living CKP runtime with the dispatcher answering verbs — no manual setup, no consumer-side SQL.

---

## A. Image-side auto-bootstrap (binding)

On first boot (fresh volume — detected by absence of a `.ck-allinone.bootstrapped` marker file inside `$PGDATA`), the supervised entrypoint MUST:

- [ ] **A1.** Wait for postgres to accept connections.
- [ ] **A2.** Execute `CREATE EXTENSION IF NOT EXISTS pgcrypto;` in the bootstrap database (default `postgres`).
- [ ] **A3.** Execute `CREATE EXTENSION IF NOT EXISTS pgrdf;`
- [ ] **A4.** Execute `CREATE EXTENSION IF NOT EXISTS pgck CASCADE;`
- [ ] **A5.** Execute `ALTER SYSTEM SET pgck.nats_url = 'nats://127.0.0.1:4222';` (or equivalent — pin via the pgCK contract, which may evolve).
- [ ] **A6.** Reload (`SELECT pg_reload_conf();`) — DO NOT restart; the supervisor owns the postgres process lifecycle.
- [ ] **A7.** Touch `$PGDATA/.ck-allinone.bootstrapped` so subsequent boots short-circuit (idempotency).
- [ ] **A8.** On re-boots with the marker present, skip A2–A7 entirely (no-op). Consumers retain control of any post-bootstrap state.

Implementation note: the cleanest place is a new s6 `oneshot` service that depends on `postgres` longrun being ready. Likely `s6-services/bootstrap-ckp/type = oneshot` + a `run` script that `psql`s through a sidecar OR uses `pg_isready` + `psql` from a tiny static binary we COPY in. (NB: pg_base ships no `psql` — see C2.)

## B. NATS action dispatcher (binding)

The `nats-relay` (or whichever process pgCK ships that subscribes to `input.kernel.pgCK.action.>` and round-trips verbs to the postgres backend) MUST be running before the container reports ready.

- [ ] **B1.** Confirm with pgCK upstream which process owns the dispatcher subscription, and where its binary / launcher lives. (May require a NOTIFY to pgCK.)
- [ ] **B2.** If the dispatcher is a separate process: add an s6 longrun service for it under `s6-services/dispatcher/`.
- [ ] **B3.** If the dispatcher is a pg background worker triggered by `pgck.nats_url`: A5 above is sufficient — verify with smoke.
- [ ] **B4.** Smoke (see D below) must publish a test verb and assert a reply within ~2 seconds.

## C. UX: `http://localhost:8000/` — WSS connects + exchanges a message (binding)

The page connects to NATS over WebSocket and proves the round-trip works. That's it.

- [ ] **C1.** Ship `/app/index.html` (served at `http://localhost:8000/`) that:
  - Opens a NATS-WSS connection to `ws://${location.hostname}:9222` on page load (derives the host from `location` so any port-mapping works).
  - Subscribes to a unique test subject (e.g. `ck.probe.<random>`).
  - Publishes a message to that same subject.
  - When the subscription delivers the message back, renders **"✓ WSS round-trip OK"** into the DOM with the bytes received.
  - If the connection fails or the round-trip doesn't return within 3 s, renders **"✗ WSS round-trip failed"** with the error.
- [ ] **C2.** `/` serves C1's `index.html` (busybox httpd picks up `index.html` by default; if not, adjust the `-h` root or layout).
- [ ] **C3.** No JS build. Inline `<script type="module">` + raw HTML. The CK.Lib.Js NATS client is already at `/cklib/*` and is the right thing to import.
- [ ] **C4.** **Acceptance (binding):** open `http://localhost:<host-port>/` in a fresh browser tab on a fresh container → the page shows ✓ within 3 seconds, no consumer setup. Headless equivalent in CI: spin up the container, point a Playwright/Puppeteer headless Chromium at `/`, assert the DOM contains `✓ WSS round-trip OK` within 5 s.

## D. Smoke test gate update (binding)

`scripts/smoke-ck-allinone.sh` currently certifies composition. Extend to certify ready-to-use behaviour.

- [ ] **D1.** Drop step ② (`CREATE EXTENSION …` issued by the smoke). Replace with an **assertion** that pgrdf + pgck are ALREADY installed in the bootstrap database after the container reports ready.
- [ ] **D2.** Assert `SHOW pgck.nats_url;` returns the configured URL (not "unrecognized").
- [ ] **D3.** Publish a verb to `input.kernel.pgCK.action.<something-safe>` via `nats-cli` in a sidecar and assert a reply on `output.<verb>` within 2 seconds.
- [ ] **D4.** HTTP GET `/` returns 200 (the new landing — C1).
- [ ] **D5.** HTTP GET `/cklib/ck-client.js` still returns 200 (regression check).
- [ ] **D6.** Keep ⑥ (no Python) and ⑦ (PID 1 = s6-svscan) intact.

## E. CI gate (per SPEC.OCI.BUNDLE.v0.4 §4.4 extension)

- [ ] **E1.** Run the new smoke (D) inside `.github/workflows/build-bundles.yml` between the image build and the SLSA attest step. **The image must not advance to attestation if D1–D6 don't pass** — same shape as the existing Python-free gate.
- [ ] **E2.** `skip_render` bundles will need this on their own release workflows too (currently the pgrdf-* line uses dedicated workflows; ck-allinone uses build-bundles.yml).

## F. Documentation

- [ ] **F1.** README "Run" section: replace the `psql … CREATE EXTENSION` example with a one-liner that just `docker run`s + `curl`s `/` to confirm the dispatcher is alive.
- [ ] **F2.** SPEC.OCI.BUNDLE.v0.4 §9.1 worked example: append an `auto_bootstrap: true` field on the canonical bundle.yaml shape.
- [ ] **F3.** PROVENANCE.md: no change needed — attestation flow is unaffected.
- [ ] **F4.** LATEST.md description for `ociger-ck-allinone` (in `tools/render-latest-md.py`): drop the stale "FastAPI process latent gap" copy (left over from v0.6.x) and write a one-paragraph that says "ready to use; browse to :8000".

## G. Cross-repo coordination (NOTIFIES, per SPEC v0.4 §7)

- [ ] **G1.** Scan `pgCK/_WIP/NOTIFIES.oci-germination.*` for any unanswered inbound about the dispatcher. RESPOND adjacent to source first.
- [ ] **G2.** If pgCK doesn't already ship a documented `pgck.nats_url` default OR the dispatcher start path is not yet codified, write `_WIP/NOTIFIES.pgCK.next.dispatcher-bootstrap-contract.md` asking upstream to (a) confirm the GUC name + default; (b) confirm the dispatcher's launcher path; (c) confirm idempotency on re-boot.
- [ ] **G3.** Once pgCK responds, lock the GUC name + dispatcher entry in the bootstrap oneshot.

## H. Versioning

- [ ] **H1.** If A–D land without breaking `docker exec psql -c 'CREATE EXTENSION pgck;'` for existing consumers (it should be a no-op via IF NOT EXISTS), ship as **`release-ck-allinone-v0.7.3`** — minor pin bump on consumers' side.
- [ ] **H2.** If A5 changes the GUC name or the dispatcher start path breaks existing consumers, ship as **`release-ck-allinone-v0.8.0`** — major bump.
- [ ] **H3.** Same release wave also bumps the bundle.yaml `description:` to drop the "supervised by s6, scratch base" boilerplate in favour of "ready-to-use out of the box".

## I. Stretch / nice-to-have (post-v0.7.3)

- [ ] **I1.** `static-cklib` v0.7.x: same auto-bootstrap (currently same gap exists).
- [ ] **I2.** Surface "Production use" in LATEST.md as a green check / red cross visual.
- [ ] **I3.** Make the s6 `bootstrap-ckp` oneshot's output structured (one JSON-line per step) so the boot log is greppable.

---

## Acceptance test (one paragraph)

A reviewer who has never touched the project runs:

```bash
docker volume create ck-allinone-test
docker run --rm -d --name test \
  -v ck-allinone-test:/var/lib/postgresql/data \
  -p 8000:8000 -p 5432:5432 -p 4222:4222 -p 9222:9222 \
  ghcr.io/sporaxis-com/ociger-ck-allinone:<next>
sleep 15
curl -s http://localhost:8000/ | grep -qi "ckp" && echo "✓ landing alive"
psql -h 127.0.0.1 -U postgres -d postgres -c \
  "SELECT extname FROM pg_extension WHERE extname IN ('pgrdf','pgck');"
# Expected: pgrdf, pgck both listed.
```

If `curl /` shows life AND `psql` shows both extensions present without any `CREATE EXTENSION` being run, we've cleared the bar.

---

**Cross-references:**

- Current smoke: `scripts/smoke-ck-allinone.sh`
- Spec: `SPEC.OCI.BUNDLE.v0.4.md` §1.4 (Delta composition), §4 (CI gates), §9.1 (worked example to extend)
- Tracks: `SPEC.OCIGERMI.TRACKS.DEVEL.v1.0.md` — this checklist's deliverable is a production-track promotion gate, not a devel-only feature
- NOTIFIES protocol: `SPEC.NOTIFIES.v0.3` (authored in CK.Lib.Js) for G2
