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

# ----------------------------------------------------------------------
# Bundle registry — single source of truth for what we publish + display.
# ----------------------------------------------------------------------

# Each entry: (package_name, heading, short_description, bundle_dir)
BUNDLES = [
    (
        "ociger-ck-allinone",
        "CKP Development Default",
        "PostgreSQL 17 + pgRDF + pgCK + NATS (4222) + NATS WSS (9222) + pgckweb (FastAPI) + CK.Lib.Js mounted at `/cklib/`. Supervisor-orchestrated, scratch base. ⚠️ FastAPI process latent gap — Postgres/pgRDF/pgCK/NATS work; FastAPI dead. Use `static-cklib` (below) for a working web layer.",
        "bundles/bundle-ck-allinone",
    ),
    (
        "ociger-pg17-pgrdf-pgck-static-cklib",
        "CKP v3.8-aligned web bundle",
        "PostgreSQL 17 + pgRDF + pgCK + NATS + NATS WSS + Go static-server + CK.Lib.Js at `/cklib/`. No Python, no FastAPI. Browser ↔ kernel via NATS WSS; HTTP serves only static assets.",
        "bundles/bundle-pg17-pgrdf-pgck-static-cklib",
    ),
    (
        "ociger-pg17-pgrdf-pgck-web-cklib",
        "Standard web bundle (no NATS)",
        "PostgreSQL 17 + pgRDF + pgCK + pgckweb (FastAPI) + CK.Lib.Js at `/cklib/`. Distroless base. ⚠️ Same FastAPI latent gap as ck-allinone. Use `static-cklib` for a working web layer.",
        "bundles/bundle-pg17-pgrdf-pgck-web-cklib",
    ),
    (
        "ociger-pg17-pgrdf-pgck-nats-micro",
        "Triple bundle + NATS, scratch base",
        "PostgreSQL 17 + pgRDF + pgCK + NATS (4222) + NATS WSS (9222). Scratch base. Canonical base for ck-allinone and static-cklib.",
        "bundles/bundle-pg17-pgrdf-pgck-nats-micro",
    ),
    (
        "ociger-pg17-pgrdf-pgck-nats",
        "Triple bundle + NATS, distroless base",
        "PostgreSQL 17 + pgRDF + pgCK + NATS + NATS WSS. Distroless base (shell + libc available).",
        "bundles/bundle-pg17-pgrdf-pgck-nats",
    ),
    (
        "ociger-pg17-pgrdf-pgck",
        "Triple bundle (no NATS)",
        "PostgreSQL 17 + pgRDF + pgCK preloaded by default (`shared_preload_libraries=pgrdf,pgck`). No NATS. Distroless base.",
        "bundles/bundle-pg17-pgrdf-pgck",
    ),
    (
        "ociger-pg17-pgrdf",
        "pgRDF only",
        "PostgreSQL 17 + pgRDF. No pgCK. Distroless base.",
        "bundles/bundle-pg17-pgrdf",
    ),
    (
        "ociger-core-pg17-nats-micro",
        "PostgreSQL 17 + NATS (scratch)",
        "PostgreSQL 17 + NATS + NATS WSS. No extensions. Scratch base.",
        "bundles/core-pg17-nats-micro",
    ),
    (
        "ociger-core-pg17-nats",
        "PostgreSQL 17 + NATS (distroless)",
        "PostgreSQL 17 + NATS + NATS WSS. No extensions. Distroless base.",
        "bundles/core-pg17-nats",
    ),
    (
        "ociger-core-pg17-micro",
        "PostgreSQL 17 only (scratch)",
        "PostgreSQL 17 only. No extensions, no NATS. Scratch base. Smallest postgres bundle.",
        "bundles/core-pg17-micro",
    ),
    (
        "ociger-core-pg17-min",
        "PostgreSQL 17 only (distroless)",
        "PostgreSQL 17 only. Distroless base (shell + libc available).",
        "bundles/core-pg17",
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


# ----------------------------------------------------------------------
# Markdown rendering
# ----------------------------------------------------------------------


def fmt_ts(iso: str) -> str:
    """2026-05-28T15:59:17Z → '2026-05-28 15:59:17' (UTC)."""
    dt = datetime.fromisoformat(iso.replace("Z", "+00:00")).astimezone(timezone.utc)
    return dt.strftime("%Y-%m-%d %H:%M:%S")


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
    lines.append(f"| Attestation        | SLSA Build Provenance v1 ✓ verified via `gh attestation verify`           |")
    lines.append(f"| Source bundle      | [`{bundle_dir}/`](./{bundle_dir}/)                                          |")
    lines.append(f"| Repo packages view | https://github.com/orgs/{owner}/packages/container/package/{pkg} |")
    lines.append("")
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

Eleven OCI bundles ship from this repo: four `core-pg17-*` infrastructure images, four `pg17-pgrdf-*` extension bundles, and three web-bearing variants for CKP v3.8 development. All multi-arch (`linux/amd64` + `linux/arm64`), anonymous public pull. This file tracks the head of each. See [Repo packages view](https://github.com/orgs/{owner}/packages?repo_name=oci-germination) for the full version matrix.

> **Policy.** Per [`PROVENANCE.md`](./PROVENANCE.md) Rule 2, only bundle releases that pass `gh attestation verify` for their published digest are advertised here. Bundles still on pre-attestation tags display *"no attested release yet"* until their next release crosses the bootstrap gate.

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
- See [`SEMANTIC-VERSIONING.md`](./SEMANTIC-VERSIONING.md), [`SPEC.OCIGERMI.TRACKS.DEVEL.v1.0.md`](./SPEC.OCIGERMI.TRACKS.DEVEL.v1.0.md), and [`PROVENANCE.md`](./PROVENANCE.md) for the full policy.
"""
    sections.append(footer)

    sys.stdout.write("\n".join(sections))
    return 0


if __name__ == "__main__":
    sys.exit(main())
