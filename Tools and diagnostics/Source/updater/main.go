package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Ragnarok179/enhanced-ps5-dualsense-haptics-bridge/internal/compatibility"
)

const (
	repoOwner        = "ragnarok179"
	repoName         = "Enhanced-PS5-DualSense-Haptics-Bridge"
	manifestName     = "SHA256SUMS.txt"
	releaseAssetName = compatibility.DefaultReleaseAsset
)

type githubRelease struct {
	TagName    string        `json:"tag_name"`
	Name       string        `json:"name"`
	Draft      bool          `json:"draft"`
	Prerelease bool          `json:"prerelease"`
	Assets     []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type updaterOptions struct {
	CompatibilityUpdate bool
	ModVersion          string
	Protocol            int
	WaitPID             int
	Relaunch            bool
}

var manifestLine = regexp.MustCompile(`^([0-9a-fA-F]{64})\s+\./(.+)$`)

// These files stay in the GitHub repository only so the public V1.1 PowerShell
// updater can still recognize and migrate from the old layout. The modern EXE
// updater does not install or manage them.
var compatibilityOnlyFiles = map[string]struct{}{
	normalizeRelative(`START_BRIDGE.bat`):                                {},
	normalizeRelative(`UPDATE_BRIDGE.bat`):                               {},
	normalizeRelative(`Tools and diagnostics\Updater\Update-Bridge.ps1`): {},
}

func main() {
	if len(os.Args) >= 3 && os.Args[1] == "--worker" {
		installRoot := os.Args[2]
		options, err := parseUpdaterOptions(os.Args[3:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
			os.Exit(2)
		}
		code := runWorker(installRoot, options)
		if !options.CompatibilityUpdate {
			fmt.Println()
			fmt.Print("Press Enter to close...")
			_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
		}
		cleanupWorkerLater()
		os.Exit(code)
	}

	options, err := parseUpdaterOptions(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
		fmt.Println()
		fmt.Print("Press Enter to close...")
		_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
		os.Exit(2)
	}
	code := launchWorker(options)
	if code != 0 {
		fmt.Println()
		fmt.Print("Press Enter to close...")
		_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
	}
	os.Exit(code)
}

func parseUpdaterOptions(args []string) (updaterOptions, error) {
	var options updaterOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--compatibility-update":
			options.CompatibilityUpdate = true
		case "--mod-version":
			if i+1 >= len(args) {
				return updaterOptions{}, errors.New("--mod-version requires a value")
			}
			i++
			options.ModVersion = strings.TrimSpace(args[i])
		case "--protocol":
			if i+1 >= len(args) {
				return updaterOptions{}, errors.New("--protocol requires a value")
			}
			i++
			protocol, err := strconv.Atoi(args[i])
			if err != nil || protocol <= 0 {
				return updaterOptions{}, fmt.Errorf("invalid protocol %q", args[i])
			}
			options.Protocol = protocol
		case "--wait-pid":
			if i+1 >= len(args) {
				return updaterOptions{}, errors.New("--wait-pid requires a value")
			}
			i++
			pid, err := strconv.Atoi(args[i])
			if err != nil || pid <= 0 {
				return updaterOptions{}, fmt.Errorf("invalid PID %q", args[i])
			}
			options.WaitPID = pid
		case "--relaunch":
			options.Relaunch = true
		default:
			return updaterOptions{}, fmt.Errorf("unknown updater option %q", args[i])
		}
	}
	if options.CompatibilityUpdate && options.Protocol <= 0 {
		return updaterOptions{}, errors.New("compatibility update requires --protocol")
	}
	return options, nil
}

func launchWorker(options updaterOptions) int {
	executable, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Unable to locate UPDATE_BRIDGE.exe: %v\n", err)
		return 2
	}

	installRoot := filepath.Dir(executable)
	tempDir, err := os.MkdirTemp("", "EnhancedDualSenseUpdater_")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Unable to create the updater temporary folder: %v\n", err)
		return 2
	}

	workerPath := filepath.Join(tempDir, "UPDATE_BRIDGE_worker.exe")
	if err := copyFile(executable, workerPath); err != nil {
		_ = os.RemoveAll(tempDir)
		fmt.Fprintf(os.Stderr, "[ERROR] Unable to prepare the updater worker: %v\n", err)
		return 2
	}

	workerArgs := []string{"--worker", installRoot}
	workerArgs = append(workerArgs, serializeUpdaterOptions(options)...)
	cmd := exec.Command(workerPath, workerArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		_ = os.RemoveAll(tempDir)
		fmt.Fprintf(os.Stderr, "[ERROR] Unable to start the updater worker: %v\n", err)
		return 2
	}

	return 0
}

