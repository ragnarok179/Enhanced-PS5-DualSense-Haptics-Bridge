package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testRelease(tag string, complete bool) githubRelease {
	assets := []githubAsset{{Name: "Enhanced_PS5_DualSense_Haptics_" + tag + ".zip", BrowserDownloadURL: "https://example.invalid/mod.zip"}}
	if complete {
		assets = append(assets, githubAsset{Name: "Enhanced_PS5_DualSense_Haptics_Bridge_" + tag + ".zip", BrowserDownloadURL: "https://example.invalid/bridge.zip"})
	}
	return githubRelease{TagName: tag, Assets: assets}
}

func TestV143UpdaterSelectsCompleteV150Release(t *testing.T) {
	releases := []githubRelease{testRelease("V1.43", true), testRelease("V1.50", true), testRelease("V1.51", false)}
	selected, ok := selectUnifiedProjectRelease(releases, "V1.43")
	if !ok || selected.Version != "V1.50" || selected.ModAsset.Name != "Enhanced_PS5_DualSense_Haptics_V1.50.zip" || selected.BridgeAsset.Name != "Enhanced_PS5_DualSense_Haptics_Bridge_V1.50.zip" {
		t.Fatalf("V1.43 update selection failed: %+v ok=%t", selected, ok)
	}
}

func TestV143AndV150UpdatersSelectV151StableRelease(t *testing.T) {
	releases := []githubRelease{
		testRelease("V1.43", true),
		testRelease("V1.50", true),
		testRelease("V1.51", true),
		testRelease("V1.52", false), // A newer release without both assets is never offered.
	}
	for _, installed := range []string{"V1.43", "V1.50"} {
		selected, ok := selectUnifiedProjectRelease(releases, installed)
		if !ok || selected.Version != "V1.51" ||
			selected.ModAsset.Name != "Enhanced_PS5_DualSense_Haptics_V1.51.zip" ||
			selected.BridgeAsset.Name != "Enhanced_PS5_DualSense_Haptics_Bridge_V1.51.zip" {
			t.Fatalf("update from %s did not select the complete V1.51 release: %+v ok=%t", installed, selected, ok)
		}
	}
}

func TestReleaseSelectionRejectsIncompletePreviewAndDowngrade(t *testing.T) {
	preview := testRelease("V1.52", true)
	preview.Prerelease = true
	releases := []githubRelease{testRelease("V1.43", true), testRelease("V1.44", false), preview}
	selected, ok := selectUnifiedProjectRelease(releases, "V1.50")
	if ok {
		t.Fatalf("release selection offered a downgrade or incomplete release: %+v", selected)
	}
}

func TestMatchingReleaseRequiresBothAssets(t *testing.T) {
	release := testRelease("V1.50", true)
	if _, _, ok := projectAssetsForRelease(release); !ok {
		t.Fatal("matching release pair rejected")
	}
	release.Assets[1].Name = "Enhanced_PS5_DualSense_Haptics_Bridge_V1.43.zip"
	if _, _, ok := projectAssetsForRelease(release); ok {
		t.Fatal("mismatched Bridge selected")
	}
	release = testRelease("V1.50", false)
	if _, _, ok := projectAssetsForRelease(release); ok {
		t.Fatal("mod-only release selected")
	}
}

func TestReleaseVersionIsStrict(t *testing.T) {
	for _, tc := range []struct{ in, want string }{{"V1.50", "V1.50"}, {"v1.50.0", "V1.50"}, {"V1.51.1", ""}, {"latest", ""}, {"V1.50-beta", ""}} {
		if got := canonicalReleaseVersion(tc.in); got != tc.want {
			t.Fatalf("%q: got %q want %q", tc.in, got, tc.want)
		}
	}
}

func TestUpdaterRejectsRemovedLegacyFlags(t *testing.T) {
	for _, args := range [][]string{{"--compatibility-update"}, {"--mod-version", "V1.3"}, {"--protocol", "41"}, {"--relaunch"}} {
		if _, err := parseUpdaterOptions(args); err == nil {
			t.Fatalf("retired option accepted: %v", args)
		}
	}
	opts, err := parseUpdaterOptions([]string{"--wait-pid", "1234"})
	if err != nil || opts.WaitPID != 1234 {
		t.Fatalf("modern wait-pid option broken: %+v %v", opts, err)
	}
}

func TestModArchiveRequiresVersionAndNoExecutables(t *testing.T) {
	mkzip := func(path, version string, executable bool) {
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		w := zip.NewWriter(f)
		files := map[string]string{"lua/dualsensePhysicsHaptics/config.lua": "return {}", "mod_info/EnhancedPS5DualSenseHaptics/info.json": `{"name":"Enhanced PS5 DualSense Haptics","version":"` + version + `"}`}
		if executable {
			files["bridge.exe"] = "bad"
		}
		for name, body := range files {
			entry, err := w.Create(name)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := entry.Write([]byte(body)); err != nil {
				t.Fatal(err)
			}
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
	dir := t.TempDir()
	valid := filepath.Join(dir, "valid.zip")
	mkzip(valid, "1.50.0", false)
	if err := verifyBeamNGModArchive(valid, "V1.50"); err != nil {
		t.Fatal(err)
	}
	wrong := filepath.Join(dir, "wrong.zip")
	mkzip(wrong, "1.43.0", false)
	if err := verifyBeamNGModArchive(wrong, "V1.50"); err == nil {
		t.Fatal("wrong version accepted")
	}
	executable := filepath.Join(dir, "executable.zip")
	mkzip(executable, "1.50.0", true)
	if err := verifyBeamNGModArchive(executable, "V1.50"); err == nil {
		t.Fatal("executable in BeamNG mod accepted")
	}
}

func TestUpdaterDoesNotNeedBeamNGInstallation(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "src.zip")
	if err := os.WriteFile(source, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	path, err := stageModForManualInstall(source, "Enhanced_PS5_DualSense_Haptics_V1.50.zip", root)
	if err != nil || filepath.Base(path) != "Enhanced_PS5_DualSense_Haptics_V1.50.zip" {
		t.Fatalf("staging failed: %q %v", path, err)
	}
	guide := manualInstallGuideText("V1.50")
	if !strings.Contains(guide, "BEAMNG MOD INSTALLATION") || strings.Contains(guide, "ONE-TIME") || strings.Contains(guide, "Repository page") {
		t.Fatalf("legacy guide survived: %s", guide)
	}
}

func TestBridgeRuntimeRequiresOnlyUnifiedUpdater(t *testing.T) {
	root := t.TempDir()
	required := []string{"START_BRIDGE.exe", "START_BRIDGE_AND_BEAMNG.exe", unifiedUpdaterName, filepath.Join("Bridge", "EnhancedPS5DualSenseHapticsUSB.exe"), filepath.Join("Bridge", "EnhancedPS5DualSenseHapticsBluetooth.exe")}
	manifest := make(map[string]string)
	for _, rel := range required {
		file := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(rel), 0644); err != nil {
			t.Fatal(err)
		}
		sum, err := fileSHA256(file)
		if err != nil {
			t.Fatal(err)
		}
		manifest[normalizeRelative(rel)] = sum
	}
	if err := writeManifest(filepath.Join(root, manifestName), manifest); err != nil {
		t.Fatal(err)
	}
	if err := verifyBridgeRuntimePackage(root, "V1.51"); err != nil {
		t.Fatalf("current Bridge rejected without obsolete updater: %v", err)
	}
}
