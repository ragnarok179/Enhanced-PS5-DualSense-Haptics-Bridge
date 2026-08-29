package main

import (
	"archive/zip"
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Ragnarok179/enhanced-ps5-dualsense-haptics-bridge/internal/buildinfo"
	"github.com/Ragnarok179/enhanced-ps5-dualsense-haptics-bridge/internal/compatibility"
)

const (
	unifiedUpdaterName = "UPDATE_DUALSENSE.exe"
	legacyUpdaterAlias = "UPDATE_BRIDGE.exe"
	modMarkerPath      = "lua/dualsensePhysicsHaptics/config.lua"
	projectName        = "Enhanced PS5 DualSense Haptics"
)

type projectRelease struct {
	Release     githubRelease
	Version     string
	ModAsset    githubAsset
	BridgeAsset githubAsset
}

func runWorker(installRoot string, o updaterOptions) int {
	abs, err := filepath.Abs(strings.TrimSpace(strings.Trim(installRoot, `"`)))
	if err != nil || abs == "" {
		fmt.Fprintln(os.Stderr, "[ERROR] The installation folder path is invalid.")
		return 1
	}
	installRoot = filepath.Clean(abs)

	fmt.Println(projectName, buildinfo.DisplayVersion, "- Updater")
	fmt.Println("The updater updates the Windows Bridge and downloads the matching BeamNG mod ZIP.")
	fmt.Println("It never searches for or modifies your BeamNG user folder.")
	fmt.Println()

	if directoryExists(filepath.Join(installRoot, ".git")) {
		fmt.Println("[ERROR] This folder is a Git working copy. Run the updater from the extracted Bridge runtime folder.")
		return 4
	}
	if o.WaitPID > 0 {
		fmt.Println("[UPDATE] Waiting for the previous Bridge process to close...")
		waitForPID(o.WaitPID, 12*time.Second)
	}
	if !waitForBridgeProcesses(12 * time.Second) {
		fmt.Println("[ERROR] The Bridge is still running. Close it and try again.")
		return 3
	}

	releases, err := fetchPublishedReleases()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Unable to check GitHub Releases: %v\n", err)
		return 1
	}
	target, ok := selectUnifiedProjectRelease(releases, buildinfo.DisplayVersion)
	if !ok {
		fmt.Println("[INFO] No complete stable project release is currently available.")
		return 0
	}

	bridgeCurrent := compatibility.CompareVersions(buildinfo.DisplayVersion, target.Version) == 0
	legacyMigration := bridgeCurrent
	if pending, ok := readPendingCompatibilityRequest(installRoot); ok && strings.TrimSpace(pending.ModVersion) != "" {
		legacyMigration = compatibility.CompareVersions(pending.ModVersion, target.Version) < 0
	}
	if compatibility.CompareVersions(target.Version, buildinfo.DisplayVersion) < 0 {
		fmt.Printf("[INFO] Installed Bridge %s is newer than the latest complete release %s; no downgrade will be performed.\n", buildinfo.DisplayVersion, target.Version)
		return 0
	}

	fmt.Printf("Installed Bridge: %s\n", buildinfo.DisplayVersion)
	fmt.Printf("Latest complete release: %s\n", target.Version)
	fmt.Println()
	printPreUpdateGuide(target, legacyMigration)
	fmt.Print("Continue? [Y/N]: ")
	answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	answer = strings.ToLower(strings.TrimSpace(answer))
	if answer != "y" && answer != "yes" {
		fmt.Println("[UPDATE] Update cancelled.")
		return 0
	}

	tempRoot, err := os.MkdirTemp("", "EnhancedDualSenseUnifiedUpdate_")
	if err != nil {
		fmt.Fprintln(os.Stderr, "[ERROR]", err)
		return 1
	}
	defer os.RemoveAll(tempRoot)
	modZip := filepath.Join(tempRoot, target.ModAsset.Name)
	bridgeZip := filepath.Join(tempRoot, target.BridgeAsset.Name)
	bridgeExtract := filepath.Join(tempRoot, "bridge")
	bridgeBackup := filepath.Join(tempRoot, "bridge_backup")
	_ = os.MkdirAll(bridgeExtract, 0o755)
	_ = os.MkdirAll(bridgeBackup, 0o755)

	fmt.Printf("[UPDATE] Downloading %s...\n", target.ModAsset.Name)
	if err := downloadFile(target.ModAsset.BrowserDownloadURL, modZip); err != nil {
		fmt.Fprintln(os.Stderr, "[ERROR] Mod download failed:", err)
		return 1
	}
	fmt.Printf("[UPDATE] Downloading %s...\n", target.BridgeAsset.Name)
	if err := downloadFile(target.BridgeAsset.BrowserDownloadURL, bridgeZip); err != nil {
		fmt.Fprintln(os.Stderr, "[ERROR] Bridge download failed:", err)
		return 1
	}
	if err := verifyGitHubAssetDigest(target.ModAsset, modZip); err != nil {
		fmt.Fprintln(os.Stderr, "[ERROR] Mod asset verification failed:", err)
		return 1
	}
	if err := verifyGitHubAssetDigest(target.BridgeAsset, bridgeZip); err != nil {
		fmt.Fprintln(os.Stderr, "[ERROR] Bridge asset verification failed:", err)
		return 1
	}
	if err := verifyBeamNGModArchive(modZip, target.Version); err != nil {
		fmt.Fprintln(os.Stderr, "[ERROR] Downloaded BeamNG mod is invalid:", err)
		return 1
	}
	if err := extractZip(bridgeZip, bridgeExtract); err != nil {
		fmt.Fprintln(os.Stderr, "[ERROR] Bridge extraction failed:", err)
		return 1
	}
	manifestPath, err := findFile(bridgeExtract, manifestName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Downloaded Bridge package does not contain %s.\n", manifestName)
		return 1
	}
	remoteRoot := filepath.Dir(manifestPath)
	if err := verifyBridgeRuntimePackage(remoteRoot, target.Version); err != nil {
		fmt.Fprintln(os.Stderr, "[ERROR] Downloaded Bridge package is invalid:", err)
		return 1
	}
	remoteManifest, err := readManifest(manifestPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "[ERROR] Unable to read downloaded Bridge manifest:", err)
		return 1
	}
	if err := verifyPackage(remoteRoot, remoteManifest); err != nil {
		fmt.Fprintln(os.Stderr, "[ERROR] Downloaded Bridge verification failed:", err)
		return 1
	}
	fmt.Println("[OK] Both release packages verified.")

	manualModPath, err := stageModForManualInstall(modZip, target.ModAsset.Name, installRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "[ERROR] Unable to save the BeamNG mod ZIP for manual installation:", err)
		return 1
	}
	if err := verifyBeamNGModArchive(manualModPath, target.Version); err != nil {
		fmt.Fprintln(os.Stderr, "[ERROR] Saved BeamNG mod verification failed:", err)
		return 1
	}

	if !bridgeCurrent {
		localManifestPath := filepath.Join(installRoot, manifestName)
		localManifest := map[string]string{}
		if fileExists(localManifestPath) {
			localManifest, err = readManifest(localManifestPath)
			if err != nil {
				fmt.Fprintln(os.Stderr, "[ERROR] Unable to read installed Bridge manifest:", err)
				return 1
			}
		}
		managedRemote := withoutCompatibilityFiles(remoteManifest)
		newFiles, changed, removed, err := compareInstallation(installRoot, managedRemote, localManifest)
		if err != nil {
			fmt.Fprintln(os.Stderr, "[ERROR] Unable to compare Bridge installation:", err)
			return 1
		}
		printChanges(newFiles, changed, removed)
		fmt.Printf("[UPDATE] Installing Bridge %s...\n", target.Version)
		if err := installUpdate(installRoot, remoteRoot, bridgeBackup, localManifestPath, managedRemote, changed, removed); err != nil {
			fmt.Fprintln(os.Stderr, "[ERROR] Bridge update failed:", err)
			fmt.Println("The BeamNG mod ZIP was downloaded, but do not install it until the Bridge update succeeds.")
			return 1
		}
		if err := verifyPackage(installRoot, managedRemote); err != nil {
			fmt.Fprintln(os.Stderr, "[ERROR] Installed Bridge verification failed:", err)
			return 1
		}
	}

	guidePath, guideErr := writeManualInstallGuide(manualModPath, target.Version, legacyMigration)
	if guideErr != nil {
		fmt.Fprintln(os.Stderr, "[WARNING] Unable to save the update instructions next to the BeamNG mod ZIP:", guideErr)
	}
	clearPendingCompatibilityRequest(installRoot)
	_ = revealDownloadedMod(manualModPath)
	if guidePath != "" {
		_ = openTextGuide(guidePath)
	}
	printFinalInstallGuide(manualModPath, target.Version, legacyMigration, guidePath)
	return 0
}

