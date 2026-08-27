package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Ragnarok179/enhanced-ps5-dualsense-haptics-bridge/internal/compatibility"
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
		normalizeRelative("START_BRIDGE.exe"):                          "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		normalizeRelative(`Bridge\EnhancedPS5DualSenseHapticsUSB.exe`): "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
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
		TagName: "v1.4.0",
		Assets: []githubAsset{
			{Name: "notes.txt", BrowserDownloadURL: "https://example.invalid/notes.txt"},
			{Name: compatibility.DefaultReleaseAsset, BrowserDownloadURL: "https://example.invalid/bridge.zip"},
		},
	}

	asset, ok := findReleaseAsset(release, compatibility.DefaultReleaseAsset)
	if !ok {
		t.Fatal("expected release package asset to be found")
	}
	if asset.BrowserDownloadURL != "https://example.invalid/bridge.zip" {
		t.Fatalf("unexpected asset URL %q", asset.BrowserDownloadURL)
	}
}

func TestFindReleaseAssetRequiresDownloadURL(t *testing.T) {
	release := githubRelease{
		TagName: "v1.4.0",
		Assets: []githubAsset{
			{Name: compatibility.DefaultReleaseAsset},
		},
	}

	if _, ok := findReleaseAsset(release, compatibility.DefaultReleaseAsset); ok {
		t.Fatal("asset without a browser download URL must not be accepted")
	}
}

func TestSelectPublishedCandidateUsesCompatibilityAsset(t *testing.T) {
	candidates := []compatibility.Release{
		{BridgeVersion: "V1.5", Tag: "V1.5", Channel: "stable", Protocols: []int{43}, Asset: "future.zip"},
		{BridgeVersion: "V1.4", Tag: "V1.4", Channel: "stable", Protocols: []int{40, 41, 42}, Asset: compatibility.DefaultReleaseAsset},
	}
	releases := []githubRelease{
		{TagName: "V1.4", Assets: []githubAsset{{Name: compatibility.DefaultReleaseAsset, BrowserDownloadURL: "https://example.invalid/v14.zip"}}},
	}
	selected, release, asset, ok := selectPublishedCandidate(candidates, releases)
	if !ok || selected.Tag != "V1.4" || release.TagName != "V1.4" || asset.BrowserDownloadURL == "" {
		t.Fatalf("wrong candidate selection: %+v %+v %+v ok=%t", selected, release, asset, ok)
	}
}

func TestProtocol42IsSupportedByV14CompatibilityData(t *testing.T) {
	idx := compatibility.Index{Schema: 1, Protocols: []compatibility.ProtocolGeneration{{ID: 40}, {ID: 41}, {ID: 42}}, Releases: []compatibility.Release{{BridgeVersion: "V1.4", Tag: "V1.4", Channel: "stable", Protocols: []int{40, 41, 42}}}}
	got := compatibility.CompatibleCandidates(idx, compatibility.Target{ModVersion: "V1.2", Protocol: 42})
	if len(got) != 1 || got[0].BridgeVersion != "V1.4" {
		t.Fatalf("V1.4 must support V1.2 protocol 42: %+v", got)
	}
}

func TestUpdaterNeverDowngradesCurrentBridge(t *testing.T) {
	candidates := []compatibility.Release{{BridgeVersion: "V1.3"}, {BridgeVersion: "V1.4"}, {BridgeVersion: "V1.5"}}
	got := newerReleaseCandidates(candidates, "V1.4")
	if len(got) != 1 || got[0].BridgeVersion != "V1.5" {
		t.Fatalf("downgrade/current candidates were not filtered: %+v", got)
	}
}
