package compatibility

import "testing"

func TestCompareCurrentAndFutureReleases(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want int
	}{
		{"V1.43", "V1.50", -1},
		{"V1.50.0", "v1.50", 0},
		{"V1.51", "V1.50", 1},
		{"V2.0", "V1.99", 1},
	} {
		got := CompareVersions(tc.a, tc.b)
		if got != tc.want {
			t.Fatalf("CompareVersions(%q,%q)=%d want %d", tc.a, tc.b, got, tc.want)
		}
	}
}
