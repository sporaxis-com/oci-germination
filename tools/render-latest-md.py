#!/usr/bin/env python3
"""Render LATEST.md from current GHCR state, gated on SLSA Build Provenance v1.

Per PROVENANCE.md Rules 2 + 3, only attestation-verified digests are
advertised. Bundles whose latest tag has no valid attestation are emitted
with a placeholder block saying "no attested release yet". This is the
ONLY allowed writer of LATEST.md.

Run from the repo root with:
  GH_TOKEN=<token> OWNER=sporaxis-com REPO=sporaxis-com/oci-germination \\
    python3 tools/render-latest-md.py > LATEST.md.new

Environment:
  GH_TOKEN — GitHub API token (workflow's GITHUB_TOKEN works)
  OWNER    — org/user namespace under which the packages live (sporaxis-com)
  REPO     — full owner/repo for attestation verification
"""

from __future__ import annotations

import json
import os
import re
import subprocess
import sys
from datetime import datetime, timezone
from typing import Iterable

# The composition engine lives alongside this file (tools/). It reads each
# bundle's bundle.yaml for the expected layer map and, when a digest-matched
# composition.confirmed.json is present, the test-confirmed real versions.
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import version_composition as vc  # noqa: E402

# ----------------------------------------------------------------------
# Bundle registry — single source of truth for what we publish + display.
# ----------------------------------------------------------------------

# Each entry: (package_name, heading, short_description, bundle_dir)
# Only the ACTIVE pg18/trixie wave is advertised. The pg17 + core-pg17 matrix is
# RETIRED/frozen (pgRDF/pgCK are pg18-only from 0.6.20/0.4.x — og#9); those images
# stay published on GHCR but are no longer tracked here. See versions.yaml `frozen:`.
BUNDLES = [
    (
        "ociger-ck-allinone",
        "CKP v3.12 all-in-one (the default)",
        "PostgreSQL 18 (trixie, glibc 2.41) + pgRDF + pgCK (`-nats` build) + pgcrypto + NATS core (4222) + NATS WSS (9222) + CK.Lib.Js at `/cklib/`. s6-overlay supervises; postgres runs as uid 999, and NATS + busybox httpd (`/app` on :8000) drop to non-root users. Scratch base. No Python, no postgres client — bootstrap runs through `postgres --single`. pgCK's `-nats` build owns the in-extension inbound dispatch, the `$SYS.REQ.USER.AUTH` auth-callout responder, and the `ckp.outbox` drain in-process (no Go relay). `ociger-ck-identity` boot-provisions the OIDC-gated auth-callout; the account seed is never baked into the image.",
        "bundles/bundle-ck-allinone",
    ),
    (
        "ociger-pg18-pgrdf-pgck-nats-micro",
        "pg18 base — pgRDF + pgCK-nats + NATS (scratch)",
        "PostgreSQL 18 (trixie, glibc 2.41) + pgRDF + pgCK (`-nats` build) + NATS (4222) + NATS WSS (9222). Scratch base, both arches built consistently on trixie. The canonical base `ck-allinone` builds `FROM`.",
        "bundles/bundle-pg18-pgrdf-pgck-nats-micro",
    ),
]


# ----------------------------------------------------------------------
# GitHub API helpers (via gh CLI for auth simplicity)
# ----------------------------------------------------------------------


def gh(args: list[str]) -> str:
    """Run `gh` and return stdout. Raises on non-zero exit."""
    res = subprocess.run(
        ["gh"] + args, capture_output=True, text=True, check=False
    )
    if res.returncode != 0:
        raise RuntimeError(f"gh {' '.join(args)} failed: {res.stderr}")
    return res.stdout


_VERSION_TAG_RE = re.compile(r"^(?:[a-z0-9-]+-)?v?[0-9]+\.[0-9]+\.[0-9]+$")
"""Matches real version tags like 'v0.6.3', 'v0.1.2', 'core-pg17-v0.1.3'.

Excludes:
- 'latest' aliases
- attestation referrer tags like 'sha256-a86a165a...'
- arch-suffixed aliases like 'v0.6.3-amd64' (we publish multi-arch manifests, not per-arch tags)
"""


