package main

import (
	"archive/zip"
	"bufio"
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
	"strings"
	"time"
)

const (
	repoOwner        = "ragnarok179"
	repoName         = "Enhanced-PS5-DualSense-Haptics-Bridge"
	manifestName     = "SHA256SUMS.txt"
	releaseAssetName = "Enhanced_PS5_DualSense_Haptics_Bridge.zip"
)

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Name    string        `json:"name"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
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
		code := runWorker(installRoot)
		fmt.Println()
		fmt.Print("Press Enter to close...")
		_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
		cleanupWorkerLater()
		os.Exit(code)
	}

	code := launchWorker()
	if code != 0 {
		fmt.Println()
		fmt.Print("Press Enter to close...")
		_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
	}
	os.Exit(code)
}

func launchWorker() int {
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

	cmd := exec.Command(workerPath, "--worker", installRoot)
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

func runWorker(installRoot string) int {
	fmt.Println("Enhanced PS5 DualSense Haptics - Manual Updater")
	fmt.Println()

	absoluteRoot, err := filepath.Abs(strings.TrimSpace(strings.Trim(installRoot, `"`)))
	if err != nil || absoluteRoot == "" {
		fmt.Fprintln(os.Stderr, "[ERROR] The installation folder path is invalid.")
		return 1
	}
	installRoot = filepath.Clean(absoluteRoot)

	if directoryExists(filepath.Join(installRoot, ".git")) {
		fmt.Println("[ERROR] This folder is a Git working copy. Use GitHub Desktop or git pull instead of the public updater.")
		return 4
	}

	for _, process := range []string{
		"EnhancedPS5DualSenseHapticsUSB.exe",
		"EnhancedPS5DualSenseHapticsBluetooth.exe",
		"START_BRIDGE.exe",
		"START_BRIDGE_AND_BEAMNG.exe",
	} {
		running, checkErr := processRunning(process)
		if checkErr == nil && running {
			fmt.Println("[ERROR] The Bridge is currently running. Close it before updating.")
			return 3
		}
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

	fmt.Println("[UPDATE] Checking the latest stable GitHub Release...")
	release, err := fetchLatestRelease()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Unable to check GitHub Releases: %v\n", err)
		return 1
	}

	releaseLabel := strings.TrimSpace(release.TagName)
	if releaseLabel == "" {
		releaseLabel = strings.TrimSpace(release.Name)
	}
	if releaseLabel == "" {
		releaseLabel = "latest release"
	}
	fmt.Printf("[UPDATE] Latest stable release: %s\n", releaseLabel)

	asset, ok := findReleaseAsset(release, releaseAssetName)
	if !ok {
		fmt.Printf("[INFO] %s does not contain the updater package %s.\n", releaseLabel, releaseAssetName)
		fmt.Println("[OK] No installable release update is available yet.")
		return 0
	}

	fmt.Printf("[UPDATE] Downloading %s...\n", asset.Name)
	if err := downloadFile(asset.BrowserDownloadURL, downloadZip); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Download failed: %v\n", err)
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
		fmt.Printf("[OK] The Bridge files are already up to date with %s.\n", releaseLabel)
		return 0
	}

	printChanges(newFiles, changedFiles, removedFiles)
	fmt.Println()
	fmt.Println("Diagnostic logs and files not managed by SHA256SUMS.txt will not be deleted.")
	fmt.Print("Install this update? [Y/N]: ")
	answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	answer = strings.ToLower(strings.TrimSpace(answer))
	if answer != "y" && answer != "yes" {
		fmt.Println("[UPDATE] Update cancelled by user.")
		return 0
	}

	if err := installUpdate(installRoot, remoteRoot, backupRoot, localManifestPath, managedRemote, changedFiles, removedFiles); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
		return 1
	}

	fmt.Printf("[OK] Update to %s installed successfully.\n", releaseLabel)
	return 0
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

func latestReleaseAPIURL() string {
	return fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", repoOwner, repoName)
}

func fetchLatestRelease() (githubRelease, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(http.MethodGet, latestReleaseAPIURL(), nil)
	if err != nil {
		return githubRelease{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "Enhanced-PS5-DualSense-Haptics-Updater")

	resp, err := client.Do(req)
	if err != nil {
		return githubRelease{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return githubRelease{}, fmt.Errorf("GitHub returned HTTP %s", resp.Status)
	}

	var release githubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&release); err != nil {
		return githubRelease{}, fmt.Errorf("invalid GitHub release response: %w", err)
	}
	if strings.TrimSpace(release.TagName) == "" && strings.TrimSpace(release.Name) == "" {
		return githubRelease{}, errors.New("GitHub returned a release without a name or tag")
	}
	return release, nil
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
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
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
