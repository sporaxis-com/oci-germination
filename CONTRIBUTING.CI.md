# CI/CD Pipeline — OCI Bundle Builds

## Automated Builds via GitHub Actions

OCI bundles are built and published to GHCR automatically when version tags are pushed.

### Tag Convention

Tags follow the pattern: `release-<bundle-name>-<version>`

| Bundle | Tag Example | Resulting Image |
|--------|-------------|-----------------|
| `pg17-pgrdf-pgck-web-cklib` | `release-pg17-pgrdf-pgck-web-cklib-1.0.0` | `ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-web-cklib:1.0.0` |
| `ck-allinone` | `release-ck-allinone-v3.8-rc2` | `ghcr.io/sporaxis-com/ociger-ck-allinone:v3.8-rc2` |

### Publishing a New Bundle Version

1. **Test locally** (optional, for pre-release validation):
   ```bash
   docker build -t ociger-pg17-pgrdf-pgck-web-cklib:1.0.0 \
     bundles/bundle-pg17-pgrdf-pgck-web-cklib/
   ```

2. **Create and push a release tag:**
   ```bash
   git tag -a release-pg17-pgrdf-pgck-web-cklib-1.0.0 \
     -m "Release pg17-pgrdf-pgck-web-cklib v1.0.0"
   git push origin release-pg17-pgrdf-pgck-web-cklib-1.0.0
   ```

3. **GitHub Actions will:**
   - Automatically detect the tag
   - Build multi-platform images (linux/amd64, linux/arm64)
   - Push to GHCR with the specified version
   - Report completion in Actions tab

### Verifying Published Images

After the workflow completes, verify the image is available:

```bash
# Inspect manifest
docker manifest inspect ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-web-cklib:1.0.0

# Pull and test
docker pull ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-web-cklib:1.0.0
bash scripts/smoke-pg17-pgrdf-pgck-web-cklib.sh \
  ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-web-cklib:1.0.0
```

### Smoke Tests

All bundles have comprehensive smoke test suites. Before publishing, run locally:

```bash
# Test pg17-pgrdf-pgck-web-cklib
bash scripts/smoke-pg17-pgrdf-pgck-web-cklib.sh ociger-pg17-pgrdf-pgck-web-cklib:1.0.0

# Test ck-allinone
bash scripts/smoke-ck-allinone.sh ociger-ck-allinone:v3.8-rc2
```

### Workflow File

The workflow is defined in `.github/workflows/build-bundles.yml` and:

- Triggers on `release-*` tag pushes only
- Uses `docker/setup-buildx-action` for multi-platform builds
- Authenticates with GHCR using `GITHUB_TOKEN` (no secrets needed)
- Caches build layers for faster rebuilds
- Publishes both amd64 and arm64 manifests to a single image:tag

### Troubleshooting

**Workflow didn't trigger:**
- Check that tag matches pattern: `release-<bundle-name>-<version>`
- Verify tag was pushed (not just created locally): `git push origin <tag>`
- Check Actions tab for any errors

**Build failed:**
- Review workflow logs in the Actions tab
- Run smoke tests locally to debug Dockerfile issues
- Push a fix commit and retag: `git tag -f <tag> && git push origin -f <tag>`

**Image not appearing in GHCR:**
- Workflow may still be running (check Actions)
- Verify authentication: `gh auth status`
- Check package visibility in GHCR settings (should be public or org-accessible)

---

## Local Development Workflow

For iterating on bundle changes without tagging:

```bash
# 1. Make Dockerfile/bundle.yaml changes
# 2. Build locally to test
docker build -t ociger-pg17-pgrdf-pgck-web-cklib:test \
  bundles/bundle-pg17-pgrdf-pgck-web-cklib/

# 3. Run smoke tests
bash scripts/smoke-pg17-pgrdf-pgck-web-cklib.sh ociger-pg17-pgrdf-pgck-web-cklib:test

# 4. When satisfied, commit changes
git add bundles/bundle-pg17-pgrdf-pgck-web-cklib/
git commit -m "refine: update bundle..."

# 5. Tag for release
git tag -a release-pg17-pgrdf-pgck-web-cklib-<version> -m "Release message"
git push origin <branch> release-pg17-pgrdf-pgck-web-cklib-<version>
```

Multi-platform local builds (with buildx) can be tested before tagging:

```bash
# Authenticate with GHCR
gh auth token | docker login ghcr.io -u $(gh api user --jq .login) --password-stdin

# Multi-platform build (local, no push)
docker buildx build --platform linux/amd64,linux/arm64 \
  --load bundles/bundle-pg17-pgrdf-pgck-web-cklib/
  
# Or, build and push to dev tag
docker buildx build --platform linux/amd64,linux/arm64 \
  --tag ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-web-cklib:dev \
  --push bundles/bundle-pg17-pgrdf-pgck-web-cklib/
```

---

## Component Versioning

Each bundle tracks independent component versions in `bundles/<bundle>/bundle.yaml`:

- **PostgreSQL** — inherited from base image (17.0)
- **pgRDF** — from extension artifact (0.5.1)
- **pgCK** — from extension artifact (0.1.2)
- **pgckweb** — FastAPI server version (0.1.0)
- **cklib** — from OCI layer (1.2.0)
- **NATS** — from embedded service (2.14.1, all-in-one only)

Update `bundle.yaml` to reflect changes; the bundle version tag is independent.

---

## Release Notes

Include in commit messages and git tags:

- **What changed:** component updates, bug fixes, new features
- **Why it matters:** security, compatibility, performance improvement
- **Breaking changes:** if any API/schema changes
- **Smoke test results:** summary of integration test outcomes

Example:

```
Release pg17-pgrdf-pgck-web-cklib v1.0.0

- Add pgckweb FastAPI server with /cklib/ mount point
- Import cklib (CK.Lib.Js 1.2.0) as OCI layer source
- Bundle specification (bundle.yaml) with component attribution
- Comprehensive 10-point smoke test suite

Smoke test: All 10 integration points pass (PostgreSQL, extensions, FastAPI, cklib serving)

Compatibility: Built on ociger-pg17-pgrdf-pgck:v0.1.1 base
```
