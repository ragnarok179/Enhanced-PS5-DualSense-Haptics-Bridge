package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Ragnarok179/enhanced-ps5-dualsense-haptics-bridge/internal/compatibility"
)

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
		{BridgeVersion: "V1.42", Tag: "V1.42", Channel: "stable", Protocols: []int{41, 42, 43}, ModMin: "V1.3", ModMax: "V1.42", Asset: "Enhanced_PS5_DualSense_Haptics_Bridge_V1.42.zip"},
	}
	releases := []githubRelease{
		{TagName: "V1.42", Assets: []githubAsset{{Name: "Enhanced_PS5_DualSense_Haptics_Bridge_V1.42.zip", BrowserDownloadURL: "https://example.invalid/v142.zip"}}},
	}
	selected, release, asset, ok := selectPublishedCandidate(candidates, releases)
	if !ok || selected.Tag != "V1.42" || release.TagName != "V1.42" || asset.BrowserDownloadURL == "" {
		t.Fatalf("wrong candidate selection: %+v %+v %+v ok=%t", selected, release, asset, ok)
	}
}

func TestSupportedLegacyVersionsCanSelectV142CompatibilityData(t *testing.T) {
	idx := compatibility.Index{Schema: 1, Protocols: []compatibility.ProtocolGeneration{{ID: 41}, {ID: 42}, {ID: 43}}, Releases: []compatibility.Release{{BridgeVersion: "V1.42", Tag: "V1.42", Channel: "stable", Protocols: []int{41, 42, 43}, ModMin: "V1.3", ModMax: "V1.42"}}}
	for _, target := range []compatibility.Target{{ModVersion: "V1.3", Protocol: 41}, {ModVersion: "V1.4", Protocol: 42}, {ModVersion: "V1.41", Protocol: 43}} {
		got := compatibility.CompatibleCandidates(idx, target)
		if len(got) != 1 || got[0].BridgeVersion != "V1.42" {
			t.Fatalf("supported legacy target %+v did not select V1.42: %+v", target, got)
		}
	}
	if got := compatibility.CompatibleCandidates(idx, compatibility.Target{ModVersion: "V1.2", Protocol: 42}); len(got) != 0 {
		t.Fatalf("V1.2 must be outside the supported migration floor: %+v", got)
	}
}

func TestUpdaterNeverDowngradesCurrentBridge(t *testing.T) {
	candidates := []compatibility.Release{{BridgeVersion: "V1.3"}, {BridgeVersion: "V1.4"}, {BridgeVersion: "V1.41"}, {BridgeVersion: "V1.42"}}
	got := newerReleaseCandidates(candidates, "V1.41")
	if len(got) != 1 || got[0].BridgeVersion != "V1.42" {
		t.Fatalf("downgrade/current candidates were not filtered: %+v", got)
	}
}

func TestUnifiedReleaseRequiresMatchingModAndBridgeAssets(t *testing.T) {
	release := githubRelease{
		TagName: "V1.42",
		Assets: []githubAsset{
			{Name: "Enhanced_PS5_DualSense_Haptics_V1.42.zip", BrowserDownloadURL: "https://example.invalid/mod.zip"},
			{Name: "Enhanced_PS5_DualSense_Haptics_Bridge_V1.42.zip", BrowserDownloadURL: "https://example.invalid/bridge.zip"},
		},
	}
	modAsset, bridgeAsset, ok := projectAssetsForRelease(release)
	if !ok || modAsset.Name == "" || bridgeAsset.Name == "" {
		t.Fatalf("complete V1.42 release was not accepted: mod=%+v bridge=%+v ok=%t", modAsset, bridgeAsset, ok)
	}
	release.Assets[0].Name = "Enhanced_PS5_DualSense_Haptics_V1.41.zip"
	if _, _, ok := projectAssetsForRelease(release); ok {
		t.Fatal("release with mismatched mod version must not be accepted")
	}
}

func TestUnifiedReleaseSelectionSkipsIncompleteAndChoosesNewest(t *testing.T) {
	releases := []githubRelease{
		{TagName: "V1.43", Assets: []githubAsset{{Name: "Enhanced_PS5_DualSense_Haptics_Bridge_V1.43.zip", BrowserDownloadURL: "https://example.invalid/v143bridge.zip"}}},
		{TagName: "V1.42", Assets: []githubAsset{
			{Name: "Enhanced_PS5_DualSense_Haptics_V1.42.zip", BrowserDownloadURL: "https://example.invalid/v142mod.zip"},
			{Name: "Enhanced_PS5_DualSense_Haptics_Bridge_V1.42.zip", BrowserDownloadURL: "https://example.invalid/v142bridge.zip"},
		}},
		{TagName: "V1.41", Assets: []githubAsset{
			{Name: "Enhanced_PS5_DualSense_Haptics_V1.41.zip", BrowserDownloadURL: "https://example.invalid/v141mod.zip"},
			{Name: "Enhanced_PS5_DualSense_Haptics_Bridge_V1.41.zip", BrowserDownloadURL: "https://example.invalid/v141bridge.zip"},
		}},
	}
	got, ok := selectUnifiedProjectRelease(releases, "V1.41")
	if !ok || got.Version != "V1.42" {
		t.Fatalf("expected newest complete release V1.42, got %+v ok=%t", got, ok)
	}
}

