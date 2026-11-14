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
	"strconv"
	"strings"
	"time"

	"github.com/Ragnarok179/enhanced-ps5-dualsense-haptics-bridge/internal/compatibility"
)

const (
	repoOwner    = "ragnarok179"
	repoName     = "Enhanced-PS5-DualSense-Haptics-Bridge"
	manifestName = "SHA256SUMS.txt"
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
	Digest             string `json:"digest"`
}
type updaterOptions struct {
	WaitPID int
}

var manifestLine = regexp.MustCompile(`^([0-9a-fA-F]{64})\s+\./(.+)$`)

func main() {
	if len(os.Args) >= 3 && os.Args[1] == "--worker" {
		opts, err := parseUpdaterOptions(os.Args[3:])
		if err != nil {
			fmt.Fprintln(os.Stderr, "[ERROR]", err)
			os.Exit(2)
		}
		code := runWorker(os.Args[2], opts)
		fmt.Println()
		fmt.Print("Press Enter to close...")
		_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
		cleanupWorkerLater()
		os.Exit(code)
	}
	opts, err := parseUpdaterOptions(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "[ERROR]", err)
		os.Exit(2)
	}
	code := launchWorker(opts)
	if code != 0 {
		fmt.Println()
		fmt.Print("Press Enter to close...")
		_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
	}
	os.Exit(code)
}

func parseUpdaterOptions(args []string) (updaterOptions, error) {
	var opts updaterOptions
	for i := 0; i < len(args); i++ {
		if args[i] != "--wait-pid" {
			return opts, fmt.Errorf("unknown updater option %q", args[i])
		}
		if i+1 >= len(args) {
			return opts, errors.New("--wait-pid requires a value")
		}
		i++
		pid, err := strconv.Atoi(args[i])
		if err != nil || pid <= 0 {
			return opts, fmt.Errorf("invalid PID %q", args[i])
		}
		opts.WaitPID = pid
	}
	return opts, nil
}

func serializeUpdaterOptions(opts updaterOptions) []string {
	if opts.WaitPID > 0 {
		return []string{"--wait-pid", strconv.Itoa(opts.WaitPID)}
	}
	return nil
}

func launchWorker(o updaterOptions) int {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Unable to locate the updater executable: %v\n", err)
		return 2
	}
	root := filepath.Dir(exe)
	temp, err := os.MkdirTemp("", "EnhancedDualSenseUpdater_")
	if err != nil {
		fmt.Fprintln(os.Stderr, "[ERROR]", err)
		return 2
	}
	worker := filepath.Join(temp, "UPDATE_DUALSENSE_worker.exe")
	if err := copyFile(exe, worker); err != nil {
		_ = os.RemoveAll(temp)
		fmt.Fprintln(os.Stderr, "[ERROR]", err)
		return 2
	}
	args := []string{"--worker", root}
	args = append(args, serializeUpdaterOptions(o)...)
	cmd := exec.Command(worker, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		_ = os.RemoveAll(temp)
		fmt.Fprintln(os.Stderr, "[ERROR]", err)
		return 2
	}
	return 0
}

func pendingCompatibilityFilePath(root string) string {
	return filepath.Join(root, "Config", compatibility.PendingFileName)
}
func clearPendingCompatibilityRequest(root string) { _ = os.Remove(pendingCompatibilityFilePath(root)) }
func releasesAPIURL() string {
	return fmt.Sprintf("https://api.github.com/repos/%s/%s/releases?per_page=100", repoOwner, repoName)
}
func fetchPublishedReleases() ([]githubRelease, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, e := http.NewRequest(http.MethodGet, releasesAPIURL(), nil)
	if e != nil {
		return nil, e
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "Enhanced-PS5-DualSense-Haptics-Updater")
	resp, e := client.Do(req)
	if e != nil {
		return nil, e
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GitHub returned HTTP %s", resp.Status)
	}
	var r []githubRelease
	if e = json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&r); e != nil {
		return nil, e
	}
	return r, nil
}
func waitForPID(pid int, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		running, e := pidRunning(pid)
		if e != nil || !running {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}
func pidRunning(pid int) (bool, error) {
	cmd := exec.Command("tasklist.exe", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH")
	out, e := cmd.CombinedOutput()
	if e != nil {
		return false, e
	}
	return strings.Contains(string(out), strconv.Itoa(pid)), nil
}
func waitForBridgeProcesses(timeout time.Duration) bool {
	names := []string{"EnhancedPS5DualSenseHapticsUSB.exe", "EnhancedPS5DualSenseHapticsBluetooth.exe", "START_BRIDGE.exe", "START_BRIDGE_AND_BEAMNG.exe"}
	deadline := time.Now().Add(timeout)
	for {
		any := false
		for _, n := range names {
			running, e := processRunning(n)
			if e == nil && running {
				any = true
				break
			}
		}
		if !any {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(150 * time.Millisecond)
	}
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

	// Write the local manifest last so an interrupted update never looks complete.
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