def get_latest_tagged_version(owner: str, package: str) -> dict | None:
    """Return the most recent VERSION-tagged GHCR version.

    Skips:
    - `latest` / `latest-amd64` / `latest-arm64` aliases
    - `sha256-…` attestation referrer tags (these are OCI referrer artifacts that
      GHCR exposes as separate versions; tagging by the digest of their subject)
    - any other non-semver tag

    Picks the most-recently-created version whose tag set contains at least one
    semver-shaped tag.
    """
    raw = gh(
        [
            "api",
            f"/orgs/{owner}/packages/container/{package}/versions",
            "--jq",
            ".",
        ]
    )
    versions = json.loads(raw)
    for v in versions:
        tags = v.get("metadata", {}).get("container", {}).get("tags", [])
        version_tags = [t for t in tags if _VERSION_TAG_RE.match(t)]
        if version_tags:
            v["_chosen_tag"] = sorted(version_tags)[-1]
            v["_all_tags"] = tags
            return v
    return None


def get_platform_digests(image_ref: str) -> dict[str, str]:
    """Use `docker manifest inspect` to get amd64/arm64 platform digests.

    Falls back to empty strings if docker isn't available or the manifest
    can't be fetched (e.g. permission). The renderer continues — the
    Index digest is still authoritative.
    """
    try:
        out = subprocess.run(
            ["docker", "manifest", "inspect", image_ref],
            capture_output=True,
            text=True,
            check=False,
        )
        if out.returncode != 0:
            return {}
        idx = json.loads(out.stdout)
        digests = {}
        for m in idx.get("manifests", []):
            plat = m.get("platform", {})
            if plat.get("os") == "linux":
                arch = plat.get("architecture")
                if arch in ("amd64", "arm64"):
                    digests[arch] = m.get("digest", "")
        return digests
    except Exception:
        return {}


def verify_attestation(image_ref: str, repo: str) -> bool:
    """Run `gh attestation verify` against the manifest tag. Return True iff verified."""
    res = subprocess.run(
        [
            "gh",
            "attestation",
            "verify",
            f"oci://{image_ref}",
            "--repo",
            repo,
        ],
        capture_output=True,
        text=True,
        check=False,
    )
    return res.returncode == 0


def get_image_labels(image_ref: str) -> dict[str, str]:
    """Use `docker buildx imagetools inspect` to read manifest labels without pulling the image.

    Every image carries `ck.bundle.role` and `ck.bundle.never-prod` labels;
    the renderer surfaces these into the LATEST.md block. Returns an empty
    dict on any error — older images that predate the label contract render
    those fields as "—".
    """
    try:
        out = subprocess.run(
            ["docker", "buildx", "imagetools", "inspect", image_ref, "--format", "{{json .Image}}"],
            capture_output=True,
            text=True,
            check=False,
        )
        if out.returncode != 0:
            return {}
        payload = json.loads(out.stdout)
        # multi-arch payload has per-arch entries; pick any one's labels.
        if isinstance(payload, dict):
            for entry in payload.values():
                cfg = (entry or {}).get("config", {})
                lbl = cfg.get("Labels") or {}
                if lbl:
                    return lbl
            # single-arch fallback
            cfg = payload.get("config", {})
            return cfg.get("Labels") or {}
        return {}
    except Exception:
        return {}


# ----------------------------------------------------------------------
# Markdown rendering
# ----------------------------------------------------------------------


def fmt_ts(iso: str) -> str:
    """2026-05-28T15:59:17Z → '2026-05-28 15:59:17' (UTC)."""
    dt = datetime.fromisoformat(iso.replace("Z", "+00:00")).astimezone(timezone.utc)
    return dt.strftime("%Y-%m-%d %H:%M:%S")


def _maybe_reprobe(bundle_dir: str, conf_path: str, index_digest: str, image_ref: str) -> dict | None:
    """Opt-in, best-effort: re-probe the live image and re-stamp the JSON to the
    advertised digest. Gated on env PROBE_COMPOSITION and on the bundle already
    opting in (a composition.confirmed.json present). Returns the fresh record on
    success, else None (caller falls back to the expected map). Never raises."""
    if not os.environ.get("PROBE_COMPOSITION") or not os.path.exists(conf_path):
        return None
    try:
        pdata = vc.probe(image_ref)
        comp = vc.compose(bundle_dir, pdata)
        comp["index_digest"] = pdata.get("_index_digest", "") or index_digest
        comp["image"] = image_ref
        comp["confirmed_at"] = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
        with open(conf_path, "w") as f:
            json.dump(comp, f, indent=2, ensure_ascii=False)
            f.write("\n")
        return comp if comp.get("index_digest") == index_digest else None
    except Exception:
        return None