func TestVersionFromProjectAssetName(t *testing.T) {
	for name, want := range map[string]string{
		"Enhanced_PS5_DualSense_Haptics_V1.42.zip":        "V1.42",
		"Enhanced_PS5_DualSense_Haptics_Bridge_V1.42.zip": "V1.42",
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
	makeZip(good, "1.42.0", false)
	if err := verifyBeamNGModArchive(good, "V1.42"); err != nil {
		t.Fatalf("valid mod rejected: %v", err)
	}
	wrong := filepath.Join(dir, "wrong.zip")
	makeZip(wrong, "1.41.0", false)
	if err := verifyBeamNGModArchive(wrong, "V1.42"); err == nil {
		t.Fatal("wrong-version mod accepted")
	}
	bad := filepath.Join(dir, "bad.zip")
	makeZip(bad, "1.42.0", true)
	if err := verifyBeamNGModArchive(bad, "V1.42"); err == nil {
		t.Fatal("mod containing EXE accepted")
	}
}

func TestLegacyUpdaterCanLandDirectlyOnV142(t *testing.T) {
	idx := compatibility.Index{
		Schema:    1,
		Protocols: []compatibility.ProtocolGeneration{{ID: 41}, {ID: 42}, {ID: 43}},
		Releases:  []compatibility.Release{{BridgeVersion: "V1.42", Tag: "V1.42", Channel: "stable", Protocols: []int{41, 42, 43}, ModMin: "V1.3", ModMax: "V1.42", Asset: "Enhanced_PS5_DualSense_Haptics_Bridge_V1.42.zip"}},
	}
	for _, target := range []compatibility.Target{{ModVersion: "V1.3", Protocol: 41}, {ModVersion: "V1.4", Protocol: 42}, {ModVersion: "V1.41", Protocol: 43}} {
		candidates := compatibility.CompatibleCandidates(idx, target)
		if len(candidates) != 1 || candidates[0].BridgeVersion != "V1.42" {
			t.Fatalf("legacy target %+v cannot land directly on V1.42: %+v", target, candidates)
		}
	}
}

func TestV142SortsNewerThanSupportedLegacyVersions(t *testing.T) {
	for _, older := range []string{"V1.3", "V1.4", "V1.41"} {
		if compatibility.CompareVersions("V1.42", older) <= 0 {
			t.Fatalf("V1.42 must sort newer than %s", older)
		}
	}
}

func TestPublicV13StyleUpdaterSelectsV142Asset(t *testing.T) {
	idx := compatibility.Index{Schema: 1, Protocols: []compatibility.ProtocolGeneration{{ID: 41}, {ID: 42}, {ID: 43}}, Releases: []compatibility.Release{{BridgeVersion: "V1.42", Tag: "V1.42", Channel: "stable", Protocols: []int{41, 42, 43}, ModMin: "V1.3", ModMax: "V1.42", Asset: "Enhanced_PS5_DualSense_Haptics_Bridge_V1.42.zip"}}}
	releases := []githubRelease{{TagName: "V1.42", Assets: []githubAsset{{Name: "Enhanced_PS5_DualSense_Haptics_Bridge_V1.42.zip", BrowserDownloadURL: "https://example.invalid/v142.zip"}}}}
	candidates := compatibility.CompatibleCandidates(idx, compatibility.Target{ModVersion: "V1.3", Protocol: 41})
	selected, release, asset, ok := selectPublishedCandidate(candidates, releases)
	if !ok || selected.BridgeVersion != "V1.42" || release.TagName != "V1.42" || asset.Name != "Enhanced_PS5_DualSense_Haptics_Bridge_V1.42.zip" {
		t.Fatalf("V1.3-style migration did not select V1.42: selected=%+v release=%+v asset=%+v ok=%t", selected, release, asset, ok)
	}
}

func TestPublicV14StyleUpdaterSelectsV142Asset(t *testing.T) {
	idx := compatibility.Index{Schema: 1, Protocols: []compatibility.ProtocolGeneration{{ID: 41}, {ID: 42}, {ID: 43}}, Releases: []compatibility.Release{{BridgeVersion: "V1.42", Tag: "V1.42", Channel: "stable", Protocols: []int{41, 42, 43}, ModMin: "V1.3", ModMax: "V1.42", Asset: "Enhanced_PS5_DualSense_Haptics_Bridge_V1.42.zip"}}}
	releases := []githubRelease{{TagName: "V1.42", Assets: []githubAsset{{Name: "Enhanced_PS5_DualSense_Haptics_Bridge_V1.42.zip", BrowserDownloadURL: "https://example.invalid/v142.zip"}}}}
	candidates := compatibility.CompatibleCandidates(idx, compatibility.Target{ModVersion: "V1.4", Protocol: 42})
	candidates = newerReleaseCandidates(candidates, "V1.4")
	selected, release, asset, ok := selectPublishedCandidate(candidates, releases)
	if !ok || selected.BridgeVersion != "V1.42" || release.TagName != "V1.42" || asset.Name != "Enhanced_PS5_DualSense_Haptics_Bridge_V1.42.zip" {
		t.Fatalf("V1.4-style migration did not select V1.42: selected=%+v release=%+v asset=%+v ok=%t", selected, release, asset, ok)
	}
}

func TestCanonicalUnifiedReleaseVersion(t *testing.T) {
	for in, want := range map[string]string{"V1.42": "V1.42", "v1.42": "V1.42", "V1.42.0": "V1.42"} {
		if got := canonicalReleaseVersion(in); got != want {
			t.Fatalf("%s: got %q want %q", in, got, want)
		}
	}
	for _, bad := range []string{"V1", "V1.42.1", "V1.42-beta", "latest"} {
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
	got, err := stageModForManualInstall(source, "Enhanced_PS5_DualSense_Haptics_V1.42.zip", root)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(got) != "Enhanced_PS5_DualSense_Haptics_V1.42.zip" {
		t.Fatalf("unexpected staged filename: %s", got)
	}
	if _, err := os.Stat(got); err != nil {
		t.Fatalf("staged file missing: %v", err)
	}
}

func TestManualInstallGuideContainsOnlyUserInstallationSteps(t *testing.T) {
	legacy := manualInstallGuideText("V1.42", true)
	for _, want := range []string{"ONE-TIME MIGRATION", "Mod Manager", "Repository page", "Unsubscribe and remove", "Do NOT extract", "make sure the mod is enabled"} {
		if !strings.Contains(legacy, want) {
			t.Fatalf("legacy tutorial missing %q: %s", want, legacy)
		}
	}
	for _, forbidden := range []string{"What this updater will do", "What it will NOT do", "downloads and verifies", "Bridge-only updater", "does not search"} {
		if strings.Contains(legacy, forbidden) {
			t.Fatalf("legacy tutorial contains internal updater explanation %q: %s", forbidden, legacy)
		}
	}
	normal := manualInstallGuideText("V1.42", false)
	if !strings.Contains(normal, "BEAMNG MOD INSTALLATION") || strings.Contains(normal, "ONE-TIME MIGRATION") {
		t.Fatalf("normal tutorial is not distinct from legacy tutorial: %s", normal)
	}
}

func TestVerifyBridgeRuntimePackageDoesNotRequireProjectReleaseMetadata(t *testing.T) {
	root := t.TempDir()
	required := []string{"START_BRIDGE.exe", "START_BRIDGE_AND_BEAMNG.exe", unifiedUpdaterName, legacyUpdaterAlias, filepath.Join("Bridge", "EnhancedPS5DualSenseHapticsUSB.exe"), filepath.Join("Bridge", "EnhancedPS5DualSenseHapticsBluetooth.exe")}
	manifest := make(map[string]string, len(required))
	for _, rel := range required {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(rel), 0o644); err != nil {
			t.Fatal(err)
		}
		hash, err := fileSHA256(path)
		if err != nil {
			t.Fatal(err)
		}
		manifest[normalizeRelative(rel)] = hash
	}
	if err := writeManifest(filepath.Join(root, manifestName), manifest); err != nil {
		t.Fatal(err)
	}
	if fileExists(filepath.Join(root, "PROJECT_RELEASE.json")) {
		t.Fatal("PROJECT_RELEASE.json must not exist in this test package")
	}
	if err := verifyBridgeRuntimePackage(root, "V1.42"); err != nil {
		t.Fatalf("valid Bridge package without PROJECT_RELEASE.json was rejected: %v", err)
	}
}