func printPreUpdateGuide(target projectRelease, legacyMigration bool) {
	fmt.Println("============================================================")
	if legacyMigration {
		fmt.Println("ONE-TIME MIGRATION GUIDE")
	} else {
		fmt.Println("UPDATE GUIDE")
	}
	fmt.Println("============================================================")
	fmt.Println("1. Close BeamNG.drive if it is open.")
	fmt.Printf("2. Press Y to continue with the %s update.\n", target.Version)
	fmt.Println("3. When the Downloads folder and installation instructions open, follow the BeamNG steps shown there.")
	fmt.Println()
}

func printFinalInstallGuide(modPath, version string, legacyMigration bool, guidePath string) {
	fmt.Println()
	fmt.Println("============================================================")
	if legacyMigration {
		fmt.Println("ONE-TIME MIGRATION - BEAMNG INSTALLATION")
	} else {
		fmt.Println("BEAMNG MOD INSTALLATION")
	}
	fmt.Println("============================================================")
	fmt.Println("If your old mod came from the BeamNG Repository:")
	fmt.Println("1. Open BeamNG.drive.")
	fmt.Println("2. Open Mod Manager.")
	fmt.Println("3. Select Enhanced PS5 DualSense Haptics.")
	fmt.Println("4. Open its Repository page from the mod entry.")
	fmt.Println("5. Unsubscribe and remove the old Repository version.")
	fmt.Println("6. Close BeamNG.drive.")
	fmt.Printf("7. Move %s into YOUR BeamNG mods folder.\n", filepath.Base(modPath))
	fmt.Println("8. Do NOT extract the ZIP.")
	fmt.Println("9. Start BeamNG.drive and make sure the mod is enabled.")
	fmt.Println()
	fmt.Println("If your old mod was installed manually:")
	fmt.Println("1. Close BeamNG.drive.")
	fmt.Println("2. Remove the old Enhanced PS5 DualSense Haptics ZIP.")
	fmt.Printf("3. Move %s into YOUR BeamNG mods folder.\n", filepath.Base(modPath))
	fmt.Println("4. Do NOT extract the ZIP.")
	fmt.Println("5. Start BeamNG.drive and make sure the mod is enabled.")
	if guidePath != "" {
		fmt.Println()
		fmt.Printf("Instructions: %s\n", guidePath)
	}
	_ = version
}