def render_composition_section(bundle_dir: str, index_digest: str, image_ref: str) -> str:
    """Composition table for a bundle: EXPECTED (bundle.yaml) + test-CONFIRMED.

    Uses bundles/<dir>/composition.confirmed.json ONLY when its recorded
    index_digest matches the digest we're advertising — so a stale confirmation
    from a prior cut never decorates a new digest. Without a digest-match it
    falls back to the expected layer map from bundle.yaml. See PROVENANCE.md.
    """
    if not os.path.exists(os.path.join(bundle_dir, "bundle.yaml")):
        return ""  # retired/placeholder bundle — no layer map to compose
    conf_path = os.path.join(bundle_dir, "composition.confirmed.json")
    confirmed = None
    if os.path.exists(conf_path):
        try:
            c = json.load(open(conf_path))
            if c.get("index_digest") == index_digest:
                confirmed = c
        except Exception:
            confirmed = None
    if confirmed is None:
        confirmed = _maybe_reprobe(bundle_dir, conf_path, index_digest, image_ref)
    if confirmed:
        table = vc.render_md(confirmed)
        stamp = (f"**test-confirmed** against this digest at "
                 f"`{confirmed.get('confirmed_at', '?')}` — every probed native version was "
                 f"read back from the running image; the rest are gated by the bundle's "
                 f"gate-before-push smoke.")
    else:
        table = vc.render_md(vc.compose(bundle_dir, None))
        stamp = ("expected layer map (bundle.yaml); live confirmation re-attaches on the "
                 "next gated build for this digest.")
    return f"**Version composition** — {stamp}\n\n{table}\n"


def render_bundle_section(
    pkg: str,
    heading: str,
    desc: str,
    bundle_dir: str,
    version_data: dict | None,
    owner: str,
    repo: str,
) -> str:
    """Return the markdown block for one bundle (attested or placeholder)."""
    lines: list[str] = []
    if version_data is None:
        # No tagged release at all.
        lines.append(f"## {pkg} — *no published release yet*")
        lines.append("")
        lines.append(desc)
        lines.append("")
        lines.append(f"|                    |                                                                          |")
        lines.append(f"|--------------------|--------------------------------------------------------------------------|")
        lines.append(f"| Source bundle      | [`{bundle_dir}/`](./{bundle_dir}/)                                          |")
        lines.append(f"| Repo packages view | https://github.com/orgs/{owner}/packages/container/package/{pkg} |")
        lines.append("")
        return "\n".join(lines)

    tag = version_data["_chosen_tag"]
    all_tags = version_data["_all_tags"]
    also = ", ".join(f"`{t}`" for t in all_tags if t != tag) or "—"
    index_digest = version_data["name"]  # GHCR's `name` field is sha256:...
    created_iso = version_data["created_at"]
    image_ref = f"ghcr.io/{owner}/{pkg}:{tag}"

    attested = verify_attestation(image_ref, repo)
    if not attested:
        lines.append(f"## {pkg} — *no attested release yet*")
        lines.append("")
        lines.append(desc)
        lines.append("")
        lines.append("> Per [`PROVENANCE.md`](./PROVENANCE.md) Rule 2, `LATEST.md` does not advertise unattested releases. The most recent GHCR tag for this bundle does not carry a SLSA Build Provenance v1 attestation (yet). This block will populate when the next release crosses the attestation gate.")
        lines.append("")
        lines.append(f"|                    |                                                                          |")
        lines.append(f"|--------------------|--------------------------------------------------------------------------|")
        lines.append(f"| Source bundle      | [`{bundle_dir}/`](./{bundle_dir}/)                                          |")
        lines.append(f"| Repo packages view | https://github.com/orgs/{owner}/packages/container/package/{pkg} |")
        lines.append("")
        return "\n".join(lines)

    # Attested — render full block
    platforms = get_platform_digests(image_ref)
    amd64 = platforms.get("amd64", "_unknown_")
    arm64 = platforms.get("arm64", "_unknown_")
    created_fmt = fmt_ts(created_iso)

    # Read role + never-prod from manifest labels (set by each bundle's Dockerfile).
    labels = get_image_labels(image_ref)
    role = labels.get("ck.bundle.role", "—")
    never_prod = labels.get("ck.bundle.never-prod", "—")
    prod_use = "Not for production" if never_prod == "true" else (
        "Production-ready" if never_prod == "false" else "—"
    )

    lines.append(f"## {pkg} — `{tag}`")
    lines.append("")
    lines.append(desc)
    lines.append("")
    lines.append("| arch  | Platform digest                                                            | Created (UTC)       |")
    lines.append("|-------|----------------------------------------------------------------------------|---------------------|")
    lines.append(f"| amd64 | `{amd64}`  | {created_fmt} |")
    lines.append(f"| arm64 | `{arm64}`  | {created_fmt} |")
    lines.append("")
    lines.append("|                    |                                                                          |")
    lines.append("|--------------------|--------------------------------------------------------------------------|")
    lines.append(f"| Pull URI           | `{image_ref}`                                                            |")
    lines.append(f"| Also tagged        | {also}                                                                  |")
    lines.append(f"| Index digest       | `{index_digest}`                                                         |")
    lines.append(f"| Role               | `{role}`                                                                |")
    lines.append(f"| Production use     | {prod_use}                                                              |")
    lines.append(f"| Attestation        | SLSA Build Provenance v1 ✓ verified via `gh attestation verify`           |")
    lines.append(f"| Source bundle      | [`{bundle_dir}/`](./{bundle_dir}/)                                          |")
    lines.append(f"| Repo packages view | https://github.com/orgs/{owner}/packages/container/package/{pkg} |")
    lines.append("")
    lines.append(render_composition_section(bundle_dir, index_digest, image_ref))
    return "\n".join(lines)


