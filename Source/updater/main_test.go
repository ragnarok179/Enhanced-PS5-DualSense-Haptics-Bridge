package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
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
		{BridgeVersion: "V1.41", Tag: "V1.41", Channel: "stable", Protocols: []int{43}, Asset: "future.zip"},
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
	candidates := []compatibility.Release{{BridgeVersion: "V1.3"}, {BridgeVersion: "V1.4"}, {BridgeVersion: "V1.41"}}
	got := newerReleaseCandidates(candidates, "V1.4")
	if len(got) != 1 || got[0].BridgeVersion != "V1.41" {
		t.Fatalf("downgrade/current candidates were not filtered: %+v", got)
	}
}

func TestUnifiedReleaseRequiresMatchingModAndBridgeAssets(t *testing.T) {
	release := githubRelease{
		TagName: "V1.41",
		Assets: []githubAsset{
			{Name: "Enhanced_PS5_DualSense_Haptics_V1.41.zip", BrowserDownloadURL: "https://example.invalid/mod.zip"},
			{Name: "Enhanced_PS5_DualSense_Haptics_Bridge_V1.41.zip", BrowserDownloadURL: "https://example.invalid/bridge.zip"},
		},
	}
	modAsset, bridgeAsset, ok := projectAssetsForRelease(release)
	if !ok || modAsset.Name == "" || bridgeAsset.Name == "" {
		t.Fatalf("complete V1.41 release was not accepted: mod=%+v bridge=%+v ok=%t", modAsset, bridgeAsset, ok)
	}

	release.Assets[0].Name = "Enhanced_PS5_DualSense_Haptics_V1.4.zip"
	if _, _, ok := projectAssetsForRelease(release); ok {
		t.Fatal("release with mismatched mod version must not be accepted")
	}
}

func TestUnifiedReleaseSelectionSkipsIncompleteAndChoosesNewest(t *testing.T) {
	releases := []githubRelease{
		{TagName: "V1.42", Assets: []githubAsset{{Name: "Enhanced_PS5_DualSense_Haptics_Bridge_V1.42.zip", BrowserDownloadURL: "https://example.invalid/v16bridge.zip"}}},
		{TagName: "V1.41", Assets: []githubAsset{
			{Name: "Enhanced_PS5_DualSense_Haptics_V1.41.zip", BrowserDownloadURL: "https://example.invalid/v141mod.zip"},
			{Name: "Enhanced_PS5_DualSense_Haptics_Bridge_V1.41.zip", BrowserDownloadURL: "https://example.invalid/v141bridge.zip"},
		}},
		{TagName: "V1.4", Assets: []githubAsset{
			{Name: "Enhanced_PS5_DualSense_Haptics_V1.4.zip", BrowserDownloadURL: "https://example.invalid/v14mod.zip"},
			{Name: "Enhanced_PS5_DualSense_Haptics_Bridge_V1.4.zip", BrowserDownloadURL: "https://example.invalid/v14bridge.zip"},
		}},
	}
	got, ok := selectUnifiedProjectRelease(releases, "V1.4")
	if !ok || got.Version != "V1.41" {
		t.Fatalf("expected newest complete release V1.41, got %+v ok=%t", got, ok)
	}
}

func TestVersionFromProjectAssetName(t *testing.T) {
	for name, want := range map[string]string{
		"Enhanced_PS5_DualSense_Haptics_V1.41.zip":        "V1.41",
		"Enhanced_PS5_DualSense_Haptics_Bridge_V1.41.zip": "V1.41",
	} {
		if got := versionFromProjectAssetName(name); got != want {
			t.Fatalf("%s: got %q want %q", name, got, want)
		}
	}
	if got := versionFromProjectAssetName("Enhanced_PS5_DualSense_Haptics_Bridge.zip"); got != "" {
		t.Fatalf("legacy unversioned asset must not be treated as unified release asset, got %q", got)
	}
}