func serializeUpdaterOptions(options updaterOptions) []string {
	args := make([]string, 0, 9)
	if options.CompatibilityUpdate {
		args = append(args, "--compatibility-update")
	}
	if options.ModVersion != "" {
		args = append(args, "--mod-version", options.ModVersion)
	}
	if options.Protocol > 0 {
		args = append(args, "--protocol", strconv.Itoa(options.Protocol))
	}
	if options.WaitPID > 0 {
		args = append(args, "--wait-pid", strconv.Itoa(options.WaitPID))
	}
	if options.Relaunch {
		args = append(args, "--relaunch")
	}
	return args
}

func runWorker(installRoot string, options updaterOptions) int {
	absoluteRoot, err := filepath.Abs(strings.TrimSpace(strings.Trim(installRoot, `"`)))
	if err != nil || absoluteRoot == "" {
		fmt.Fprintln(os.Stderr, "[ERROR] The installation folder path is invalid.")
		return 1
	}
	installRoot = filepath.Clean(absoluteRoot)

	if !options.CompatibilityUpdate {
		if pending, ok := readPendingCompatibilityRequest(installRoot); ok {
			options.CompatibilityUpdate = true
			options.ModVersion = pending.ModVersion
			options.Protocol = pending.Protocol
			options.Relaunch = true
		}
	}

	if options.CompatibilityUpdate {
		fmt.Println("Enhanced PS5 DualSense Haptics - Compatibility Updater")
	} else {
		fmt.Println("Enhanced PS5 DualSense Haptics - Manual Updater")
	}
	fmt.Println()

	if directoryExists(filepath.Join(installRoot, ".git")) {
		fmt.Println("[ERROR] This folder is a Git working copy. Use GitHub Desktop or git pull instead of the public updater.")
		return 4
	}

	if options.WaitPID > 0 {
		fmt.Println("[UPDATE] Waiting for the incompatible Bridge process to close...")
		waitForPID(options.WaitPID, 12*time.Second)
	}
	if !waitForBridgeProcesses(installRoot, 12*time.Second) {
		fmt.Println("[ERROR] The Bridge is still running. Close it and try again.")
		return 3
	}

	tempRoot, err := os.MkdirTemp("", "EnhancedDualSenseUpdate_")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Unable to create a temporary folder: %v\n", err)
		return 1
	}
	defer os.RemoveAll(tempRoot)

	downloadZip := filepath.Join(tempRoot, "repository.zip")
	extractRoot := filepath.Join(tempRoot, "extract")
	backupRoot := filepath.Join(tempRoot, "backup")
	if err := os.MkdirAll(extractRoot, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Unable to prepare the extraction folder: %v\n", err)
		return 1
	}
	if err := os.MkdirAll(backupRoot, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Unable to prepare the backup folder: %v\n", err)
		return 1
	}

	if options.CompatibilityUpdate {
		modLabel := options.ModVersion
		if modLabel == "" {
			modLabel = "unknown"
		}
		fmt.Printf("[UPDATE] Looking for the newest stable Bridge compatible with BeamNG mod %s / protocol %d...\n", modLabel, options.Protocol)
	} else {
		fmt.Println("[UPDATE] Checking stable Bridge releases...")
	}

	releases, err := fetchPublishedReleases()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Unable to check GitHub Releases: %v\n", err)
		fmt.Println("Please try again later.")
		return 1
	}
	index, err := fetchCompatibilityIndex(releases)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Unable to read the released compatibility index: %v\n", err)
		fmt.Println("Please try again later.")
		return 1
	}

	var candidates []compatibility.Release
	target := compatibility.Target{ModVersion: options.ModVersion, Protocol: options.Protocol}
	if options.CompatibilityUpdate {
		candidates = compatibility.CompatibleCandidates(index, target)
	} else {
		candidates = compatibility.StableCandidates(index)
	}
	if len(candidates) == 0 {
		printNoCompatibleRelease(options)
		return 0
	}

	selectedMeta, release, asset, ok := selectPublishedCandidate(candidates, releases)
	if !ok {
		printNoCompatibleRelease(options)
		return 0
	}

	releaseLabel := strings.TrimSpace(release.TagName)
	if releaseLabel == "" {
		releaseLabel = strings.TrimSpace(release.Name)
	}
	fmt.Printf("[UPDATE] Selected stable release: %s\n", releaseLabel)

	fmt.Printf("[UPDATE] Downloading %s...\n", asset.Name)
	if err := downloadFile(asset.BrowserDownloadURL, downloadZip); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Download failed: %v\n", err)
		fmt.Println("Please try again later.")
		return 1
	}

	fmt.Println("[UPDATE] Extracting release package...")
	if err := extractZip(downloadZip, extractRoot); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Extraction failed: %v\n", err)
		return 1
	}

	remoteManifestPath, err := findFile(extractRoot, manifestName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] The downloaded repository does not contain %s. Update cancelled.\n", manifestName)
		return 1
	}
	remoteRoot := filepath.Dir(remoteManifestPath)
	if !fileExists(filepath.Join(remoteRoot, "START_BRIDGE.exe")) || !fileExists(filepath.Join(remoteRoot, "UPDATE_BRIDGE.exe")) {
		fmt.Fprintln(os.Stderr, "[ERROR] The downloaded repository layout is not recognized. Update cancelled.")
		return 1
	}

	remoteManifest, err := readManifest(remoteManifestPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
		return 1
	}

	fmt.Println("[UPDATE] Verifying downloaded files with SHA-256...")
	if err := verifyPackage(remoteRoot, remoteManifest); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
		return 1
	}

	// Verify the release's own signed-by-manifest compatibility metadata after
	// package integrity succeeds. This catches a stale/mislabelled GitHub release.
	packagedIndex, err := readCompatibilityIndex(filepath.Join(remoteRoot, compatibility.IndexFileName))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Downloaded package compatibility metadata is invalid: %v\n", err)
		return 1
	}
	packagedRelease, ok := compatibility.FindRelease(packagedIndex, selectedMeta.Tag)
	if !ok || !strings.EqualFold(strings.TrimSpace(packagedRelease.BridgeVersion), strings.TrimSpace(selectedMeta.BridgeVersion)) {
		fmt.Fprintln(os.Stderr, "[ERROR] Downloaded package does not declare the selected Bridge release. Update cancelled.")
		return 1
	}
	if options.CompatibilityUpdate && !packagedRelease.Supports(target) {
		fmt.Fprintln(os.Stderr, "[ERROR] Downloaded package does not support the detected BeamNG protocol. Update cancelled.")
		return 1
	}

	managedRemote := withoutCompatibilityFiles(remoteManifest)
	localManifestPath := filepath.Join(installRoot, manifestName)
	localManifest := map[string]string{}
	if fileExists(localManifestPath) {
		localManifest, err = readManifest(localManifestPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] Unable to read the installed checksum manifest: %v\n", err)
			return 1
		}
	}

	newFiles, changedFiles, removedFiles, err := compareInstallation(installRoot, managedRemote, localManifest)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Unable to compare installed files: %v\n", err)
		return 1
	}

	if len(newFiles)+len(changedFiles)+len(removedFiles) == 0 {
		if options.CompatibilityUpdate {
			fmt.Println("[ERROR] The selected compatible release is already installed, but the running Bridge rejected the mod protocol.")
			fmt.Println("Please try again later or report the compatibility metadata mismatch.")
			return 1
		}
		fmt.Printf("[OK] The Bridge files are already up to date with %s.\n", releaseLabel)
		return 0
	}

	printChanges(newFiles, changedFiles, removedFiles)
	fmt.Println()
	fmt.Println("Diagnostic logs and files not managed by SHA256SUMS.txt will not be deleted.")
	if !options.CompatibilityUpdate {
		fmt.Print("Install this update? [Y/N]: ")
		answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		answer = strings.ToLower(strings.TrimSpace(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("[UPDATE] Update cancelled by user.")
			return 0
		}
	} else {
		fmt.Println("[UPDATE] Compatibility update was explicitly approved from the Bridge; installing automatically.")
	}

	if err := installUpdate(installRoot, remoteRoot, backupRoot, localManifestPath, managedRemote, changedFiles, removedFiles); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
		return 1
	}

	fmt.Printf("[OK] Update to %s installed successfully.\n", releaseLabel)
	if options.CompatibilityUpdate {
		_ = os.Remove(pendingCompatibilityFilePath(installRoot))
	}
	if options.Relaunch {
		launcher := filepath.Join(installRoot, "START_BRIDGE.exe")
		fmt.Println("[UPDATE] Restarting the Bridge...")
		cmd := exec.Command(launcher)
		cmd.Dir = installRoot
		if err := cmd.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] Update succeeded but the Bridge could not be restarted: %v\n", err)
			fmt.Println("Start START_BRIDGE.exe manually.")
			return 1
		}
	}
	return 0
}