func manualInstallGuideText(version string, legacyMigration bool) string {
	var b strings.Builder
	b.WriteString(projectName + " " + version + "\r\n")
	if legacyMigration {
		b.WriteString("ONE-TIME MIGRATION - BEAMNG INSTALLATION\r\n")
	} else {
		b.WriteString("BEAMNG MOD INSTALLATION\r\n")
	}
	b.WriteString("\r\nIF YOUR OLD MOD CAME FROM THE BEAMNG REPOSITORY:\r\n")
	b.WriteString("1. Open BeamNG.drive.\r\n")
	b.WriteString("2. Open Mod Manager.\r\n")
	b.WriteString("3. Select Enhanced PS5 DualSense Haptics.\r\n")
	b.WriteString("4. Open its Repository page from the mod entry.\r\n")
	b.WriteString("5. Unsubscribe and remove the old Repository version.\r\n")
	b.WriteString("6. Close BeamNG.drive.\r\n")
	b.WriteString("7. Move Enhanced_PS5_DualSense_Haptics_" + version + ".zip into YOUR BeamNG mods folder.\r\n")
	b.WriteString("8. Do NOT extract the ZIP.\r\n")
	b.WriteString("9. Start BeamNG.drive and make sure the mod is enabled.\r\n")
	b.WriteString("\r\nIF YOUR OLD MOD WAS INSTALLED MANUALLY:\r\n")
	b.WriteString("1. Close BeamNG.drive.\r\n")
	b.WriteString("2. Remove the old Enhanced PS5 DualSense Haptics ZIP.\r\n")
	b.WriteString("3. Move Enhanced_PS5_DualSense_Haptics_" + version + ".zip into YOUR BeamNG mods folder.\r\n")
	b.WriteString("4. Do NOT extract the ZIP.\r\n")
	b.WriteString("5. Start BeamNG.drive and make sure the mod is enabled.\r\n")
	return b.String()
}

