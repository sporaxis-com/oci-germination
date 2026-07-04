# CI/CD Pipeline — OCI Bundle Builds

How releases are cut and what guards keep the published artifacts honest. The full policy lives in [PROVENANCE.md](./PROVENANCE.md); the versioning scheme lives in [SEMANTIC-VERSIONING.md](./SEMANTIC-VERSIONING.md); the historical record (good and bad) is in [CHANGELOG.md](./CHANGELOG.md). This file walks through the day-to-day "I want to ship a bump" loop.

## Automated builds via GitHub Actions

Every published GHCR artifact is built and pushed by a GitHub Actions workflow — never from a workstation. The workflows trigger on tag pushes whose prefix matches the bundle:

| Bundle | Tag pattern (workflow trigger) | Workflow file | Resulting image |
|---|---|---|---|
| `ck-allinone` | `release-ck-allinone-v*` | `build-bundles.yml` | `ghcr.io/sporaxis-com/ociger-ck-allinone:<v>` |
| `pg17-pgrdf-pgck-static-cklib` | `release-pg17-pgrdf-pgck-static-cklib-v*` | `build-bundles.yml` | `ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-static-cklib:<v>` |
| `pg17-pgrdf-pgck-nats-micro` (pg_base) | `pg17-pgrdf-pgck-nats-micro-v*` (**no `release-` prefix**) | `pg17-pgrdf-pgck-nats-micro-release.yml` | `ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-nats-micro:<v>` |
| `pg17-pgrdf-pgck-nats` | `pg17-pgrdf-pgck-nats-v*` | `pg17-pgrdf-pgck-nats-release.yml` | matching GHCR tag |
| `pg17-pgrdf-pgck` | `pg17-pgrdf-pgck-v*` | `pg17-pgrdf-pgck-release.yml` | matching GHCR tag |
| `pg17-pgrdf` | `pg17-pgrdf-v*` | `pg17-pgrdf-release.yml` | matching GHCR tag |
| `core-pg17-{min,micro,nats,nats-micro}` | `core-pg17-<variant>-v*` | `core-pg17-<variant>-release.yml` | matching GHCR tag |

Each tagged release goes through these gates inside CI before any artifact lands on GHCR:

1. **Verify job** — Go unit tests, generator round-trip (`scripts/generate.sh` re-emits the bundle outputs from `bundle.yaml`), generated-files-committed assertion (so hand-edits don't drift from the generator), pgRDF preload contract lint.
2. **Build** — multi-arch (`linux/amd64` + `linux/arm64`) image build.
3. **Smoke** — bundle-specific smoke script (`scripts/smoke-<bundle>.sh`) against the freshly built local image. Must exit 0.
4. **Python-free assertion** (ck-allinone only) — `docker exec` searches the prod image for `python*`, `uvicorn*`, `fastapi*` paths; any hit fails the build.
5. **Push to GHCR** — multi-arch manifest published.
6. **SLSA Build Provenance v1 attestation** — `actions/attest-build-provenance@v1` issues a signed attestation binding the digest to the workflow run + tag.

If any step fails, **no artifact reaches GHCR**. The version number is permanently spent (see Rule 9 below).

## Publishing a new bundle version — step-by-step

```bash
# 1. Edit the bundle's Dockerfile + bundle.yaml + smoke-script defaults if needed.
#    Make ALL changes for this cut on one commit so the version number is atomic.

# 2. Run the local smoke against the local build before tagging.
docker build --platform linux/arm64 \
    --build-arg BUNDLE_VERSION=v0.7.25 \
    -f bundles/bundle-ck-allinone/Dockerfile \
    -t ociger-ck-allinone:v0.7.25-local .
bash scripts/smoke-ck-allinone.sh ociger-ck-allinone:v0.7.25-local
# 10/10 green is the gate. If smoke fails, fix on the same commit, re-smoke.

# 3. Commit. Push main first.
git add bundles/bundle-ck-allinone/
git commit -m "feat(ck-allinone v0.7.25): …"
git push origin main

# 4. Tag with the next monotonic version. Push the tag.
git tag release-ck-allinone-v0.7.25
git push origin release-ck-allinone-v0.7.25

# 5. The workflow fires. Watch the run; if it fails, see "When CI fails" below.

# 6. On green, gh attestation verify confirms the digest:
gh attestation verify oci://ghcr.io/sporaxis-com/ociger-ck-allinone:v0.7.25 \
    --repo sporaxis-com/oci-germination
# Then update-latest-md.yml fires on workflow_run and advances LATEST.md.
```

## When CI fails — the hard rule

**The version number is permanently spent.** Do NOT delete the tag and re-push. Do NOT force-move the tag.

The fix is a new commit on `main` plus the **next** version number:

```bash
# CI failed at v0.7.25. Do NOT touch the v0.7.25 tag — it stays where it is.

# Author the fix.
git add scripts/smoke-ck-allinone.sh
git commit -m "fix(smoke): bump expected pgCK version default

The v0.7.25 CI run failed because the smoke script's default
PGCK_EXPECTED_VERSION hadn't been bumped to track the new pin.
This commit ships the fix; the next release is v0.7.14."
git push origin main

# Tag the fix with the NEXT version.
git tag release-ck-allinone-v0.7.14
git push origin release-ck-allinone-v0.7.14
```

Then record the failure in `CHANGELOG.md`:

```markdown
### v0.7.25 — 2026-06-XX — FAILED
- **Tried:** intent of this cut
- **Tested:** which CI step ran and which one failed
- **Cause:** root cause
- **Fix:** what changed
- **Verdict:** FAILED. CI run <id>. No artifact reached GHCR.
```

And the SHIPPED v0.7.14 entry that follows it.

This honors three things at once: the immutable container digest history on GHCR stays clean (no orphan digests under deleted tags), the CI run history reads unambiguously (`v0.7.25 FAILED` then `v0.7.14 SUCCESS`), and the audit trail in `CHANGELOG.md` makes the version-number gap self-explanatory.

## Smoke tests

Bundle-specific smoke scripts under `scripts/`:

- `smoke-pg17-pgrdf-pgck-nats-micro.sh` — pg_base 10-check (pg ready, pgRDF + pgCK + pgcrypto install + version match, parse_turtle pgatomic, NATS core PONG, NATS WSS port up).
- `smoke-ck-allinone.sh` — ck-allinone bundle-specific (auto-bootstrap, NATS, WSS round-trip, §B4 dispatch-bridge round-trip, Python-free, PID 1 = s6-svscan).
- `smoke-pg17-pgrdf-pgck-static-cklib.sh`, `smoke-pg17-pgrdf-pgck-nats.sh`, `smoke-pg17-pgrdf-pgck.sh`, `smoke-pg17-pgrdf.sh` — sibling bundles in the matrix.

Each smoke takes the image tag as `$1` and exits non-zero on any failure. Override expected component versions via env (`PGRDF_EXPECTED_VERSION=…`, `PGCK_EXPECTED_VERSION=…`) when you need to.

## Workstation pushes are prohibited at every tier

`docker push`, `docker buildx --push`, `gh release create` against this org's GHCR namespaces — none of them are allowed from a workstation. The single sanctioned publish path is the tag-pushes-trigger-Actions loop above. The reasoning is the SLSA attestation gate: a workstation push cannot produce a GitHub-issued OIDC attestation; the next `update-latest-md.yml` run would reject it and `LATEST.md` would refuse to advance.

For pre-tag local validation, `docker build` + local smoke is fine. `docker buildx build --load` is fine. Anything with `--push` is not.

## Verifying a published image

```bash
# Manifest (multi-arch).
docker manifest inspect ghcr.io/sporaxis-com/ociger-ck-allinone:v0.7.25

# Attestation (the gate).
gh attestation verify oci://ghcr.io/sporaxis-com/ociger-ck-allinone:v0.7.25 \
    --repo sporaxis-com/oci-germination

# Smoke against the published image.
bash scripts/smoke-ck-allinone.sh ghcr.io/sporaxis-com/ociger-ck-allinone:v0.7.25
```

## Troubleshooting

**Workflow didn't trigger:**
- Check the tag exactly matches the workflow's prefix (notably: pg_base bundles do NOT use `release-`).
- Confirm the tag was pushed: `git ls-remote --tags origin | grep <tag>`.
- Check the Actions tab for an event but a skipped run (sometimes path filters block).

**Build failed:**
- Check the failed-step log: `gh run view <id> --log-failed`.
- Do **not** re-tag the failed version. Fix on a new commit and bump to the next version per the rule above.
- Update `CHANGELOG.md` with the FAILED entry.

**Image not appearing in GHCR after a green run:**
- Check that the "Push to GHCR" step actually ran (some matrix bundles have separate push jobs).
- Verify the package visibility on GHCR (should be public; `gh api /orgs/sporaxis-com/packages/container/<name>` shows the current state).

## Cross-references

- [PROVENANCE.md](./PROVENANCE.md) — full release policy (Rules 1–10, including the monotonic-version rule).
- [SEMANTIC-VERSIONING.md](./SEMANTIC-VERSIONING.md) — tag prefix table + version-number scheme.
- [CHANGELOG.md](./CHANGELOG.md) — every release attempt, with verdict.
- [LATEST.md](./LATEST.md) — auto-rendered head of each bundle, attestation-gated.