func pendingCompatibilityFilePath(installRoot string) string {
	return filepath.Join(installRoot, "Tools and diagnostics", "Config", compatibility.PendingFileName)
}

func readPendingCompatibilityRequest(installRoot string) (compatibility.PendingTarget, bool) {
	data, err := os.ReadFile(pendingCompatibilityFilePath(installRoot))
	if err != nil {
		return compatibility.PendingTarget{}, false
	}
	var pending compatibility.PendingTarget
	if json.Unmarshal(data, &pending) != nil || pending.Protocol <= 0 {
		return compatibility.PendingTarget{}, false
	}
	fmt.Printf("[UPDATE] Pending BeamNG compatibility request detected: mod %s / protocol %d.\n", fallbackModVersion(pending.ModVersion), pending.Protocol)
	return pending, true
}

func fallbackModVersion(version string) string {
	if strings.TrimSpace(version) == "" {
		return "unknown"
	}
	return strings.TrimSpace(version)
}

func printNoCompatibleRelease(options updaterOptions) {
	if options.CompatibilityUpdate {
		modLabel := options.ModVersion
		if modLabel == "" {
			modLabel = "unknown"
		}
		fmt.Printf("[INFO] No published Bridge release compatible with BeamNG mod %s / protocol %d is available yet.\n", modLabel, options.Protocol)
	} else {
		fmt.Println("[INFO] No installable stable Bridge release is available yet.")
	}
	fmt.Println("Please try again later.")
}

