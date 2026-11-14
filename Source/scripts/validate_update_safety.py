#!/usr/bin/env python3
"""Check version and compatibility metadata before distributing an update.

The CI generates BRIDGE_COMPATIBILITY.json from the source version before this
validation. No older migration paths are allowed to re-enter the release.
"""
from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path

BUILDINFO = Path("Source/internal/buildinfo/buildinfo.go")
COMPATIBILITY = Path("BRIDGE_COMPATIBILITY.json")

VERSION_RE = re.compile(r"^[Vv]?(\d+)\.(\d+)(?:\.(\d+))?$")


def parse_version(value: str) -> tuple[int, int, int] | None:
    match = VERSION_RE.fullmatch(value.strip())
    if not match:
        return None
    major, minor, patch = match.groups()
    return int(major), int(minor), int(patch or 0)


def read_build_versions(repo: Path) -> tuple[str, str]:
    path = repo / BUILDINFO
    text = path.read_text(encoding="utf-8")
    numeric = re.search(r'\bVersion\s*=\s*"([^"]+)"', text)
    display = re.search(r'\bDisplayVersion\s*=\s*"([^"]+)"', text)
    if not numeric or not display:
        raise ValueError(f"unable to parse version constants in {BUILDINFO}")
    return numeric.group(1).strip(), display.group(1).strip()


def error(errors: list[str], message: str) -> None:
    errors.append(message)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo-root", default=".")
    args = parser.parse_args()
    repo = Path(args.repo_root).resolve()

    errors: list[str] = []

    # CRITICAL: the build must have a valid, internally consistent version.
    try:
        numeric_version, display_version = read_build_versions(repo)
    except Exception as exc:
        print(f"[update-safety] FAILED: {exc}", file=sys.stderr)
        return 1

    numeric_parts = parse_version(numeric_version)
    display_parts = parse_version(display_version)
    if numeric_parts is None:
        error(errors, f"invalid build Version: {numeric_version!r}")
    if display_parts is None:
        error(errors, f"invalid DisplayVersion: {display_version!r}")
    if numeric_parts is not None and display_parts is not None and numeric_parts[:2] != display_parts[:2]:
        error(errors, f"Version {numeric_version} and DisplayVersion {display_version} do not describe the same release")

    # The index is published alongside the matching build; reject stale data.
    compat_path = repo / COMPATIBILITY
    try:
        compat = json.loads(compat_path.read_text(encoding="utf-8"))
    except Exception as exc:
        error(errors, f"invalid {COMPATIBILITY}: {exc}")
        compat = None

    current_entry = None
    if isinstance(compat, dict):
        if compat.get("schema") != 1:
            error(errors, f"unsupported compatibility schema: {compat.get('schema')!r}")

        releases = compat.get("releases")
        if not isinstance(releases, list) or not releases:
            error(errors, "compatibility index contains no releases")
            releases = []

        matches = [
            item for item in releases
            if isinstance(item, dict)
            and str(item.get("bridgeVersion", "")).strip().lower() == display_version.lower()
        ]
        if len(matches) != 1:
            error(errors, f"compatibility index must contain exactly one current {display_version} entry; found {len(matches)}")
        else:
            current_entry = matches[0]

        # Building an older source tree while the index already points to a newer
        # stable Bridge is dangerous: it can re-publish stale update metadata.
        if display_parts is not None:
            for item in releases:
                if not isinstance(item, dict) or str(item.get("channel", "")).lower() != "stable":
                    continue
                candidate = parse_version(str(item.get("bridgeVersion", "")))
                if candidate is not None and candidate > display_parts:
                    error(errors, f"compatibility index contains newer stable release {item.get('bridgeVersion')} than this build {display_version}")

    expected_asset = f"Enhanced_PS5_DualSense_Haptics_Bridge_{display_version}.zip"
    if current_entry is not None:
        if str(current_entry.get("tag", "")).strip().lower() != display_version.lower():
            error(errors, f"current compatibility tag must match {display_version}")
        if str(current_entry.get("channel", "")).strip().lower() != "stable":
            error(errors, "current compatibility entry must be stable")
        if str(current_entry.get("asset", "")).strip() != expected_asset:
            error(errors, f"current compatibility asset must be {expected_asset}")

        protocols = current_entry.get("protocols")
        if not isinstance(protocols, list) or not protocols or any(not isinstance(p, int) or p <= 0 for p in protocols):
            error(errors, "current compatibility entry must declare at least one valid protocol")

        mod_min = str(current_entry.get("modMin", "")).strip()
        mod_max = str(current_entry.get("modMax", "")).strip()
        min_parts = parse_version(mod_min) if mod_min else None
        max_parts = parse_version(mod_max) if mod_max else None
        if mod_min and min_parts is None:
            error(errors, f"invalid modMin version: {mod_min}")
        if mod_max and max_parts is None:
            error(errors, f"invalid modMax version: {mod_max}")
        if min_parts is not None and max_parts is not None and min_parts > max_parts:
            error(errors, f"modMin {mod_min} is newer than modMax {mod_max}")
        if display_parts is not None:
            if min_parts is not None and display_parts < min_parts:
                error(errors, f"current release {display_version} is below its own modMin {mod_min}")
            if max_parts is not None and display_parts > max_parts:
                error(errors, f"current release {display_version} is above its own modMax {mod_max}")

    # Only the unified V1.43+ release line is supported. Protocol 41/42
    # migration entries must not reappear in an automatically generated index.
    if isinstance(compat, dict):
        generations = compat.get("protocols")
        if not isinstance(generations, list) or [g.get("id") for g in generations if isinstance(g, dict)] != [43]:
            error(errors, "only gameplay protocol 43 is supported in the current release")
        if len(releases) != 1:
            error(errors, "compatibility metadata must declare only the current stable release")
    if current_entry is not None:
        if current_entry.get("protocols") != [43]:
            error(errors, "current release must advertise only protocol 43")
        if str(current_entry.get("modMin", "")).strip() != "V1.43":
            error(errors, "supported update floor must remain V1.43")
        if str(current_entry.get("modMax", "")).strip() != display_version:
            error(errors, "modMax must match this build")

    if errors:
        print("[update-safety] FAILED", file=sys.stderr)
        for message in errors:
            print(f" - {message}", file=sys.stderr)
        return 1

    print(f"[update-safety] OK: {display_version}; critical update metadata is coherent")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