func writeManualInstallGuide(modPath, version string, legacyMigration bool) (string, error) {
	dir := filepath.Dir(modPath)
	guide := filepath.Join(dir, "Enhanced_PS5_DualSense_Haptics_"+version+"_INSTALL_INSTRUCTIONS.txt")
	if err := os.WriteFile(guide, []byte(manualInstallGuideText(version, legacyMigration)), 0o644); err != nil {
		return "", err
	}
	return guide, nil
}

func openTextGuide(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	return exec.Command("notepad.exe", path).Start()
}

func preferredManualModDirectory(installRoot string) string {
	home, _ := os.UserHomeDir()
	for _, candidate := range []string{
		filepath.Join(home, "Downloads"),
		filepath.Join(home, "Desktop"),
		installRoot,
	} {
		candidate = filepath.Clean(candidate)
		if candidate != "." && directoryExists(candidate) {
			return candidate
		}
	}
	return installRoot
}

func stageModForManualInstall(source, assetName, installRoot string) (string, error) {
	destination := filepath.Join(preferredManualModDirectory(installRoot), filepath.Base(assetName))
	if err := copyFile(source, destination); err != nil {
		return "", err
	}
	return destination, nil
}

func revealDownloadedMod(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	cmd := exec.Command("explorer.exe", "/select,"+path)
	return cmd.Start()
}

func selectUnifiedProjectRelease(releases []githubRelease, current string) (projectRelease, bool) {
	candidates := make([]projectRelease, 0)
	for _, release := range releases {
		if release.Draft || release.Prerelease || strings.TrimSpace(release.TagName) == "" {
			continue
		}
		modAsset, bridgeAsset, ok := projectAssetsForRelease(release)
		if !ok {
			continue
		}
		version := canonicalReleaseVersion(release.TagName)
		if version == "" || compatibility.CompareVersions(version, current) < 0 {
			continue
		}
		candidates = append(candidates, projectRelease{Release: release, Version: version, ModAsset: modAsset, BridgeAsset: bridgeAsset})
	}
	if len(candidates) == 0 {
		return projectRelease{}, false
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return compatibility.CompareVersions(candidates[i].Version, candidates[j].Version) > 0
	})
	return candidates[0], true
}

func projectAssetsForRelease(release githubRelease) (githubAsset, githubAsset, bool) {
	version := canonicalReleaseVersion(release.TagName)
	if version == "" {
		return githubAsset{}, githubAsset{}, false
	}
	var modAsset, bridgeAsset githubAsset
	for _, asset := range release.Assets {
		name := strings.TrimSpace(asset.Name)
		if strings.TrimSpace(asset.BrowserDownloadURL) == "" || !strings.HasSuffix(strings.ToLower(name), ".zip") {
			continue
		}
		assetVersion := versionFromProjectAssetName(name)
		if assetVersion == "" || compatibility.CompareVersions(assetVersion, version) != 0 {
			continue
		}
		if strings.HasPrefix(strings.ToLower(name), strings.ToLower("Enhanced_PS5_DualSense_Haptics_Bridge_")) {
			if bridgeAsset.Name != "" {
				return githubAsset{}, githubAsset{}, false
			}
			bridgeAsset = asset
		} else if strings.HasPrefix(strings.ToLower(name), strings.ToLower("Enhanced_PS5_DualSense_Haptics_")) {
			if modAsset.Name != "" {
				return githubAsset{}, githubAsset{}, false
			}
			modAsset = asset
		}
	}
	return modAsset, bridgeAsset, modAsset.Name != "" && bridgeAsset.Name != ""
}

func canonicalReleaseVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	v = strings.TrimPrefix(strings.TrimPrefix(v, "V"), "v")
	parts := strings.Split(v, ".")
	if len(parts) == 3 && parts[2] == "0" {
		parts = parts[:2]
	}
	if len(parts) != 2 {
		return ""
	}
	for _, part := range parts {
		if part == "" {
			return ""
		}
		for _, r := range part {
			if r < '0' || r > '9' {
				return ""
			}
		}
	}
	return "V" + parts[0] + "." + parts[1]
}

