# Changelog

Release history for the two bundles this repo currently cuts under the
**CKP v3.9 Critical Isolation Alpha** track:

- `ociger-ck-allinone` — the consumer-facing all-in-one (`ghcr.io/sporaxis-com/ociger-ck-allinone`)
- `ociger-pg17-pgrdf-pgck-nats-micro` — the pg base it consumes (`ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-nats-micro`)

## Format

Every release **attempt** is recorded — both successful and failed.

- **SHIPPED** entries published to GHCR and verified attested. Each has a git tag and a digest.
- **FAILED** entries hit a CI gate before any artifact reached GHCR. They have no git tag; their context is captured here so the gap in the version sequence is explained.

Each entry carries three lines so the historical record reads cleanly:

- **Tried** — the intent of the cut (what was being changed and why).
- **Tested** — the CI gates that ran and their outcome. For pg_base that's: Go unit tests, generator round-trip, generated-files-committed assertion, pgRDF preload lint, multi-arch build, **10-check smoke** (pg ready, pgRDF + pgCK + pgcrypto installed, version-match check, NATS core PONG, NATS WSS port up), SLSA Build Provenance v1 attest. For ck-allinone the smoke is bundle-specific: pg ready, §A auto-bootstrap extensions, NATS core, NATS WSS, busybox httpd serves `/cklib/*`, root WSS round-trip landing, WSS round-trip from sidecar, **§B4 dispatch-bridge round-trip** (input.kernel.<K>.action.<verb> → ckp.dispatch → result.kernel.<K>.<verb>), Python-free check, PID 1 = s6-svscan.
- **Verdict** — SHIPPED with digest + attestation status, or FAILED with the specific step that failed and the cause.

See [PROVENANCE.md](./PROVENANCE.md) §Release attempt policy for the rules that govern this file and how it interacts with `LATEST.md`.

## Policy summary (full text in PROVENANCE.md)

1. **Version numbers are monotonic and never reused.** A failed CI attempt at `vN.M.K` permanently retires that number. The next attempt is `vN.M.(K+1)`, never a re-push of `vN.M.K`.
2. **CHANGELOG.md captures every attempt; git tags are created only for successful attempts.** Gaps in the version sequence are explained here.
3. **LATEST.md advances only when SLSA Build Provenance v1 verifies the GHCR digest** — the auto-regen workflow is the only sanctioned writer.

> **Note on transition:** the **2026-06-10 / 2026-06-11 waves below include two entries (`pg_base v0.1.8` and `pg_base v0.1.11`) where a failed first attempt was retried under the same tag** before this policy was set. Under the policy now in force those would have advanced to the next version number instead. The honest history is recorded as-is.

---

## ociger-ck-allinone

### v0.7.15 — 2026-06-11 — FAILED (Dockerfile COPY still missing; image pushed pre-gate then failed smoke — unattested v0.7.15 + :latest leaked to GHCR)
- **Tried:** land the init.sql dropped from v0.7.14, promote ⑤d to the STRONG seal gate (ok:true + proof_digest, no PENDING branch), add the ⑤e governance-plane gate (propose → vote → apply → epoch advance, per CKP v3.9 §5 — verified working end-to-end in local probes as ck_participant, including over the NATS relay).
- **Tested:** CI smoke FAILED at ②d (demo kernel bootstrap) — `could not open file "/ontology/task.ttl"`. The commit carried init.sql + the new smoke, but the v0.7.14 autostash had dropped FOUR files, and only init.sql + smoke were re-committed: the Dockerfile (`COPY …/ontologies/ /ontology/`) and the relay's 2-arg switch (main.go) were still uncommitted. The new ②d gate caught the missing fixtures exactly as designed.
- **Cause (two distinct defects):** (1) incomplete recovery from the v0.7.14 autostash incident; (2) the workflow pushed the multi-arch manifest BEFORE the smoke gate, so the failed run still published an **unattested** `v0.7.15` + moved `:latest` to it on GHCR. `LATEST.md` correctly refused to advance (attestation gate), but registry `:latest` pointed at an ungated artifact until v0.7.16 superseded it.
- **Fix (lands in v0.7.16):** commit the remaining two files; restructure `build-bundles.yml` to gate-before-push (build native → Python gate → smoke → only then push multi-arch + attest), making a leaked unattested `:latest` structurally impossible; ②d now prints the swallowed import error on failure.
- **Verdict:** FAILED. CI run 27367366946. Version number spent per PROVENANCE rule 9.

