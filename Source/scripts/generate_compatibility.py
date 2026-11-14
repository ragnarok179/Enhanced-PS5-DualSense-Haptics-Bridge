#!/usr/bin/env python3
"""Generate the compatibility index for the version being built.

GitHub Releases are published manually. In CI this script only prepares an
artifact; it never queries GitHub, modifies the remote repository, or guesses
that the latest *published* release is the new build version.
"""
from __future__ import annotations

import argparse
import json
import re
from pathlib import Path

MIN_SUPPORTED_MOD = "V1.43"
WIRE_INTRODUCED = "V1.41"


def read_current_version(repo: Path) -> str:
    source = (repo / "Source/internal/buildinfo/buildinfo.go").read_text(encoding="utf-8")
    match = re.search(r'DisplayVersion\s*=\s*"(V\d+\.\d+)"', source)
    if not match:
        raise ValueError("Expected DisplayVersion in Source/internal/buildinfo/buildinfo.go")
    return match.group(1)


def read_current_protocol(repo: Path) -> int:
    source = (repo / "Source/protocol.go").read_text(encoding="utf-8")
    version = re.search(r"\bprotocolVersion\s*=\s*(\d+)", source)
    minimum = re.search(r"\bprotocolMinVersion\s*=\s*(\d+)", source)
    maximum = re.search(r"\bprotocolMaxVersion\s*=\s*(\d+)", source)
    if not all((version, minimum, maximum)):
        raise ValueError("Cannot read the gameplay protocol bounds")
    values = [int(item.group(1)) for item in (version, minimum, maximum)]
    if len(set(values)) != 1:
        raise ValueError("Expected a single current protocol; old migrations must not be re-enabled")
    if values[0] != 43:
        raise ValueError("Gameplay protocol changed: reassess the V1.43 update floor before generating compatibility metadata")
    return values[0]


def generate(repo: Path) -> dict:
    version = read_current_version(repo)
    protocol = read_current_protocol(repo)
    return {
        "schema": 1,
        "protocolPolicy": "Manual updates from V1.43 onward. The updater checks complete stable GitHub Releases; older migration paths are not supported.",
        "protocols": [
            {
                "id": protocol,
                "status": "current",
                "introduced": WIRE_INTRODUCED,
                "description": "Gameplay wire protocol shared by the V1.43+ release line.",
            }
        ],
        "releases": [
            {
                "bridgeVersion": version,
                "tag": version,
                "channel": "stable",
                "protocols": [protocol],
                "modMin": MIN_SUPPORTED_MOD,
                "modMax": version,
                "asset": f"Enhanced_PS5_DualSense_Haptics_Bridge_{version}.zip",
            }
        ],
    }


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo-root", type=Path, default=Path(__file__).resolve().parents[2])
    parser.add_argument("--check", action="store_true", help="verify committed JSON instead of writing it")
    args = parser.parse_args()
    repo = args.repo_root.resolve()
    target = repo / "BRIDGE_COMPATIBILITY.json"
    output = json.dumps(generate(repo), indent=2, ensure_ascii=False) + "\n"
    if args.check:
        if not target.exists() or target.read_text(encoding="utf-8") != output:
            raise SystemExit(f"{target} is not synchronized with the build version")
        print("[compatibility] checked:", read_current_version(repo))
    else:
        target.write_text(output, encoding="utf-8")
        print("[compatibility] generated:", read_current_version(repo))


if __name__ == "__main__":
    main()