func versionFromProjectAssetName(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	if !strings.HasSuffix(lower, ".zip") {
		return ""
	}
	base := strings.TrimSuffix(strings.TrimSpace(name), filepath.Ext(name))
	prefixes := []string{"Enhanced_PS5_DualSense_Haptics_Bridge_", "Enhanced_PS5_DualSense_Haptics_"}
	for _, prefix := range prefixes {
		if len(base) > len(prefix) && strings.EqualFold(base[:len(prefix)], prefix) {
			return canonicalReleaseVersion(base[len(prefix):])
		}
	}
	return ""
}

func fallbackVersion(v string) string {
	if strings.TrimSpace(v) == "" {
		return "unknown"
	}
	return strings.TrimSpace(v)
}

func verifyGitHubAssetDigest(asset githubAsset, path string) error {
	digest := strings.TrimSpace(asset.Digest)
	if digest == "" {
		return nil
	}
	parts := strings.SplitN(digest, ":", 2)
	if len(parts) != 2 || !strings.EqualFold(strings.TrimSpace(parts[0]), "sha256") {
		return nil
	}
	got, err := fileSHA256(path)
	if err != nil {
		return err
	}
	if !strings.EqualFold(got, strings.TrimSpace(parts[1])) {
		return fmt.Errorf("SHA-256 mismatch for %s", asset.Name)
	}
	return nil
}

func verifyBeamNGModArchive(path, targetVersion string) error {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer archive.Close()
	markerFound := false
	infoFound := false
	for _, entry := range archive.File {
		rel := normalizeZipEntry(entry.Name)
		if rel == normalizeZipEntry(modMarkerPath) {
			markerFound = true
		}
		if strings.HasPrefix(strings.ToLower(rel), "mod_info/") && strings.HasSuffix(strings.ToLower(rel), "/info.json") {
			r, err := entry.Open()
			if err != nil {
				return err
			}
			var info struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			}
			err = json.NewDecoder(r).Decode(&info)
			_ = r.Close()
			if err != nil {
				continue
			}
			if strings.Contains(strings.ToLower(info.Name), "dualsense") && strings.Contains(strings.ToLower(info.Name), "haptics") {
				infoFound = true
				if compatibility.CompareVersions(info.Version, targetVersion) != 0 {
					return fmt.Errorf("mod metadata version %q does not match release %s", info.Version, targetVersion)
				}
			}
		}
		if strings.HasSuffix(strings.ToLower(rel), ".exe") {
			return fmt.Errorf("BeamNG mod archive unexpectedly contains executable %q", entry.Name)
		}
	}
	if !markerFound {
		return fmt.Errorf("missing %s", modMarkerPath)
	}
	if !infoFound {
		return errors.New("DualSense mod metadata was not found")
	}
	return nil
}

func normalizeZipEntry(name string) string {
	return strings.TrimPrefix(strings.ReplaceAll(filepath.ToSlash(name), "\\", "/"), "./")
}

func verifyBridgeRuntimePackage(root, targetVersion string) error {
	required := []string{
		"START_BRIDGE.exe",
		"START_BRIDGE_AND_BEAMNG.exe",
		unifiedUpdaterName,
		legacyUpdaterAlias,
		filepath.Join("Bridge", "EnhancedPS5DualSenseHapticsUSB.exe"),
		filepath.Join("Bridge", "EnhancedPS5DualSenseHapticsBluetooth.exe"),
		manifestName,
	}
	for _, rel := range required {
		if !fileExists(filepath.Join(root, rel)) {
			return fmt.Errorf("missing required Bridge file %s", rel)
		}
	}
	manifest, err := readManifest(filepath.Join(root, manifestName))
	if err != nil {
		return err
	}
	if err := verifyPackage(root, manifest); err != nil {
		return err
	}
	if compatibility.CompareVersions(targetVersion, buildinfo.DisplayVersion) < 0 {
		return fmt.Errorf("refusing Bridge downgrade from %s to %s", buildinfo.DisplayVersion, targetVersion)
	}
	return nil
}
