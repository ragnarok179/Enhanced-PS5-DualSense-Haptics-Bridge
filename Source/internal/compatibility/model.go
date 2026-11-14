package compatibility

import (
	"strconv"
	"strings"
)

const PendingFileName = "pending_bridge_compatibility.json"

type PendingTarget struct {
	ModVersion string `json:"modVersion,omitempty"`
	Protocol   int    `json:"protocol"`
}

// CompareVersions is shared by the Bridge, updater and release selection.
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

func versionParts(version string) [4]int {
	var out [4]int
	value := strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(version), "V"), "v")
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == '.' || r == '-' || r == '+' || r == '_'
	})
	for i := 0; i < len(out) && i < len(parts); i++ {
		n, err := strconv.Atoi(parts[i])
		if err != nil {
			break
		}
		out[i] = n
	}
	return out
}
