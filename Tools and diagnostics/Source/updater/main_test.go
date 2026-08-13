package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCompatibilityFilesAreFiltered(t *testing.T) {
	manifest := map[string]string{
		normalizeRelative("START_BRIDGE.bat"):                                "d",
		normalizeRelative("UPDATE_BRIDGE.bat"):                               "a",
		normalizeRelative(`Tools and diagnostics\Updater\Update-Bridge.ps1`): "b",
		normalizeRelative("UPDATE_BRIDGE.exe"):                               "c",
	}

	filtered := withoutCompatibilityFiles(manifest)
	if len(filtered) != 1 {
		t.Fatalf("expected one managed file, got %d", len(filtered))
	}
	if filtered[normalizeRelative("UPDATE_BRIDGE.exe")] != "c" {
		t.Fatal("UPDATE_BRIDGE.exe should remain managed")
	}
}

func TestCompareInstallationRemovesCompatibilityScripts(t *testing.T) {
	root := t.TempDir()
	legacy := normalizeRelative("UPDATE_BRIDGE.bat")
	legacyPath := filepath.Join(root, legacy)
	if err := os.WriteFile(legacyPath, []byte("legacy"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, removed, err := compareInstallation(root, map[string]string{}, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 1 || removed[0] != legacy {
		t.Fatalf("expected legacy updater to be removed, got %v", removed)
	}
}

func TestManifestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), manifestName)
	manifest := map[string]string{
		normalizeRelative("START_BRIDGE.exe"):                                                "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		normalizeRelative(`Tools and diagnostics\Bridge\EnhancedPS5DualSenseHapticsUSB.exe`): "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}
	if err := writeManifest(path, manifest); err != nil {
		t.Fatal(err)
	}
	readBack, err := readManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(readBack) != len(manifest) {
		t.Fatalf("expected %d entries, got %d", len(manifest), len(readBack))
	}
	for key, value := range manifest {
		if readBack[key] != value {
			t.Fatalf("manifest mismatch for %q", key)
		}
	}
}

func TestFindReleaseAsset(t *testing.T) {
	release := githubRelease{
		TagName: "V1.2",
		Assets: []githubAsset{
			{Name: "notes.txt", BrowserDownloadURL: "https://example.invalid/notes.txt"},
			{Name: releaseAssetName, BrowserDownloadURL: "https://example.invalid/bridge.zip"},
		},
	}

	asset, ok := findReleaseAsset(release, releaseAssetName)
	if !ok {
		t.Fatal("expected release package asset to be found")
	}
	if asset.BrowserDownloadURL != "https://example.invalid/bridge.zip" {
		t.Fatalf("unexpected asset URL %q", asset.BrowserDownloadURL)
	}
}

func TestFindReleaseAssetRequiresDownloadURL(t *testing.T) {
	release := githubRelease{
		TagName: "V1.2",
		Assets: []githubAsset{
			{Name: releaseAssetName},
		},
	}

	if _, ok := findReleaseAsset(release, releaseAssetName); ok {
		t.Fatal("asset without a browser download URL must not be accepted")
	}
}