# ----------------------------------------------------------------------
# Entrypoint
# ----------------------------------------------------------------------


def main() -> int:
    owner = os.environ.get("OWNER", "sporaxis-com")
    repo = os.environ.get("REPO", "sporaxis-com/oci-germination")

    header = f"""<!--
  This file is auto-generated by .github/workflows/update-latest-md.yml after a
  successful release workflow run AND after SLSA Build Provenance v1
  attestations verify against the GHCR digests it advertises. Do NOT edit by
  hand — manual edits are reverted on the next workflow run. See
  PROVENANCE.md for the policy.
-->

# oci-germination — latest published artifacts

The active **CKP v3.12** wave: the `ociger-ck-allinone` all-in-one and the `ociger-pg18-pgrdf-pgck-nats-micro` base it builds `FROM`. The image ships both the v3.11 and v3.12 FINAL ontology trees; `ckp.boot()` grounds on the v3.12 root, while `init.sql`'s two module Adoptions still cite the v3.11 `wave`/`lexicon` modules (v3.12 ships no `lexicon`). Both PostgreSQL 18 / trixie, multi-arch (`linux/amd64` + `linux/arm64`), anonymous public pull. This file tracks the attested head of each. The retired pg17 + core-pg17 matrix (frozen at the pg18 move — pgRDF/pgCK are pg18-only) stays published on GHCR but is no longer tracked here; see the [Repo packages view](https://github.com/orgs/{owner}/packages?repo_name=oci-germination).

> **Policy.** Per [`PROVENANCE.md`](./PROVENANCE.md) Rule 2, only bundle releases that pass `gh attestation verify` for their published digest are advertised here.

"""

    sections = [header]
    for pkg, heading, desc, bundle_dir in BUNDLES:
        version_data = get_latest_tagged_version(owner, pkg)
        section = render_bundle_section(
            pkg, heading, desc, bundle_dir, version_data, owner, repo
        )
        sections.append(section)

    footer = f"""## Pin policy

- `latest` tracks the **most recent attested tag** on each bundle's production image name.
- Tagged versions are immutable on GHCR.
- All bundles are multi-arch manifest lists — pulling by the multi-arch `latest` or `vX.Y.Z` tag resolves to the right platform automatically.
- From the first attested release forward, every entry on this page ships with a verifiable **SLSA Build Provenance v1** attestation tying it to a specific GitHub Actions workflow run on this repo.
- See [`SEMANTIC-VERSIONING.md`](./SEMANTIC-VERSIONING.md) and [`PROVENANCE.md`](./PROVENANCE.md) for the full policy.
"""
    sections.append(footer)

    sys.stdout.write("\n".join(sections))
    return 0


if __name__ == "__main__":
    sys.exit(main())