func releasesAPIURL() string {
	return fmt.Sprintf("https://api.github.com/repos/%s/%s/releases?per_page=100", repoOwner, repoName)
}

// selectCompatibilityIndexAsset returns the newest stable published release
// that carries BRIDGE_COMPATIBILITY.json as a release asset. The updater never
// reads compatibility metadata from the development branch.
func selectCompatibilityIndexAsset(releases []githubRelease) (githubRelease, githubAsset, bool) {
	var selected githubRelease
	var selectedAsset githubAsset
	found := false
	for _, release := range releases {
		if release.Draft || release.Prerelease || strings.TrimSpace(release.TagName) == "" {
			continue
		}
		asset, ok := findReleaseAsset(release, compatibility.IndexFileName)
		if !ok {
			continue
		}
		if !found || compatibility.CompareVersions(release.TagName, selected.TagName) > 0 {
			selected, selectedAsset, found = release, asset, true
		}
	}
	return selected, selectedAsset, found
}

func fetchCompatibilityIndex(releases []githubRelease) (compatibility.Index, error) {
	release, asset, ok := selectCompatibilityIndexAsset(releases)
	if !ok {
		return compatibility.Index{}, fmt.Errorf("no stable GitHub Release contains %s", compatibility.IndexFileName)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(http.MethodGet, asset.BrowserDownloadURL, nil)
	if err != nil {
		return compatibility.Index{}, err
	}
	req.Header.Set("User-Agent", "Enhanced-PS5-DualSense-Haptics-Updater")
	resp, err := client.Do(req)
	if err != nil {
		return compatibility.Index{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return compatibility.Index{}, fmt.Errorf("GitHub returned HTTP %s for %s in %s", resp.Status, compatibility.IndexFileName, release.TagName)
	}
	var index compatibility.Index
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&index); err != nil {
		return compatibility.Index{}, fmt.Errorf("invalid compatibility index from %s: %w", release.TagName, err)
	}
	if err := compatibility.Validate(index); err != nil {
		return compatibility.Index{}, err
	}
	return index, nil
}

func readCompatibilityIndex(path string) (compatibility.Index, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return compatibility.Index{}, err
	}
	var index compatibility.Index
	if err := json.Unmarshal(data, &index); err != nil {
		return compatibility.Index{}, err
	}
	if err := compatibility.Validate(index); err != nil {
		return compatibility.Index{}, err
	}
	return index, nil
}

