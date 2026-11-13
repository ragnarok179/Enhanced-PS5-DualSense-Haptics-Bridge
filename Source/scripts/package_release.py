#!/usr/bin/env python3
"""Build and package a verified V1.50 Windows Bridge and BeamNG mod.

Uses only the Python standard library and the selected Go toolchain.
The same deterministic ZIP builder is used for the Bridge, mod and bundle.
Never auto-publishes or edits the user's BeamNG installation.
"""
from __future__ import annotations

import argparse
import hashlib
import os
import re
import shutil
import sys
import subprocess
import tempfile
import zipfile
from pathlib import Path

ZIP_DATE = (1980, 1, 1, 0, 0, 0)
BRIDGE_EXES = {
    "Bridge/EnhancedPS5DualSenseHapticsUSB.exe": ("-tags", "usb", "."),
    "Bridge/EnhancedPS5DualSenseHapticsBluetooth.exe": ("-tags", "bluetooth", "."),
    "START_BRIDGE.exe": ("./launcher",),
    "UPDATE_DUALSENSE.exe": ("./updater",),
}
RUNTIME_FILES = (
    "BRIDGE_COMPATIBILITY.json",
    "Config/feel_profile_v1.json",
    "Diagnostics/OPEN_LOGS_FOLDER.bat",
    "Diagnostics/RUN_DIAGNOSTIC.bat",
    "Diagnostics/Run-Diagnostic.ps1",
    "IMPORTANT_GUIDE_-_RUN_UPDATE_DUALSENSE_TO_FINISH_UPDATE.txt",
    "LICENSE",
    "README.md",
    "THIRD_PARTY_NOTICES.md",
)


def sha256(path: Path) -> str:
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def package_zip(source_files: dict[str, Path], destination: Path) -> None:
    """Create ZIPs without host-dependent mtimes, ordering or permissions."""
    destination.parent.mkdir(parents=True, exist_ok=True)
    with zipfile.ZipFile(destination, "w", zipfile.ZIP_DEFLATED, compresslevel=9) as out:
        for archive_name, source in sorted(source_files.items()):
            if not source.is_file():
                raise FileNotFoundError(f"Release file missing: {source}")
            item = zipfile.ZipInfo(archive_name, date_time=ZIP_DATE)
            item.compress_type = zipfile.ZIP_DEFLATED
            item.external_attr = 0o100644 << 16
            out.writestr(item, source.read_bytes(), compress_type=zipfile.ZIP_DEFLATED, compresslevel=9)


def checksum_manifest(source_files: dict[str, Path]) -> bytes:
    lines = (f"{sha256(path)}  ./{name}" for name, path in sorted(source_files.items()))
    return ("\n".join(lines) + "\n").encode("ascii")


def build_windows(source: Path, stage: Path) -> dict[str, Path]:
    env = dict(os.environ, GOOS="windows", GOARCH="amd64", CGO_ENABLED="0")
    binaries: dict[str, Path] = {}
    for rel, args in BRIDGE_EXES.items():
        dest = stage / rel
        dest.parent.mkdir(parents=True, exist_ok=True)
        cmd = ["go", "build", "-trimpath", "-buildvcs=false", "-ldflags=-s -w -buildid=", *args[:-1], "-o", str(dest), args[-1]]
        print("[build]", rel, flush=True)
        subprocess.run(cmd, cwd=source, env=env, check=True)
        binaries[rel] = dest
    binaries["START_BRIDGE_AND_BEAMNG.exe"] = binaries["START_BRIDGE.exe"]
    binaries["UPDATE_BRIDGE.exe"] = binaries["UPDATE_DUALSENSE.exe"]
    return binaries


def verify_version(repo: Path) -> str:
    source = (repo / "Source/internal/buildinfo/buildinfo.go").read_text(encoding="utf-8")
    match = re.search(r'DisplayVersion\s*=\s*"(V\d+\.\d+)"', source)
    if not match:
        raise ValueError("Could not read display version")
    version = match.group(1)
    subprocess.run([sys.executable, str(repo / "Source/scripts/validate_update_safety.py"), "--repo-root", str(repo)], check=True)
    return version


