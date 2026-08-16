package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Ragnarok179/enhanced-ps5-dualsense-haptics-bridge/internal/compatibility"
)

func TestCompatibilityUpdaterOptionsRoundTrip(t *testing.T) {
	want := updaterOptions{
		CompatibilityUpdate: true,
		ModVersion:          "V1.6.0",
		Protocol:            42,
		WaitPID:             1234,
		Relaunch:            true,
	}
	args := serializeUpdaterOptions(want)
	got, err := parseUpdaterOptions(args)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round-trip mismatch: got %+v want %+v", got, want)
	}
}

func TestCompatibilityUpdaterRequiresProtocol(t *testing.T) {
	if _, err := parseUpdaterOptions([]string{"--compatibility-update", "--mod-version", "V1.6.0"}); err == nil {
		t.Fatal("expected compatibility update without protocol to fail")
	}
}

func TestUnknownUpdaterOptionRejected(t *testing.T) {
	if _, err := parseUpdaterOptions([]string{"--unexpected"}); err == nil {
		t.Fatal("expected unknown option to fail")
	}
}

func TestSelectPublishedCandidateUsesNewestCompatiblePublishedRelease(t *testing.T) {
	candidates := []compatibility.Release{
		{BridgeVersion: "V1.7.0", Tag: "V1.7.0", Channel: "stable", Protocols: []int{43}},
		{BridgeVersion: "V1.6.1", Tag: "V1.6.1", Channel: "stable", Protocols: []int{41, 42}},
		{BridgeVersion: "V1.5.4", Tag: "V1.5.4", Channel: "stable", Protocols: []int{40, 41}},
	}
	releases := []githubRelease{
		{TagName: "V1.7.0", Assets: []githubAsset{{Name: releaseAssetName, BrowserDownloadURL: "https://example.invalid/17.zip"}}},
		{TagName: "V1.5.4", Assets: []githubAsset{{Name: releaseAssetName, BrowserDownloadURL: "https://example.invalid/154.zip"}}},
		{TagName: "V1.6.1", Assets: []githubAsset{{Name: releaseAssetName, BrowserDownloadURL: "https://example.invalid/161.zip"}}},
	}
	selected, _, _, ok := selectPublishedCandidate(candidates[1:], releases)
	if !ok || selected.Tag != "V1.6.1" {
		t.Fatalf("expected V1.6.1, got %+v ok=%v", selected, ok)
	}
}

func TestSelectPublishedCandidateSkipsDraftPrereleaseAndMissingAsset(t *testing.T) {
	candidates := []compatibility.Release{
		{BridgeVersion: "V1.6.2", Tag: "V1.6.2", Channel: "stable", Protocols: []int{41}},
		{BridgeVersion: "V1.6.1", Tag: "V1.6.1", Channel: "stable", Protocols: []int{41}},
		{BridgeVersion: "V1.6.0", Tag: "V1.6.0", Channel: "stable", Protocols: []int{41}},
		{BridgeVersion: "V1.5.4", Tag: "V1.5.4", Channel: "stable", Protocols: []int{41}},
	}
	releases := []githubRelease{
		{TagName: "V1.6.2", Draft: true, Assets: []githubAsset{{Name: releaseAssetName, BrowserDownloadURL: "x"}}},
		{TagName: "V1.6.1", Prerelease: true, Assets: []githubAsset{{Name: releaseAssetName, BrowserDownloadURL: "x"}}},
		{TagName: "V1.6.0", Assets: []githubAsset{{Name: "source.zip", BrowserDownloadURL: "x"}}},
		{TagName: "V1.5.4", Assets: []githubAsset{{Name: releaseAssetName, BrowserDownloadURL: "ok"}}},
	}
	selected, _, _, ok := selectPublishedCandidate(candidates, releases)
	if !ok || selected.Tag != "V1.5.4" {
		t.Fatalf("expected V1.5.4 fallback, got %+v ok=%v", selected, ok)
	}
}

func TestFileSHA256NormalizesTextLineEndings(t *testing.T) {
	temp := t.TempDir()
	lf := filepath.Join(temp, "lf.txt")
	crlf := filepath.Join(temp, "crlf.txt")
	cr := filepath.Join(temp, "cr.txt")
	if err := os.WriteFile(lf, []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(crlf, []byte("one\r\ntwo\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cr, []byte("one\rtwo\r"), 0o644); err != nil {
		t.Fatal(err)
	}
	want, err := fileSHA256(lf)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{crlf, cr} {
		got, err := fileSHA256(path)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("text hash must be line-ending independent: %s != %s", got, want)
		}
	}
}

func TestFileSHA256DoesNotNormalizeBinary(t *testing.T) {
	temp := t.TempDir()
	path := filepath.Join(temp, "binary.bin")
	data := []byte{'A', '\r', '\n', 0, 'B'}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := fileSHA256(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	want := hex.EncodeToString(sum[:])
	if got != want {
		t.Fatalf("binary hash changed: got %s want %s", got, want)
	}
}

func TestSelectCompatibilityIndexAssetUsesNewestStableRelease(t *testing.T) {
	releases := []githubRelease{
		{TagName: "V1.5.4", Assets: []githubAsset{{Name: compatibility.IndexFileName, BrowserDownloadURL: "154.json"}}},
		{TagName: "V1.6.0", Prerelease: true, Assets: []githubAsset{{Name: compatibility.IndexFileName, BrowserDownloadURL: "160-pre.json"}}},
		{TagName: "V1.5.5", Assets: []githubAsset{{Name: compatibility.IndexFileName, BrowserDownloadURL: "155.json"}}},
	}
	release, asset, ok := selectCompatibilityIndexAsset(releases)
	if !ok || release.TagName != "V1.5.5" || asset.BrowserDownloadURL != "155.json" {
		t.Fatalf("unexpected compatibility-index asset selection: release=%+v asset=%+v ok=%v", release, asset, ok)
	}
}
