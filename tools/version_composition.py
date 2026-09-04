#!/usr/bin/env python3
"""Version composition — EXPECTED (bundle.yaml) vs REAL (running image).

bundle.yaml is the single source for a bundle's layer map: the base image, the
postgres engine, every extension, and every runtime/web component (s6, busybox
httpd, cklib mount, NATS, the launcher, the dispatch relay) with its version and
mapping. This tool reads that map and — with --probe — stands the image up and
reads back each NATIVE version surface, so LATEST.md can carry test-CONFIRMED
versions, not just the ones we intended.

Probed live (a running pg image tells the truth):
  postgresql      SHOW server_version
  pgrdf  ext/fn   pg_extension.extversion / pgrdf.version()
  pgck   ext/fn   pg_extension.extversion / pgck_version()

Gate-confirmed (the bundle's gate-before-push smoke exercises these — busybox
serves /cklib, NATS answers core+WSS, the relay round-trips, etc.): base image,
pgcrypto, s6-overlay, busybox, cklib, nats-server, launcher, relay.

Usage:
  python3 tools/version-composition.py <bundle_dir> [--image REF] [--probe]
          [--json OUT] [--md]
Exit 0 if every PROBED layer matches expected (a stale native string is a
tracked deviation that prints ⚠ but does not fail — it is upstream's to fix and
is footnoted to the open NOTIFY). Non-zero on a real mismatch (wrong build).
"""
from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
import time

import yaml

PSQL_PROBES = {
    "postgres": "SHOW server_version;",
    "pgrdf_ext": "SELECT extversion FROM pg_extension WHERE extname='pgrdf';",
    "pgck_ext": "SELECT extversion FROM pg_extension WHERE extname='pgck';",
    "pgrdf_fn": "SELECT pgrdf.version();",
    "pgck_fn": "SELECT pgck_version();",
}

# Native version strings that are KNOWN-stale upstream and tracked by a NOTIFY.
# Listed here so the gate prints ⚠ instead of ✗ and does not block releases over
# a cosmetic upstream bug. Remove an entry the moment upstream ships the fix.
KNOWN_STALE = {
    # (empty) — pgck_version() de-staled in pgCK 0.4.15 (now CARGO_PKG_VERSION-derived);
    # no tracked native-version exceptions. Re-add an entry only if an upstream freezes
    # its self-report again.
}


def sh(args: list[str], **kw) -> subprocess.CompletedProcess:
    return subprocess.run(args, capture_output=True, text=True, check=False, **kw)


def expected_layers(bundle_dir: str) -> list[dict]:
    d = yaml.safe_load(open(os.path.join(bundle_dir, "bundle.yaml")))
    img = d.get("image", {}) or {}
    rows: list[dict] = []
    base = (img.get("base_image") or "").split("/")[-1]
    rows.append({
        "layer": "base image", "kind": "base",
        "expected": f"{base}:{img.get('base_image_version', '')}".strip(":"),
        "mapping": "FROM", "probe": None,
    })
    rows.append({
        "layer": "postgresql", "kind": "engine",
        "expected": str(img.get("pg_major", "17")), "mapping": "server",
        "probe": "postgres",
    })
    for name, v in (d.get("extensions") or {}).items():
        v = v or {}
        rows.append({
            "layer": name, "kind": "extension",
            "expected": str(v.get("version")), "mapping": "CREATE EXTENSION",
            "probe": f"{name}_ext" if f"{name}_ext" in PSQL_PROBES else None,
        })
        if f"{name}_fn" in PSQL_PROBES:  # the native .version() surface
            rows.append({
                "layer": f"{name}.version()", "kind": "native",
                "expected": str(v.get("version")), "mapping": "self-report",
                "probe": f"{name}_fn",
            })
    for name, v in (d.get("components") or {}).items():
        v = v or {}
        mp = v.get("mount_path") or (f"httpd :{v['port']}" if v.get("port") else v.get("role", "—"))
        rows.append({
            "layer": name, "kind": "component",
            "expected": str(v.get("version")), "mapping": mp, "probe": None,
        })
    return rows


def index_digest(image_ref: str) -> str:
    out = sh(["docker", "buildx", "imagetools", "inspect", image_ref])
    for line in out.stdout.splitlines():
        if line.strip().lower().startswith("digest:"):
            return line.split(":", 1)[1].strip()
    return ""


