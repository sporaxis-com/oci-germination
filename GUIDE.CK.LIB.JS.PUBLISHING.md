# Publishing CK.Lib.Js Multi-Variant Images — Fix for Broken Manifest

**Problem:** CK.Lib.Js v1.2.0 published with corrupted multi-platform manifest:
```
ghcr.io/conceptkernel/bundle-ck-lib-js:dev@sha256:b527c7055fbc7... (linux/amd64)
ghcr.io/conceptkernel/bundle-ck-lib-js:dev@sha256:74e2a7202b8d... (linux/arm64)
ghcr.io/conceptkernel/bundle-ck-lib-js:dev@sha256:6ec0cdea... (unknown/unknown)
↑ BROKEN: Different digests per platform + "unknown" indicates manifest corruption
```

**Root Cause:** Images were built separately per-platform instead of in a single multi-arch invocation. Each `docker build` produced a different image, then the manifest merge failed.

---

## 1. Correct Publishing Workflow

### 1.1 Naming Fix

**Change in `bundles/bundle-ck-lib-js/bundle.yaml`:**

```yaml
# WRONG (current):
image:
  registry: ghcr.io/conceptkernel/bundle-ck-lib-js  ← "bundle-" prefix in image name

# CORRECT:
image:
  registry: ghcr.io/conceptkernel/ck-lib-js         ← no "bundle-" prefix
```

The bundle specification lives in `bundles/bundle-ck-lib-js/`, but the published image should be `ck-lib-js`.

### 1.2 Single buildx Invocation (Multi-Platform)

Rebuild and publish as a single command:

```bash
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --target static \
  --tag ghcr.io/conceptkernel/ck-lib-js:1.2.0 \
  --tag ghcr.io/conceptkernel/ck-lib-js:latest \
  --push \
  bundles/bundle-ck-lib-js/
```

Then separately:

```bash
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --target dev \
  --tag ghcr.io/conceptkernel/ck-lib-js:1.2.0-dev \
  --push \
  bundles/bundle-ck-lib-js/
```

**Critical:** Both commands must:
1. Include `--platform linux/amd64,linux/arm64` (both platforms in one invocation).
2. Use `--push` to publish directly (does not create a local image).
3. Be run from the same Dockerfile (no per-platform builds).

### 1.3 Verify the Manifest

After pushing, verify that the manifest is correct:

```bash
docker buildx imagetools inspect ghcr.io/conceptkernel/ck-lib-js:1.2.0
```

Expected output:
```
Name:      ghcr.io/conceptkernel/ck-lib-js:1.2.0
MediaType: application/vnd.docker.distribution.manifest.list.v2+json
Digest:    sha256:abc123...

Manifests:
  Name:      ghcr.io/conceptkernel/ck-lib-js:1.2.0
  MediaType: application/vnd.docker.distribution.manifest.v2+json
  Platform:  linux/amd64
  Digest:    sha256:def456...

  Name:      ghcr.io/conceptkernel/ck-lib-js:1.2.0
  MediaType: application/vnd.docker.distribution.manifest.v2+json
  Platform:  linux/arm64
  Digest:    sha256:def456...  ← MUST be the SAME as amd64
```

If the per-platform digests are **different**, the manifest is broken. Rebuild.

---

## 2. GitHub Actions Workflow

Update `.github/workflows/bundle-ck-lib-js-release.yml`:

```yaml
name: Publish CK.Lib.Js Bundle

on:
  push:
    tags:
      - 'bundle-ck-lib-js-v*'

jobs:
  publish:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - uses: docker/setup-buildx-action@v3
      
      - uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
      
      # Extract version from tag (e.g., bundle-ck-lib-js-v1.2.0 → 1.2.0)
      - name: Parse version
        id: version
        run: |
          TAG="${{ github.ref_name }}"
          VERSION="${TAG#bundle-ck-lib-js-v}"
          echo "version=${VERSION}" >> $GITHUB_OUTPUT
      
      # Build and publish static image
      - name: Build and push static image
        uses: docker/build-push-action@v5
        with:
          context: bundles/bundle-ck-lib-js
          platforms: linux/amd64,linux/arm64
          target: static
          push: true
          tags: |
            ghcr.io/conceptkernel/ck-lib-js:${{ steps.version.outputs.version }}
            ghcr.io/conceptkernel/ck-lib-js:latest
      
      # Build and publish dev image
      - name: Build and push dev image
        uses: docker/build-push-action@v5
        with:
          context: bundles/bundle-ck-lib-js
          platforms: linux/amd64,linux/arm64
          target: dev
          push: true
          tags: |
            ghcr.io/conceptkernel/ck-lib-js:${{ steps.version.outputs.version }}-dev
      
      # Publish release with digests
      - name: Publish release
        run: |
          docker buildx imagetools inspect ghcr.io/conceptkernel/ck-lib-js:${{ steps.version.outputs.version }} \
            > /tmp/static-manifest.txt
          docker buildx imagetools inspect ghcr.io/conceptkernel/ck-lib-js:${{ steps.version.outputs.version }}-dev \
            > /tmp/dev-manifest.txt
          
          gh release create bundle-ck-lib-js-v${{ steps.version.outputs.version }} \
            --title "CK.Lib.Js v${{ steps.version.outputs.version }}" \
            --notes "Static: ghcr.io/conceptkernel/ck-lib-js:${{ steps.version.outputs.version }}
          Dev: ghcr.io/conceptkernel/ck-lib-js:${{ steps.version.outputs.version }}-dev
          $(cat /tmp/static-manifest.txt)
          $(cat /tmp/dev-manifest.txt)"
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

---

## 3. Fix Checklist

- [ ] Update `bundles/bundle-ck-lib-js/bundle.yaml`: Change `image.registry` from `ghcr.io/conceptkernel/bundle-ck-lib-js` to `ghcr.io/conceptkernel/ck-lib-js`
- [ ] Verify Dockerfile has proper `--target static` and `--target dev` stages
- [ ] Update `.github/workflows/bundle-ck-lib-js-release.yml` to use `docker/build-push-action@v5` with `platforms: linux/amd64,linux/arm64`
- [ ] Delete existing broken images from GHCR (optional; newer tags will supersede)
- [ ] Tag new release: `git tag bundle-ck-lib-js-v1.2.1` (or bump to v1.2.0 if not yet released)
- [ ] Push tag: `git push origin bundle-ck-lib-js-v1.2.1`
- [ ] Wait for workflow to complete
- [ ] Verify manifests: `docker buildx imagetools inspect ghcr.io/conceptkernel/ck-lib-js:1.2.1`
- [ ] Test pull: `docker pull ghcr.io/conceptkernel/ck-lib-js:1.2.1` from both amd64 and arm64
- [ ] Set package visibility to public in GitHub package settings

---

## 4. Why This Matters

Per CKP ontology:
- `ckp:WebServing` — static HTML/JS/CSS assets (the `ck-lib-js:1.2.0` image)
- `ckp:APIServing` — HTTP API endpoint (the `ck-lib-js:1.2.0-dev` image, if dev server exposes an API)

Different semantic types deserve separate images. Using tag suffixes (`:1.2.0` vs `:1.2.0-dev`) makes this clear without duplicating the bundle specification or breaking the manifest.

---

**Reference:** [SPEC.OCI.BUNDLE.v0.1 § 6.3 Multi-Platform Manifests & Per-Architecture Digests](SPEC.OCI.BUNDLE.v0.1.md#63-multi-platform-manifests--per-architecture-digests)