func TestVerifyBeamNGModArchiveRejectsExecutableAndWrongVersion(t *testing.T) {
	makeZip := func(path, version string, executable bool) {
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		zw := zip.NewWriter(f)
		for name, body := range map[string]string{
			"lua/dualsensePhysicsHaptics/config.lua":         "return {}",
			"mod_info/EnhancedPS5DualSenseHaptics/info.json": `{"name":"Enhanced PS5 DualSense Haptics","version":"` + version + `"}`,
		} {
			w, _ := zw.Create(name)
			_, _ = w.Write([]byte(body))
		}
		if executable {
			w, _ := zw.Create("bad.exe")
			_, _ = w.Write([]byte("x"))
		}
		_ = zw.Close()
		_ = f.Close()
	}
	dir := t.TempDir()
	good := filepath.Join(dir, "good.zip")
	makeZip(good, "1.41.0", false)
	if err := verifyBeamNGModArchive(good, "V1.41"); err != nil {
		t.Fatalf("valid mod rejected: %v", err)
	}
	wrong := filepath.Join(dir, "wrong.zip")
	makeZip(wrong, "1.4.0", false)
	if err := verifyBeamNGModArchive(wrong, "V1.41"); err == nil {
		t.Fatal("wrong-version mod accepted")
	}
	bad := filepath.Join(dir, "bad.zip")
	makeZip(bad, "1.41.0", true)
	if err := verifyBeamNGModArchive(bad, "V1.41"); err == nil {
		t.Fatal("mod containing EXE accepted")
	}
}

func TestLegacyUpdaterCanLandOnV141MigrationHub(t *testing.T) {
	idx := compatibility.Index{
		Schema:    1,
		Protocols: []compatibility.ProtocolGeneration{{ID: 40}, {ID: 41}, {ID: 42}, {ID: 43}},
		Releases: []compatibility.Release{
			{BridgeVersion: "V1.3", Tag: "V1.3", Channel: "stable", Protocols: []int{40, 41}, Asset: "Enhanced_PS5_DualSense_Haptics_Bridge.zip"},
			{BridgeVersion: "V1.4", Tag: "V1.4", Channel: "stable", Protocols: []int{40, 41, 42}, Asset: "Enhanced_PS5_DualSense_Haptics_Bridge.zip"},
			{BridgeVersion: "V1.41", Tag: "V1.41", Channel: "stable", Protocols: []int{40, 41, 42, 43}, Asset: "Enhanced_PS5_DualSense_Haptics_Bridge_V1.41.zip"},
		},
	}
	for _, target := range []compatibility.Target{
		{ModVersion: "V1.1", Protocol: 41},
		{ModVersion: "V1.2", Protocol: 42},
		{ModVersion: "V1.41", Protocol: 43},
	} {
		candidates := compatibility.CompatibleCandidates(idx, target)
		newer := newerReleaseCandidates(candidates, "V1.4")
		if len(newer) == 0 || newer[0].BridgeVersion != "V1.41" || newer[0].Asset != "Enhanced_PS5_DualSense_Haptics_Bridge_V1.41.zip" {
			t.Fatalf("legacy migration target %+v cannot land on V1.41: %+v", target, newer)
		}
	}
}

func TestV141SortsNewerThanPublicLegacyVersions(t *testing.T) {
	for _, older := range []string{"V1.3", "V1.4"} {
		if compatibility.CompareVersions("V1.41", older) <= 0 {
			t.Fatalf("V1.41 must sort newer than %s for legacy updater migration", older)
		}
	}
}

func TestPublicV13StyleUpdaterSelectsV141Asset(t *testing.T) {
	idx := compatibility.Index{
		Schema:    1,
		Protocols: []compatibility.ProtocolGeneration{{ID: 40}, {ID: 41}, {ID: 42}, {ID: 43}},
		Releases: []compatibility.Release{
			{BridgeVersion: "V1.3", Tag: "V1.3", Channel: "stable", Protocols: []int{40, 41}, Asset: "Enhanced_PS5_DualSense_Haptics_Bridge.zip"},
			{BridgeVersion: "V1.4", Tag: "V1.4", Channel: "stable", Protocols: []int{40, 41, 42}, Asset: "Enhanced_PS5_DualSense_Haptics_Bridge.zip"},
			{BridgeVersion: "V1.41", Tag: "V1.41", Channel: "stable", Protocols: []int{40, 41, 42, 43}, Asset: "Enhanced_PS5_DualSense_Haptics_Bridge_V1.41.zip"},
		},
	}
	releases := []githubRelease{
		{TagName: "V1.3", Assets: []githubAsset{{Name: "Enhanced_PS5_DualSense_Haptics_Bridge.zip", BrowserDownloadURL: "https://example.invalid/v13.zip"}}},
		{TagName: "V1.4", Assets: []githubAsset{{Name: "Enhanced_PS5_DualSense_Haptics_Bridge.zip", BrowserDownloadURL: "https://example.invalid/v14.zip"}}},
		{TagName: "V1.41", Assets: []githubAsset{{Name: "Enhanced_PS5_DualSense_Haptics_Bridge_V1.41.zip", BrowserDownloadURL: "https://example.invalid/v141.zip"}}},
	}

	// Public V1.3 compatibility mode used CompatibleCandidates followed directly
	// by selectPublishedCandidate. Protocol 41 must therefore land on V1.41.
	candidates := compatibility.CompatibleCandidates(idx, compatibility.Target{ModVersion: "V1.1", Protocol: 41})
	selected, release, asset, ok := selectPublishedCandidate(candidates, releases)
	if !ok || selected.BridgeVersion != "V1.41" || release.TagName != "V1.41" || asset.Name != "Enhanced_PS5_DualSense_Haptics_Bridge_V1.41.zip" {
		t.Fatalf("V1.3-style migration did not select V1.41: selected=%+v release=%+v asset=%+v ok=%t", selected, release, asset, ok)
	}
}