func selectPublishedCandidate(candidates []compatibility.Release, releases []githubRelease) (compatibility.Release, githubRelease, githubAsset, bool) {
	published := make(map[string]githubRelease, len(releases))
	for _, release := range releases {
		if release.Draft || release.Prerelease || strings.TrimSpace(release.TagName) == "" {
			continue
		}
		published[strings.ToLower(strings.TrimSpace(release.TagName))] = release
	}
	for _, candidate := range candidates {
		release, ok := published[strings.ToLower(strings.TrimSpace(candidate.Tag))]
		if !ok {
			continue
		}
		asset, ok := findReleaseAsset(release, candidate.NormalizedAsset())
		if !ok {
			continue
		}
		return candidate, release, asset, true
	}
	return compatibility.Release{}, githubRelease{}, githubAsset{}, false
}

func fetchPublishedReleases() ([]githubRelease, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(http.MethodGet, releasesAPIURL(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "Enhanced-PS5-DualSense-Haptics-Updater")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GitHub returned HTTP %s while listing releases", resp.Status)
	}
	var releases []githubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&releases); err != nil {
		return nil, fmt.Errorf("invalid GitHub releases response: %w", err)
	}
	return releases, nil
}

func waitForPID(pid int, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		running, err := pidRunning(pid)
		if err != nil || !running {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func pidRunning(pid int) (bool, error) {
	cmd := exec.Command("tasklist.exe", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, err
	}
	return strings.Contains(string(output), strconv.Itoa(pid)), nil
}

func waitForBridgeProcesses(installRoot string, timeout time.Duration) bool {
	_ = installRoot
	names := []string{
		"EnhancedPS5DualSenseHapticsUSB.exe",
		"EnhancedPS5DualSenseHapticsBluetooth.exe",
		"START_BRIDGE.exe",
		"START_BRIDGE_AND_BEAMNG.exe",
	}
	deadline := time.Now().Add(timeout)
	for {
		anyRunning := false
		for _, process := range names {
			running, err := processRunning(process)
			if err == nil && running {
				anyRunning = true
				break
			}
		}
		if !anyRunning {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(150 * time.Millisecond)
	}
}

func withoutCompatibilityFiles(manifest map[string]string) map[string]string {
	filtered := make(map[string]string, len(manifest))
	for path, hash := range manifest {
		if _, compatibilityOnly := compatibilityOnlyFiles[normalizeRelative(path)]; compatibilityOnly {
			continue
		}
		filtered[path] = hash
	}
	return filtered
}

func compareInstallation(installRoot string, remote, local map[string]string) ([]string, []string, []string, error) {
	var newFiles, changedFiles, removedFiles []string
	removedSet := map[string]struct{}{}

	for relative, expectedHash := range remote {
		localPath := filepath.Join(installRoot, relative)
		if !fileExists(localPath) {
			newFiles = append(newFiles, relative)
			continue
		}
		actualHash, err := fileSHA256(localPath)
		if err != nil {
			return nil, nil, nil, err
		}
		if actualHash != expectedHash {
			changedFiles = append(changedFiles, relative)
		}
	}

	for relative := range local {
		if _, stillManaged := remote[relative]; stillManaged {
			continue
		}
		localPath := filepath.Join(installRoot, relative)
		if fileExists(localPath) {
			removedSet[relative] = struct{}{}
		}
	}

	// Remove repository-only migration scripts even if a local manifest from a
	// manually assembled package did not list them.
	for relative := range compatibilityOnlyFiles {
		if fileExists(filepath.Join(installRoot, relative)) {
			removedSet[relative] = struct{}{}
		}
	}
	for relative := range removedSet {
		removedFiles = append(removedFiles, relative)
	}

	sort.Strings(newFiles)
	sort.Strings(changedFiles)
	sort.Strings(removedFiles)
	return newFiles, changedFiles, removedFiles, nil
}

func installUpdate(installRoot, remoteRoot, backupRoot, localManifestPath string, remote map[string]string, changedFiles, removedFiles []string) error {
	toBackup := append(append([]string{}, changedFiles...), removedFiles...)
	for _, relative := range toBackup {
		source := filepath.Join(installRoot, relative)
		if fileExists(source) {
			if err := copyFile(source, filepath.Join(backupRoot, relative)); err != nil {
				return fmt.Errorf("unable to back up %q: %w", relative, err)
			}
		}
	}

	manifestBackup := filepath.Join(backupRoot, manifestName)
	manifestExisted := fileExists(localManifestPath)
	if manifestExisted {
		if err := copyFile(localManifestPath, manifestBackup); err != nil {
			return fmt.Errorf("unable to back up %s: %w", manifestName, err)
		}
	}

	created := make([]string, 0)
	rollback := func() {
		fmt.Println("[UPDATE] Restoring previous files...")
		for _, relative := range created {
			_ = os.Remove(filepath.Join(installRoot, relative))
		}
		for _, relative := range toBackup {
			backup := filepath.Join(backupRoot, relative)
			if fileExists(backup) {
				_ = copyFile(backup, filepath.Join(installRoot, relative))
			}
		}
		if manifestExisted && fileExists(manifestBackup) {
			_ = copyFile(manifestBackup, localManifestPath)
		} else if !manifestExisted {
			_ = os.Remove(localManifestPath)
		}
	}

	fmt.Println("[UPDATE] Installing new and changed files...")
	paths := sortedKeys(remote)
	for _, relative := range paths {
		source := filepath.Join(remoteRoot, relative)
		destination := filepath.Join(installRoot, relative)
		if !fileExists(destination) {
			created = append(created, relative)
		}
		if err := copyFile(source, destination); err != nil {
			rollback()
			return fmt.Errorf("update failed while installing %q: %w", relative, err)
		}
	}

	fmt.Println("[UPDATE] Removing obsolete managed files...")
	for _, relative := range removedFiles {
		if err := os.Remove(filepath.Join(installRoot, relative)); err != nil && !errors.Is(err, os.ErrNotExist) {
			rollback()
			return fmt.Errorf("update failed while removing %q: %w", relative, err)
		}
	}

	// Write the modern local manifest without the repository-only V1.1
	// compatibility scripts. This allows the EXE updater to remove those scripts
	// after a successful migration while the repository can keep them for old users.
	if err := writeManifest(localManifestPath, remote); err != nil {
		rollback()
		return fmt.Errorf("unable to write the installed checksum manifest: %w", err)
	}

	fmt.Println("[UPDATE] Verifying installed files...")
	if err := verifyPackage(installRoot, remote); err != nil {
		rollback()
		return fmt.Errorf("installed file verification failed: %w", err)
	}

	return nil
}

func printChanges(newFiles, changedFiles, removedFiles []string) {
	fmt.Println()
	fmt.Println("Update found:")
	fmt.Printf("  New files:     %d\n", len(newFiles))
	fmt.Printf("  Changed files: %d\n", len(changedFiles))
	fmt.Printf("  Removed files: %d\n", len(removedFiles))

	if len(newFiles) > 0 {
		fmt.Println("\nNew:")
		for _, file := range newFiles {
			fmt.Printf("  + %s\n", file)
		}
	}
	if len(changedFiles) > 0 {
		fmt.Println("\nChanged:")
		for _, file := range changedFiles {
			fmt.Printf("  * %s\n", file)
		}
	}
	if len(removedFiles) > 0 {
		fmt.Println("\nObsolete managed files:")
		for _, file := range removedFiles {
			fmt.Printf("  - %s\n", file)
		}
	}
}

func findReleaseAsset(release githubRelease, name string) (githubAsset, bool) {
	for _, asset := range release.Assets {
		if strings.EqualFold(strings.TrimSpace(asset.Name), strings.TrimSpace(name)) && strings.TrimSpace(asset.BrowserDownloadURL) != "" {
			return asset, true
		}
	}
	return githubAsset{}, false
}

func downloadFile(url, destination string) error {
	client := &http.Client{Timeout: 2 * time.Minute}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Enhanced-PS5-DualSense-Haptics-Updater")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GitHub returned HTTP %s", resp.Status)
	}

	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	out, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	return err
}

func extractZip(zipPath, destination string) error {
	archive, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer archive.Close()

	cleanDestination, err := filepath.Abs(destination)
	if err != nil {
		return err
	}
	prefix := cleanDestination + string(os.PathSeparator)

	for _, entry := range archive.File {
		target := filepath.Join(cleanDestination, filepath.FromSlash(entry.Name))
		cleanTarget, err := filepath.Abs(target)
		if err != nil {
			return err
		}
		if cleanTarget != cleanDestination && !strings.HasPrefix(cleanTarget, prefix) {
			return fmt.Errorf("unsafe ZIP entry %q", entry.Name)
		}

		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(cleanTarget, 0o755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(cleanTarget), 0o755); err != nil {
			return err
		}
		source, err := entry.Open()
		if err != nil {
			return err
		}
		destinationFile, err := os.Create(cleanTarget)
		if err != nil {
			source.Close()
			return err
		}
		_, copyErr := io.Copy(destinationFile, source)
		closeErr := destinationFile.Close()
		source.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func findFile(root, name string) (string, error) {
	var found string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && strings.EqualFold(entry.Name(), name) {
			found = path
			return io.EOF
		}
		return nil
	})
	if errors.Is(err, io.EOF) && found != "" {
		return found, nil
	}
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", os.ErrNotExist
	}
	return found, nil
}