### v0.7.14 — 2026-06-11 — SHIPPED **DEFECTIVE** (4 files dropped from the release commit; superseded)
- **Post-ship correction (same day):** the release commit was assembled with `git pull --rebase --autostash` run between `git add` and `git commit`; the autostash re-application unstaged FOUR of the six intended files — `init.sql` (bootstrap_kernel CALL + role-floor grants), the `Dockerfile` (`/ontology/` fixtures COPY), `cmd/ociger-pgck-relay/main.go` (the 4-arg→2-arg GOVERNED dispatch switch), and the smoke upgrades. Only the vendored ontology TTL files landed. The published image therefore still runs the v0.7.13-era relay (4-arg stub) with no kernel bootstrap: every seal path errors `relation "ckp.ledger" does not exist`, and dispatch via the relay still returns the `verb not governed yet (CI-B)` stub envelope. CI smoke passed because the tag also carried the OLD soft smoke. Lessons, now structural: (1) ship order is commit→rebase→push, never autostash with a non-empty index; (2) verify `git show <tag>:<file>` for artifact-defining files before tagging; (3) a smoke PENDING branch must match a SPECIFIC upstream error signature, never a class as broad as `does not exist`. **Do not deploy v0.7.14.** The "cascade layer 5" claim in the original entry below is retracted — with the dropped files actually in place, the full seal works as ck_participant via psql AND via the NATS relay, and the v3.9 governance plane (propose→vote→apply, epoch advance) round-trips end-to-end on stock pgCK 0.4.1.
- *Original entry as written at ship time (its claims describe the intended content, which the published artifact did not carry):*
- **Tried:** close 4 of the 5 install-cascade layers between v0.7.13's relay calling the 4-arg CI-A-2 stub and a working end-to-end seal. (a) Relay now calls the GOVERNED 2-arg `ckp.dispatch(verb, payload)` per CK.Lib.Js verify-v150 RESPONSE-4. (b) init.sql `CALL ckp.bootstrap_kernel()` + `ALTER TABLE … OWNER TO ck_substrate` for the lazy-created instances/ledger/proof/outbox + `GRANT USAGE ON SCHEMA pgrdf` to the floor roles. (c) Vendored 8 pgCK ontology TTL fixtures (task, goal, core, affordance, delegation, delivery, proof, validate) at `/ontology/` so `ckp.import_module` resolves out of the box. (d) Four NEW substantive smoke gates (②a pgRDF round-trip parse+count, ②b CI-A role floor shape, ②c CI-B registry populated, ②d demo kernel bootstrap via vendored ontologies) + ⑤d upgraded to assert the relay called the GOVERNED 2-arg form (FAIL if it ever regresses to the 4-arg stub — the v0.7.11..v0.7.13 escape class).
- **Pins:** pg_base v0.1.11, cklib 1.4.2 (unchanged; this cut is dispatch-correctness + test surface).
- **Tested:** multi-arch build OK; smoke 12/12 green incl. **the four new substantive gates** + **⑤d's automated wrong-overload trap**; SLSA attest verified. Honest disclosure: ⑤d's seal step reports `OK GOVERNED-2-arg-reached PENDING-pgCK-install` — the relay calls the right overload + reaches the seal handler, but the handler errors with `relation "ckp.instances" does not exist` when invoked through the participant-role SECDEF chain. This is the 5th install-cascade layer that's upstream-gated; NOTIFY filed at pgCK detailing all five. The smoke gate tightens to `OK ok=true id=task-… proof_digest=…` automatically when pgCK ships the install fix.
- **Verdict:** SHIPPED. Digest `sha256:85475218c60ad4bd70a23d107f9d380360c64dee356cc99e258044dd9198a8de`. CI run 27356975675. `gh attestation verify` exit 0.
- **Regression trapping (closing the loop from this session's "how do we trap defective releases" thread):** ⑤d now FAILS the build if a future cut's relay calls the 4-arg stub. ②a fails if pgRDF parse/storage regresses. ②b fails if the role floor regresses. ②c fails if the affordance registry empties. ②d fails if kernel bootstrap via vendored ontology breaks. The bundle now proves substantive properties on every cut, not just "the container starts and the files are there."

### v0.7.13 — 2026-06-11 — SHIPPED
- **Tried:** composition-completeness cut. Fixes the bundled `/app/index.html` landing-page WSS probe so it works in both supported topologies (direct docker run with port-mapped `:9222`, and envoy/gateway-fronted with TLS + `/wss` upgrade) — v0.7.5..v0.7.12 hardcoded `ws://${location.hostname}:9222` which silently broke every envoy-fronted consumer. Adds three reserved web surfaces (`/app/web/`, `/app/web2/`, `/app/wss/`) so downstream consumers can rely on them being served by busybox httpd; each ships a self-identifying stub page. Adds two new smoke gates that would have caught both classes of regression on the first failing cut.
- **Pins:** pg_base v0.1.11, cklib 1.4.2 (unchanged from v0.7.12 — this cut is composition + smoke surface only).
- **Tested:** multi-arch build OK; "Verify no Python" OK; smoke green on the expanded gate set — ⑤ `/cklib/*` 200, **⑤+ `/web/`, `/web2/`, `/wss/` all 200 + self-identifying** (new gate), ⑤b landing 200 **+ both topology branches (`/wss` AND `:9222`) present in JS** (new gate), ⑤c WSS round-trip via sidecar, ⑤d §B4 dispatch-bridge round-trip. SLSA attest verified.
- **Verdict:** SHIPPED. Digest `sha256:f959c67b2144f5e5b7ecb20bd84b47383ce767e8ad8e8d9e82a29df0d4bd5d26`. CI run 27349274298.
- **First release under the new monotonic-version policy** (PROVENANCE rules 9 + 10); next release will be v0.7.14 regardless of outcome.

### v0.7.12 — 2026-06-11 — SHIPPED
- **Tried:** patch follow-on consuming the freshly-bumped pg_base v0.1.11 (pgRDF 0.5.43 + pgCK 0.4.1). No bridge or cklib change.
- **Pins:** pg_base v0.1.11, cklib 1.4.2, relay v0.7.11 generation.
- **Tested:** multi-arch build OK; "Verify no Python" OK; ck-allinone smoke 10/10 green (incl. §B4 dispatch-bridge round-trip — input → ckp.dispatch → result.kernel.pgCK.<verb> returns the typed envelope on a fresh volume); SLSA attest verified.
- **Verdict:** SHIPPED. Digest `sha256:9b10734cb12a2b22ad482aaa1afe26a43fa8690bd777020bebd98a8bf0d02c03`. CI run 27340539421 (build-bundles.yml). `gh attestation verify` exit 0.

### v0.7.11 — 2026-06-11 — SHIPPED
- **Tried:** replace `ociger-pgck-relay` from bus-level echo into a real NATS↔pgCK dispatch bridge. Subscribe `input.kernel.<K>.action.<verb>`; CALL `ckp.dispatch(verb, kernel_urn, payload::jsonb, identity)` over TCP+scram as `ck_participant`; publish typed reply on `result.kernel.<K>.<verb>` preserving inbound headers. Closes CK.Lib.Js NOTIFY ask #2 from the integration-gaps-block-v150 thread.
- **Pins:** pg_base v0.1.10, cklib 1.4.2.
- **Tested:** multi-arch build OK; "Verify no Python" OK; smoke 10/10 green; **§B4 was the substantive new gate** — verified end-to-end against a fresh volume that the bridge connects to pg as ck_participant, calls `ckp.dispatch`, and returns the typed envelope `{"ok":false,"error":"verb not governed yet (CI-B): task.create","kernel":"ckp://Kernel#pgCK","delegate":true}` on `result.kernel.pgCK.task.create`; SLSA attest verified.
- **Verdict:** SHIPPED. Digest `sha256:adba90e8f3dd1743b0919c1cfe28127e5b82f6334bb9f12e132a2b44424d1b32`. CI run 27338971702.
- **Local pre-tag note:** initial `go mod tidy` pulled `pgx v5.10.0`, which raised the `go.mod` `go` directive to `1.25` — broke the in-Dockerfile `golang:1.24-bookworm` builder. Caught and fixed locally before any tag push (pinned `pgx v5.5.5`, reset `go.mod` `go 1.24.4`). Did not consume a version number.

### v0.7.10 — 2026-06-10 — SHIPPED
- **Tried:** wave bumping all three deps in one cut — pgRDF 0.5.41 → 0.5.42, pgCK 0.3.4 → 0.4.0 (the "Critical Isolation ENFORCED" milestone, CI-A..CI-E all live), cklib 1.4.1 → 1.4.2 (per CK.Lib.Js NOTIFY).
- **Pins:** pg_base v0.1.10, cklib 1.4.2.
- **Tested:** multi-arch build OK; "Verify no Python" OK; smoke 10/10 green (§B4 dispatcher round-trip in the echo-relay shape — input → event.kernel.pgCK.<verb> — still passing on this cut).
- **Verdict:** SHIPPED. Digest `sha256:5356acabbe0ec002f887a50e91d65b21a74d46f33a567023d0929f1df4184002`. CI run 27332339483.

### v0.7.9 — 2026-06-10 — SHIPPED
- **Tried:** Track D governance landing — pgRDF 0.5.40 → 0.5.41, pgCK 0.3.3 → 0.3.4, cklib 1.4.0 → 1.4.1.
- **Pins:** pg_base v0.1.9, cklib 1.4.1.
- **Tested:** multi-arch build OK; "Verify no Python" OK; smoke 10/10 green.
- **Verdict:** SHIPPED. Digest `sha256:d9d8b8c03cedd77d8fa15b85112a814a9f41806396fa6470f77c1801cc56f6e0`. CI run 27310125400.

### v0.7.8 — 2026-06-10 — SHIPPED
- **Tried:** first cut on the v3.9 Critical Isolation Alpha contract — Track A role-floor + Track B sealed registry + Track C plan compiler. pgRDF 0.5.40, pgCK 0.3.3, cklib 1.4.0.
- **Pins:** pg_base v0.1.8, cklib 1.4.0.
- **Tested:** multi-arch build OK; "Verify no Python" OK; smoke 10/10 green.
- **Verdict:** SHIPPED. Digest `sha256:f5c76475281652538e6890953f21f7597d9cbd2c7fcfb6a1b3a186ed7a299935`. CI run 27305681133.

### Pre-v0.7.8 history
v0.7.0 … v0.7.7 + the v0.6.x line predate this changelog. See `git tag` history.

---

## ociger-pg17-pgrdf-pgck-nats-micro (pg_base)

### v0.1.11 — 2026-06-11 — SHIPPED (note: retried under same tag — see policy note)
- **Tried:** patch wave bringing pgRDF 0.5.42 → 0.5.43 + pgCK 0.4.0 → 0.4.1.
- **Tested:** Go unit tests OK; generator round-trip OK; generated-files-committed OK; pgRDF preload lint OK; multi-arch build OK; pg_base smoke 10/10 (pg ready, pgRDF 0.5.43 installed + pgRDF version match, pgCK 0.4.1 installed + pgCK version match, pgcrypto installed, parse_turtle pgatomic OK, NATS core PONG, NATS WSS port up).
- **Verdict:** SHIPPED. Digest `sha256:979c39f6ed025ea4f1ad992cf85f5ff71c06da609e0750d3101e453bb4c61dae`. CI run 27340383747.

### v0.1.11 (first attempt) — 2026-06-11 — FAILED
- **Tried:** same component pins as the SHIPPED entry above (pgRDF 0.5.43 + pgCK 0.4.1).
- **Tested:** Go unit tests OK; **`scripts/generate.sh` (the "Regenerate bundle outputs" step) FAILED with `yaml: line 6: mapping values are not allowed in this context`**. Smoke + build + attest never ran — the verify job aborted at the generator step.
- **Cause:** `bundles/bundle-ck-allinone/bundle.yaml` line 6 description contained unquoted colons (`"…dispatch-bridge relay: subscribe…"`, `"payload::jsonb"`) that YAML interpreted as mapping-key syntax.
- **Fix:** wrap the description in double quotes and soften one colon to an em-dash.
- **Verdict:** FAILED. CI run 27340232522. No artifact reached GHCR.
- **Policy note:** under the policy now in force this attempt would not have been retried under the same tag — it would have skipped to `v0.1.12`. The tag was force-moved at the time; CHANGELOG.md is the authoritative record of the failure.

### v0.1.10 — 2026-06-11 — SHIPPED
- **Tried:** pgRDF 0.5.41 → 0.5.42 + pgCK 0.3.4 → 0.4.0 (CI-A..CI-E all live).
- **Tested:** Go unit tests OK; generator round-trip OK; pgRDF preload lint OK; multi-arch build OK; pg_base smoke 10/10.
- **Verdict:** SHIPPED. Digest `sha256:029bb0f1ab5846fe45bd4b2cb02e6d8982573d392521d6b7674b077891e544e1`. CI run 27332143551.

### v0.1.9 — 2026-06-10 — SHIPPED
- **Tried:** pgRDF 0.5.40 → 0.5.41 + pgCK 0.3.3 → 0.3.4 (Track D governance).
- **Tested:** Go unit tests OK; generator round-trip OK; pgRDF preload lint OK; multi-arch build OK; pg_base smoke 10/10.
- **Verdict:** SHIPPED. Digest `sha256:d675e83f7577d8e06f6de0d7376da6d9a13b0122561a4ed59f750a4a5475d34e`. CI run 27309978027.

### v0.1.8 — 2026-06-10 — SHIPPED (note: retried under same tag — see policy note)
- **Tried:** pgRDF 0.5.28 → 0.5.40 + pgCK 0.2.2 → 0.3.3 — first wave on the v3.9 Critical Isolation Alpha contract.
- **Tested:** Go unit tests OK; generator round-trip OK; pgRDF preload lint OK; multi-arch build OK; pg_base smoke 10/10 (with the smoke script's expected-version defaults already bumped from the prior failed attempt's fix).
- **Verdict:** SHIPPED. Digest `sha256:1c38896d1dafdf2e7c196ecdc2697c41b7717b56b2033fcfa2286c692100f5da`. CI run 27305471818.

### v0.1.8 (first attempt) — 2026-06-10 — FAILED
- **Tried:** same component pins as the SHIPPED entry above (pgRDF 0.5.40 + pgCK 0.3.3).
- **Tested:** Go unit tests OK; generator OK; pgRDF preload lint OK; multi-arch build OK; **pg_base smoke FAILED at the "version-match check" sub-step** with `wrong-version: pgrdf available=0.5.40 expected=0.5.28`.
- **Cause:** `scripts/smoke-pg17-pgrdf-pgck-nats-micro.sh` had a hardcoded default `PGRDF_EXPECTED_VERSION=0.5.28` / `PGCK_EXPECTED_VERSION=0.2.2`. The new image carried 0.5.40 / 0.3.3, the assertion failed, smoke exited 1.
- **Fix:** bump the smoke script's default expected versions to track the new pins.
- **Verdict:** FAILED. CI run 27305317588. No artifact reached GHCR.
- **Policy note:** as with `v0.1.11`, this was retried under the same tag at the time. Under the current policy it would skip to `v0.1.9`.

### Pre-v0.1.8 history
v0.1.0 … v0.1.7 predate this changelog. See `git tag` history.

---

## Cross-bundle release cadence note

A `ck-allinone` cut almost always consumes the latest `pg_base` cut. The two version sequences move together:

| ck-allinone | pg_base | wave theme |
|---|---|---|
| v0.7.12 | v0.1.11 | pgRDF 0.5.43 + pgCK 0.4.1 patch follow-on |
| v0.7.11 | v0.1.10 | dispatch-bridge rewrite (only the relay changed) |
| v0.7.10 | v0.1.10 | pgCK 0.4.0 (CI-A..CI-E enforced) + cklib 1.4.2 |
| v0.7.9 | v0.1.9 | Track D governance |
| v0.7.8 | v0.1.8 | Track A+B+C alpha cut |