func TestPublicV14StyleUpdaterSelectsV141Asset(t *testing.T) {
	idx := compatibility.Index{
		Schema:    1,
		Protocols: []compatibility.ProtocolGeneration{{ID: 40}, {ID: 41}, {ID: 42}, {ID: 43}},
		Releases: []compatibility.Release{
			{BridgeVersion: "V1.4", Tag: "V1.4", Channel: "stable", Protocols: []int{40, 41, 42}, Asset: "Enhanced_PS5_DualSense_Haptics_Bridge.zip"},
			{BridgeVersion: "V1.41", Tag: "V1.41", Channel: "stable", Protocols: []int{40, 41, 42, 43}, Asset: "Enhanced_PS5_DualSense_Haptics_Bridge_V1.41.zip"},
		},
	}
	releases := []githubRelease{
		{TagName: "V1.4", Assets: []githubAsset{{Name: "Enhanced_PS5_DualSense_Haptics_Bridge.zip", BrowserDownloadURL: "https://example.invalid/v14.zip"}}},
		{TagName: "V1.41", Assets: []githubAsset{{Name: "Enhanced_PS5_DualSense_Haptics_Bridge_V1.41.zip", BrowserDownloadURL: "https://example.invalid/v141.zip"}}},
	}

	// Public V1.4 filters out current/older candidates before selecting the asset.
	candidates := compatibility.CompatibleCandidates(idx, compatibility.Target{ModVersion: "V1.2", Protocol: 42})
	candidates = newerReleaseCandidates(candidates, "V1.4")
	selected, release, asset, ok := selectPublishedCandidate(candidates, releases)
	if !ok || selected.BridgeVersion != "V1.41" || release.TagName != "V1.41" || asset.Name != "Enhanced_PS5_DualSense_Haptics_Bridge_V1.41.zip" {
		t.Fatalf("V1.4-style migration did not select V1.41: selected=%+v release=%+v asset=%+v ok=%t", selected, release, asset, ok)
	}
}

func TestCanonicalUnifiedReleaseVersion(t *testing.T) {
	for in, want := range map[string]string{"V1.41": "V1.41", "v1.41": "V1.41", "V1.41.0": "V1.41"} {
		if got := canonicalReleaseVersion(in); got != want {
			t.Fatalf("%s: got %q want %q", in, got, want)
		}
	}
	for _, bad := range []string{"V1", "V1.41.1", "V1.41-beta", "latest"} {
		if got := canonicalReleaseVersion(bad); got != "" {
			t.Fatalf("invalid release %q accepted as %q", bad, got)
		}
	}
}

func TestStageModForManualInstallNeverNeedsBeamNGPath(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.zip")
	if err := os.WriteFile(source, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := stageModForManualInstall(source, "Enhanced_PS5_DualSense_Haptics_V1.41.zip", root)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(got) != "Enhanced_PS5_DualSense_Haptics_V1.41.zip" {
		t.Fatalf("unexpected staged filename: %s", got)
	}
	if _, err := os.Stat(got); err != nil {
		t.Fatalf("staged file missing: %v", err)
	}
}

func TestManualInstallGuideContainsOnlyUserInstallationSteps(t *testing.T) {
	guide := manualInstallGuideText("V1.41")
	for _, want := range []string{"BEAMNG MOD INSTALLATION", "Mod Manager", "Repository page", "Unsubscribe and remove", "Do NOT extract", "make sure the mod is enabled"} {
		if !strings.Contains(guide, want) {
			t.Fatalf("tutorial missing %q: %s", want, guide)
		}
	}
	for _, forbidden := range []string{"ONE-TIME MIGRATION", "What this updater will do", "What it will NOT do", "downloads and verifies", "Bridge-only updater", "does not search"} {
		if strings.Contains(guide, forbidden) {
			t.Fatalf("tutorial contains non-user or migration explanation %q: %s", forbidden, guide)
		}
	}
}