def ensure_release_contents(repo: Path, stage: Path, version: str, binaries: dict[str, Path]) -> None:
    release = repo / "Release"
    release.mkdir(exist_ok=True)
    binary_files = {name: path for name, path in binaries.items()}
    runtime_files = {name: repo / name for name in RUNTIME_FILES}
    manifest_content = checksum_manifest({**runtime_files, **binary_files})
    checksum_file = stage / "SHA256SUMS.txt"
    checksum_file.write_bytes(manifest_content)
    bridge_files = {**runtime_files, **binary_files, "SHA256SUMS.txt": checksum_file}
    for name, path in binary_files.items():
        dest = repo / name
        dest.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(path, dest)
    shutil.copyfile(checksum_file, repo / "SHA256SUMS.txt")

    # The game mod must contain no Source/, Bridge/ or Windows executable entries.
    mod_root = repo / "BeamNG-Mod"
    mod_files = {path.relative_to(mod_root).as_posix(): path for path in mod_root.rglob("*") if path.is_file()}
    if not mod_files or any(name.lower().endswith((".exe", ".dll")) for name in mod_files):
        raise ValueError("BeamNG mod payload is empty or contains a Windows executable")
    mod_zip = release / f"Enhanced_PS5_DualSense_Haptics_{version}.zip"
    bridge_zip = release / f"Enhanced_PS5_DualSense_Haptics_Bridge_{version}.zip"
    package_zip(mod_files, mod_zip)
    package_zip(bridge_files, bridge_zip)
    shutil.copyfile(repo / "BRIDGE_COMPATIBILITY.json", release / "BRIDGE_COMPATIBILITY.json")

    provenance = release / "BUILD_PROVENANCE.txt"
    go_version = subprocess.check_output(["go", "version"], text=True).strip()
    provenance.write_text(
        f"Release: {version}\nToolchain: {go_version}\n"
        "GOOS=windows GOARCH=amd64 CGO_ENABLED=0\n"
        "Build flags: -trimpath -buildvcs=false -ldflags=-s -w -buildid=\n"
        "Source: Source/scripts/package_release.py\n",
        encoding="utf-8",
    )
    checksums = {
        "BUILD_PROVENANCE.txt": provenance,
        mod_zip.name: mod_zip,
        bridge_zip.name: bridge_zip,
        "BRIDGE_COMPATIBILITY.json": release / "BRIDGE_COMPATIBILITY.json",
        f"RELEASE_NOTES_{version}.md": release / f"RELEASE_NOTES_{version}.md",
        f"MOD_ACTIVATION_TEST_{version}.md": release / f"MOD_ACTIVATION_TEST_{version}.md",
    }
    (release / "RELEASE_SHA256SUMS.txt").write_bytes(checksum_manifest(checksums))

    bundle_manifest = stage / f"{version}_SHA256SUMS.txt"
    bundle_items = {**checksums}
    del bundle_items[f"RELEASE_NOTES_{version}.md"]
    bundle_items["README.md"] = repo / "README.md"
    bundle_manifest.write_bytes(checksum_manifest(bundle_items))
    bundle = release / f"Enhanced_PS5_DualSense_Haptics_{version}_INSTALLATION_BUNDLE.zip"
    package_zip({**bundle_items, bundle_manifest.name: bundle_manifest}, bundle)
    print(f"[release] {mod_zip.name}: {sha256(mod_zip)}")
    print(f"[release] {bridge_zip.name}: {sha256(bridge_zip)}")
    print(f"[release] {bundle.name}: {sha256(bundle)}")


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo-root", type=Path, default=Path(__file__).resolve().parents[2])
    args = parser.parse_args()
    repo = args.repo_root.resolve()
    version = verify_version(repo)
    with tempfile.TemporaryDirectory(prefix="dualsense-release-") as tmp:
        stage = Path(tmp)
        binaries = build_windows(repo / "Source", stage)
        ensure_release_contents(repo, stage, version, binaries)


if __name__ == "__main__":
    main()
