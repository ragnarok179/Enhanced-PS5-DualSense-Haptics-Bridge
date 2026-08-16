package compatibility

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const (
	IndexSchemaVersion   = 1
	IndexFileName        = "BRIDGE_COMPATIBILITY.json"
	PendingFileName      = "pending_bridge_compatibility.json"
	DefaultReleaseAsset  = "Enhanced_PS5_DualSense_Haptics_Bridge.zip"
	ProtocolID           = "DPH"
	CurrentBridgeVersion = "V1.3"
	CurrentProtocol      = 41
	LegacyProtocol       = 40
)

// Target describes the BeamNG mod generation that needs a compatible Bridge.
type Target struct {
	ModVersion string
	Protocol   int
}

// PendingTarget is written locally when the Bridge detects an incompatible mod.
// It lets a later manual UPDATE_BRIDGE.exe retry the exact same compatibility
// request without contacting GitHub during Bridge startup.
type PendingTarget struct {
	ModVersion string `json:"modVersion,omitempty"`
	Protocol   int    `json:"protocol"`
}

// Release is one stable Bridge build listed in BRIDGE_COMPATIBILITY.json.
// Protocols is intentionally explicit rather than inferred from Bridge version:
// a newer Bridge may support several generations of BeamNG mods at once.
type ProtocolGeneration struct {
	ID          int    `json:"id"`
	Status      string `json:"status,omitempty"`
	Introduced  string `json:"introduced,omitempty"`
	Description string `json:"description,omitempty"`
}

type Release struct {
	BridgeVersion string `json:"bridgeVersion"`
	Tag           string `json:"tag"`
	Channel       string `json:"channel"`
	Protocols     []int  `json:"protocols"`
	ModMin        string `json:"modMin,omitempty"`
	ModMax        string `json:"modMax,omitempty"`
	Asset         string `json:"asset,omitempty"`
}

type Index struct {
	Schema    int                  `json:"schema"`
	Policy    string               `json:"protocolPolicy,omitempty"`
	Protocols []ProtocolGeneration `json:"protocols"`
	Releases  []Release            `json:"releases"`
}

func SupportedProtocols() []int {
	return []int{LegacyProtocol, CurrentProtocol}
}

func ProtocolSupported(protocol int) bool {
	for _, supported := range SupportedProtocols() {
		if protocol == supported {
			return true
		}
	}
	return false
}

func (r Release) NormalizedAsset() string {
	if strings.TrimSpace(r.Asset) == "" {
		return DefaultReleaseAsset
	}
	return strings.TrimSpace(r.Asset)
}

func (r Release) Supports(target Target) bool {
	if target.Protocol <= 0 {
		return false
	}
	protocolOK := false
	for _, protocol := range r.Protocols {
		if protocol == target.Protocol {
			protocolOK = true
			break
		}
	}
	if !protocolOK {
		return false
	}

	if strings.TrimSpace(r.ModMin) != "" {
		if strings.TrimSpace(target.ModVersion) == "" || compareVersions(target.ModVersion, r.ModMin) < 0 {
			return false
		}
	}
	if strings.TrimSpace(r.ModMax) != "" {
		if strings.TrimSpace(target.ModVersion) == "" || compareVersions(target.ModVersion, r.ModMax) > 0 {
			return false
		}
	}
	return true
}

