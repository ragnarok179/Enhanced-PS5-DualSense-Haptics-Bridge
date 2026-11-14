#!/usr/bin/env python3
"""Reject malformed release archives and stale Bridge binaries before upload."""
from __future__ import annotations

import argparse
import hashlib
import json
import re
import zipfile
from pathlib import Path


def digest(contents: bytes) -> str:
    return hashlib.sha256(contents).hexdigest()


def manifest_entries(contents: bytes) -> dict[str, str]:
    entries = {}
    for line in contents.decode("ascii").splitlines():
        match = re.fullmatch(r"([0-9a-f]{64})  \./(.+)", line)
        if not match:
            raise ValueError(f"Invalid checksum line: {line!r}")
        entries[match[2]] = match[1]
    if not entries:
        raise ValueError("Checksum manifest must not be empty")
    return entries


def verify_zip(zip_path: Path) -> None:
    with zipfile.ZipFile(zip_path) as archive:
        names = archive.namelist()
        if len(names) != len(set(names)) or archive.testzip() is not None:
            raise ValueError(f"Duplicate or damaged ZIP entries in {zip_path}")
        for name in names:
            path = Path(name)
            if path.is_absolute() or ".." in path.parts or "\\" in name:
                raise ValueError(f"Unsafe archive entry: {name}")


def verify_bridge(repo: Path, path: Path) -> None:
    verify_zip(path)
    with zipfile.ZipFile(path) as archive:
        # The published V1.43 updater rejects packages missing UPDATE_BRIDGE.exe.
        # It must be an alias of the *current* updater, not old migration code.
        if archive.read("UPDATE_BRIDGE.exe") != archive.read("UPDATE_DUALSENSE.exe"):
            raise ValueError("V1.43 updater compatibility alias is missing or differs from the unified updater")
        manifest = manifest_entries(archive.read("SHA256SUMS.txt"))
        if set(manifest) != set(archive.namelist()) - {"SHA256SUMS.txt"}:
            raise ValueError("Bridge manifest and file list differ")
        for name, expected in manifest.items():
            if digest(archive.read(name)) != expected:
                raise ValueError(f"Wrong Bridge checksum: {name}")
            source = repo / name
            if not source.is_file() or digest(source.read_bytes()) != expected:
                raise ValueError(f"Release Bridge entry is stale relative to repository: {name}")


def verify_mod(repo: Path, path: Path, version: str) -> None:
    verify_zip(path)
    with zipfile.ZipFile(path) as archive:
        archive_names = set(archive.namelist())
        source_files = {f.relative_to(repo / "BeamNG-Mod").as_posix(): f for f in (repo / "BeamNG-Mod").rglob("*") if f.is_file()}
        if archive_names != set(source_files):
            raise ValueError("Mod ZIP content differs from BeamNG-Mod source tree")
        for name, file in source_files.items():
            if digest(archive.read(name)) != digest(file.read_bytes()):
                raise ValueError(f"Wrong mod content: {name}")
        mod_info_path = "mod_info/EnhancedPS5DualSenseHaptics/info.json"
        info = json.loads(archive.read(mod_info_path))
        if info.get("version") != version[1:] + ".0":
            raise ValueError("Mod and Bridge versions differ")
        mod_info_id = Path(mod_info_path).parent.name.upper()
        if info.get("tagid") != mod_info_id:
            raise ValueError("Mod Manager cannot resolve the UI ID without matching modData.tagid")
        if any(name.lower().endswith((".exe", ".dll")) for name in archive_names):
            raise ValueError("Mod ZIP contains a Windows executable")


def verify_manifests(release: Path) -> None:
    for name, expected in manifest_entries((release / "RELEASE_SHA256SUMS.txt").read_bytes()).items():
        file = release / name
        if not file.is_file() or digest(file.read_bytes()) != expected:
            raise ValueError(f"Wrong release checksum: {name}")


def verify_bundle(release: Path, version: str) -> None:
    file = release / f"Enhanced_PS5_DualSense_Haptics_{version}_INSTALLATION_BUNDLE.zip"
    verify_zip(file)
    with zipfile.ZipFile(file) as archive:
        checksum_name = f"{version}_SHA256SUMS.txt"
        manifest = manifest_entries(archive.read(checksum_name))
        if set(archive.namelist()) != set(manifest) | {checksum_name}:
            raise ValueError("Bundle manifest and file list differ")
        for name, expected in manifest.items():
            if digest(archive.read(name)) != expected:
                raise ValueError(f"Wrong bundle checksum: {name}")
            original = release / name if (release / name).is_file() else release.parent / name
            if not original.is_file() or digest(original.read_bytes()) != expected:
                raise ValueError(f"Bundle contains a stale artifact: {name}")


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo-root", type=Path, default=Path(__file__).resolve().parents[2])
    args = parser.parse_args()
    repo = args.repo_root.resolve()
    compat = json.loads((repo / "BRIDGE_COMPATIBILITY.json").read_text(encoding="utf8"))
    current = [r for r in compat["releases"] if r.get("channel") == "stable"]
    if len(current) != 1:
        raise ValueError("Expected a single current stable release")
    version = current[0]["bridgeVersion"]
    release = repo / "Release"
    if (release / "BRIDGE_COMPATIBILITY.json").read_bytes() != (repo / "BRIDGE_COMPATIBILITY.json").read_bytes():
        raise ValueError("Release and repository compatibility metadata differ")
    verify_bridge(repo, release / f"Enhanced_PS5_DualSense_Haptics_Bridge_{version}.zip")
    verify_mod(repo, release / f"Enhanced_PS5_DualSense_Haptics_{version}.zip", version)
    verify_manifests(release)
    verify_bundle(release, version)
    print(f"[release-check] OK: {version}; mod, Bridge, bundle and manifests are consistent")


if __name__ == "__main__":
    main()