func readManifest(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	manifest := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		match := manifestLine.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		relative := normalizeRelative(match[2])
		manifest[relative] = strings.ToLower(match[1])
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(manifest) == 0 {
		return nil, fmt.Errorf("checksum manifest is empty or invalid: %s", path)
	}
	return manifest, nil
}

func writeManifest(path string, manifest map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, relative := range sortedKeys(manifest) {
		slashPath := filepath.ToSlash(relative)
		if _, err := fmt.Fprintf(writer, "%s  ./%s\n", manifest[relative], slashPath); err != nil {
			return err
		}
	}
	return writer.Flush()
}

func verifyPackage(root string, manifest map[string]string) error {
	for relative, expected := range manifest {
		path := filepath.Join(root, relative)
		if !fileExists(path) {
			return fmt.Errorf("package verification failed: missing %q", relative)
		}
		actual, err := fileSHA256(path)
		if err != nil {
			return err
		}
		if actual != expected {
			return fmt.Errorf("package verification failed: SHA-256 mismatch for %q", relative)
		}
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	// SHA256SUMS.txt is generated from Git's canonical LF representation so a
	// Windows CRLF working tree and the release ZIP verify identically. Match the
	// generator exactly: files containing NUL are binary; other files normalize
	// CRLF and lone CR to LF before hashing.
	if bytes.IndexByte(data, 0) < 0 {
		write := 0
		for read := 0; read < len(data); read++ {
			if data[read] == '\r' {
				if read+1 < len(data) && data[read+1] == '\n' {
					read++
				}
				data[write] = '\n'
				write++
				continue
			}
			data[write] = data[read]
			write++
		}
		data = data[:write]
	}

	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func copyFile(source, destination string) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()

	output, err := os.Create(destination)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func processRunning(imageName string) (bool, error) {
	cmd := exec.Command("tasklist.exe", "/FI", "IMAGENAME eq "+imageName, "/NH")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, err
	}
	return strings.Contains(strings.ToLower(string(output)), strings.ToLower(imageName)), nil
}

func cleanupWorkerLater() {
	executable, err := os.Executable()
	if err != nil {
		return
	}
	tempDir := filepath.Dir(executable)
	if !strings.Contains(strings.ToLower(filepath.Base(tempDir)), strings.ToLower("EnhancedDualSenseUpdater_")) {
		return
	}
	command := fmt.Sprintf(`ping 127.0.0.1 -n 2 >nul & rmdir /s /q "%s"`, strings.ReplaceAll(tempDir, `"`, ""))
	_ = exec.Command("cmd.exe", "/C", command).Start()
}

func normalizeRelative(path string) string {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "./")
	path = strings.TrimPrefix(path, `.\`)
	path = strings.ReplaceAll(path, "/", string(os.PathSeparator))
	path = strings.ReplaceAll(path, `\`, string(os.PathSeparator))
	return filepath.Clean(path)
}

func sortedKeys(manifest map[string]string) []string {
	keys := make([]string, 0, len(manifest))
	for key := range manifest {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func directoryExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
