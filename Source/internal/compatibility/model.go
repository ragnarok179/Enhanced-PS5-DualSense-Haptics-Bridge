package compatibility

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const (
	IndexSchemaVersion  = 1
	IndexFileName       = "BRIDGE_COMPATIBILITY.json"
	PendingFileName     = "pending_bridge_compatibility.json"
	DefaultReleaseAsset = "Enhanced_PS5_DualSense_Haptics_Bridge.zip"
)

type Target struct {
	ModVersion string
	Protocol   int
}
type PendingTarget struct {
	ModVersion string `json:"modVersion,omitempty"`
	Protocol   int    `json:"protocol"`
}
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

func (r Release) NormalizedAsset() string {
	if strings.TrimSpace(r.Asset) == "" {
		return DefaultReleaseAsset
	}
	return strings.TrimSpace(r.Asset)
}
func (r Release) Supports(t Target) bool {
	if t.Protocol <= 0 {
		return false
	}
	ok := false
	for _, p := range r.Protocols {
		if p == t.Protocol {
			ok = true
			break
		}
	}
	if !ok {
		return false
	}
	if strings.TrimSpace(r.ModMin) != "" && (strings.TrimSpace(t.ModVersion) == "" || CompareVersions(t.ModVersion, r.ModMin) < 0) {
		return false
	}
	if strings.TrimSpace(r.ModMax) != "" && (strings.TrimSpace(t.ModVersion) == "" || CompareVersions(t.ModVersion, r.ModMax) > 0) {
		return false
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
	known := map[int]struct{}{}
	last := 0
	for i, g := range index.Protocols {
		if g.ID <= 0 {
			return fmt.Errorf("protocol generation %d has invalid id %d", i, g.ID)
		}
		if g.ID <= last {
			return fmt.Errorf("protocol generation ids must be strictly increasing; %d follows %d", g.ID, last)
		}
		if _, x := known[g.ID]; x {
			return fmt.Errorf("duplicate protocol generation %d", g.ID)
		}
		known[g.ID] = struct{}{}
		last = g.ID
	}
	if len(index.Releases) == 0 {
		return errors.New("compatibility index contains no releases")
	}
	tags := map[string]struct{}{}
	for i, r := range index.Releases {
		if strings.TrimSpace(r.BridgeVersion) == "" {
			return fmt.Errorf("release %d has no bridgeVersion", i)
		}
		tag := strings.TrimSpace(r.Tag)
		if tag == "" {
			return fmt.Errorf("release %d has no tag", i)
		}
		key := strings.ToLower(tag)
		if _, x := tags[key]; x {
			return fmt.Errorf("duplicate release tag %q", tag)
		}
		tags[key] = struct{}{}
		if len(r.Protocols) == 0 {
			return fmt.Errorf("release %q declares no supported protocols", tag)
		}
		seen := map[int]struct{}{}
		for _, p := range r.Protocols {
			if _, x := known[p]; !x {
				return fmt.Errorf("release %q references undeclared protocol generation %d", tag, p)
			}
			if _, x := seen[p]; x {
				return fmt.Errorf("release %q repeats protocol %d", tag, p)
			}
			seen[p] = struct{}{}
		}
	}
	return nil
}
func CompatibleCandidates(index Index, t Target) []Release {
	out := make([]Release, 0, len(index.Releases))
	for _, r := range index.Releases {
		if strings.EqualFold(strings.TrimSpace(r.Channel), "stable") && r.Supports(t) {
			out = append(out, r)
		}
	}
	sortNewest(out)
	return out
}
func StableCandidates(index Index) []Release {
	out := make([]Release, 0, len(index.Releases))
	for _, r := range index.Releases {
		if strings.EqualFold(strings.TrimSpace(r.Channel), "stable") {
			out = append(out, r)
		}
	}
	sortNewest(out)
	return out
}
func FindRelease(index Index, tag string) (Release, bool) {
	for _, r := range index.Releases {
		if strings.EqualFold(strings.TrimSpace(r.Tag), strings.TrimSpace(tag)) {
			return r, true
		}
	}
	return Release{}, false
}
func sortNewest(r []Release) {
	sort.SliceStable(r, func(i, j int) bool { return CompareVersions(r[i].BridgeVersion, r[j].BridgeVersion) > 0 })
}
func CompareVersions(a, b string) int {
	pa, pb := versionParts(a), versionParts(b)
	for i := range pa {
		if pa[i] < pb[i] {
			return -1
		}
		if pa[i] > pb[i] {
			return 1
		}
	}
	return 0
}
func versionParts(v string) [4]int {
	var out [4]int
	clean := strings.TrimSpace(v)
	clean = strings.TrimPrefix(strings.TrimPrefix(clean, "V"), "v")
	fields := strings.FieldsFunc(clean, func(r rune) bool { return r == '.' || r == '-' || r == '+' || r == '_' })
	for i := 0; i < len(out) && i < len(fields); i++ {
		n, e := strconv.Atoi(fields[i])
		if e != nil {
			break
		}
		out[i] = n
	}
	return out
}