def probe(image_ref: str) -> dict:
    """Stand the image up, read each native version surface, tear it down."""
    suf = str(int(time.time()))
    net, name = f"vercomp-{suf}", f"vercomp-{suf}"
    sh(["docker", "network", "create", net])
    # OCIGER_CK_ADMIT_ANONYMOUS=on IS REQUIRED, and its absence was a silent
    # publishing defect. v0.7.43 made the image ship CLOSED: a container with no
    # realm REFUSES TO BOOT. This prober boots a bare container, so from v0.7.43
    # it never started — every probe then failed with "could not translate host
    # name", and those error strings were written into composition.confirmed.json
    # and PUBLISHED as the `Confirmed (real)` column of LATEST.md's composition
    # table, with all_match:false, for v0.7.43 and v0.7.44.
    #
    # This is a version probe, not an identity test: the anonymous tier is the
    # right posture for it, and declaring it is what the rest of the repo's
    # harnesses do since v0.7.43. Do not remove it to "simplify" the call.
    sh(["docker", "run", "-d", "--name", name, "--network", net,
        "-e", "POSTGRES_PASSWORD=vercomp", "-e", "OCIGER_CK_PARTICIPANT_PASSWORD=vercomp",
        "-e", "OCIGER_CK_ADMIT_ANONYMOUS=on",
        image_ref])
    try:
        for _ in range(60):
            logs = sh(["docker", "logs", name]).stdout + sh(["docker", "logs", name]).stderr
            if "ready to accept connections" in logs:
                break
            time.sleep(1)
        time.sleep(3)
        results = {}
        for key, q in PSQL_PROBES.items():
            r = sh(["docker", "run", "--rm", "--network", net, "-e", "PGPASSWORD=vercomp",
                    "postgres:18-trixie", "psql", "-h", name, "-U", "postgres",
                    "-d", "postgres", "-At", "-c", q])
            results[key] = (r.stdout.strip() or r.stderr.strip().splitlines()[0] if r.stderr else r.stdout.strip())
        results["_index_digest"] = index_digest(image_ref)
        return results
    finally:
        sh(["docker", "rm", "-f", name])
        sh(["docker", "network", "rm", net])


def verdict(row: dict, real: str | None) -> tuple[str, str]:
    """Return (confirmed_display, verdict_symbol)."""
    if row["probe"] is None:
        return ("gate-before-push", "✓ gated")
    if real is None or real == "":
        return ("—", "? unprobed")
    exp = row["expected"]
    # postgres: compare majors; native fn: expected must be a substring of real
    if row["probe"] == "postgres":
        ok = real.split(".")[0] == exp
    elif row["kind"] == "native":
        ok = exp in real
    else:
        ok = real == exp
    if ok:
        return (real, "✓")
    if KNOWN_STALE.get(row["probe"]) == real:
        return (real, "⚠ stale¹")
    return (real, "✗")


def compose(bundle_dir: str, probe_data: dict | None) -> dict:
    rows = expected_layers(bundle_dir)
    out_rows, all_ok = [], True
    for r in rows:
        real = (probe_data or {}).get(r["probe"]) if r["probe"] else None
        confirmed, sym = verdict(r, real)
        if sym == "✗":
            all_ok = False
        out_rows.append({**r, "confirmed": confirmed, "verdict": sym})
    return {
        "bundle": os.path.basename(bundle_dir.rstrip("/")),
        "index_digest": (probe_data or {}).get("_index_digest", ""),
        "all_match": all_ok,
        "layers": out_rows,
    }


def render_md(comp: dict) -> str:
    """A GitHub-flavoured markdown pipe table (LATEST.md / PROVENANCE.md are docs)."""
    out = [
        "| Layer | Kind | Expected | Mapping | Confirmed (real) | Verdict |",
        "|-------|------|----------|---------|------------------|---------|",
    ]
    has_stale = False
    for r in comp["layers"]:
        if "¹" in r["verdict"]:
            has_stale = True
        conf = f"`{r['confirmed']}`" if r["probe"] else r["confirmed"]
        out.append(
            f"| `{r['layer']}` | {r['kind']} | `{r['expected']}` | "
            f"`{r['mapping']}` | {conf} | {r['verdict']} |"
        )
    md = "\n".join(out)
    if has_stale:
        md += ("\n\n¹ native self-report frozen upstream (the extension build is the correct "
               "version — see `extversion`); tracked in the open pgCK `pgck_version()`-stale "
               "NOTIFY, not release-blocking.")
    return md


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("bundle_dir")
    ap.add_argument("--image")
    ap.add_argument("--probe", action="store_true")
    ap.add_argument("--json")
    ap.add_argument("--md", action="store_true")
    a = ap.parse_args()
    pdata = probe(a.image) if (a.probe and a.image) else None
    comp = compose(a.bundle_dir, pdata)
    if a.json:
        from datetime import datetime, timezone
        comp_out = dict(comp)
        comp_out["confirmed_at"] = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ") if pdata else None
        comp_out["image"] = a.image or ""
        with open(a.json, "w") as f:
            json.dump(comp_out, f, indent=2, ensure_ascii=False)
            f.write("\n")
        print(f"wrote {a.json} (all_match={comp['all_match']})", file=sys.stderr)
    if a.md or not a.json:
        print(render_md(comp))
    # Exit non-zero only on a real (non-stale) mismatch.
    real_fail = any(r["verdict"] == "✗" for r in comp["layers"])
    return 1 if real_fail else 0


if __name__ == "__main__":
    sys.exit(main())