func Validate(index Index) error {
	if index.Schema != IndexSchemaVersion {
		return fmt.Errorf("unsupported compatibility index schema %d", index.Schema)
	}
	if len(index.Protocols) == 0 {
		return errors.New("compatibility index contains no protocol generations")
	}
	knownProtocols := map[int]struct{}{}
	lastProtocol := 0
	for i, generation := range index.Protocols {
		if generation.ID <= 0 {
			return fmt.Errorf("protocol generation %d has invalid id %d", i, generation.ID)
		}
		if generation.ID <= lastProtocol {
			return fmt.Errorf("protocol generation ids must be strictly increasing; %d follows %d", generation.ID, lastProtocol)
		}
		if _, exists := knownProtocols[generation.ID]; exists {
			return fmt.Errorf("duplicate protocol generation %d", generation.ID)
		}
		knownProtocols[generation.ID] = struct{}{}
		lastProtocol = generation.ID
	}
	if len(index.Releases) == 0 {
		return errors.New("compatibility index contains no releases")
	}
	seenTags := map[string]struct{}{}
	for i, release := range index.Releases {
		if strings.TrimSpace(release.BridgeVersion) == "" {
			return fmt.Errorf("release %d has no bridgeVersion", i)
		}
		tag := strings.TrimSpace(release.Tag)
		if tag == "" {
			return fmt.Errorf("release %d has no tag", i)
		}
		key := strings.ToLower(tag)
		if _, exists := seenTags[key]; exists {
			return fmt.Errorf("duplicate release tag %q", tag)
		}
		seenTags[key] = struct{}{}
		if len(release.Protocols) == 0 {
			return fmt.Errorf("release %q declares no supported protocols", tag)
		}
		seenProtocols := map[int]struct{}{}
		for _, protocol := range release.Protocols {
			if protocol <= 0 {
				return fmt.Errorf("release %q contains invalid protocol %d", tag, protocol)
			}
			if _, known := knownProtocols[protocol]; !known {
				return fmt.Errorf("release %q references undeclared protocol generation %d", tag, protocol)
			}
			if _, duplicate := seenProtocols[protocol]; duplicate {
				return fmt.Errorf("release %q repeats protocol %d", tag, protocol)
			}
			seenProtocols[protocol] = struct{}{}
		}
	}
	return nil
}

// CompatibleCandidates returns newest-first stable releases supporting target.
func CompatibleCandidates(index Index, target Target) []Release {
	candidates := make([]Release, 0, len(index.Releases))
	for _, release := range index.Releases {
		if !strings.EqualFold(strings.TrimSpace(release.Channel), "stable") {
			continue
		}
		if release.Supports(target) {
			candidates = append(candidates, release)
		}
	}
	sortNewest(candidates)
	return candidates
}

// StableCandidates is used by the explicit manual updater when no running mod
// supplied a target. It still follows the compatibility index rather than the
// development branch.
func StableCandidates(index Index) []Release {
	candidates := make([]Release, 0, len(index.Releases))
	for _, release := range index.Releases {
		if strings.EqualFold(strings.TrimSpace(release.Channel), "stable") {
			candidates = append(candidates, release)
		}
	}
	sortNewest(candidates)
	return candidates
}

func FindRelease(index Index, tag string) (Release, bool) {
	for _, release := range index.Releases {
		if strings.EqualFold(strings.TrimSpace(release.Tag), strings.TrimSpace(tag)) {
			return release, true
		}
	}
	return Release{}, false
}

func sortNewest(releases []Release) {
	sort.SliceStable(releases, func(i, j int) bool {
		return compareVersions(releases[i].BridgeVersion, releases[j].BridgeVersion) > 0
	})
}

// CompareVersions exposes the project's stable Vx.y.z ordering to the updater
// without duplicating version parsing outside the compatibility package.
func CompareVersions(a, b string) int { return compareVersions(a, b) }

// compareVersions intentionally handles the project's Vx.y.z labels without a
// third-party dependency. Stable release selection ignores prerelease labels.
func compareVersions(a, b string) int {
	pa := versionParts(a)
	pb := versionParts(b)
	for i := 0; i < len(pa); i++ {
		if pa[i] < pb[i] {
			return -1
		}
		if pa[i] > pb[i] {
			return 1
		}
	}
	return 0
}

func versionParts(version string) [4]int {
	var parts [4]int
	clean := strings.TrimSpace(version)
	clean = strings.TrimPrefix(clean, "V")
	clean = strings.TrimPrefix(clean, "v")
	fields := strings.FieldsFunc(clean, func(r rune) bool {
		return r == '.' || r == '-' || r == '+' || r == '_'
	})
	for i := 0; i < len(parts) && i < len(fields); i++ {
		n, err := strconv.Atoi(fields[i])
		if err != nil {
			break
		}
		parts[i] = n
	}
	return parts
}
