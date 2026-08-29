package compatibility

import "testing"

func sampleIndex() Index {
	return Index{Schema: 1, Protocols: []ProtocolGeneration{{ID: 40}, {ID: 41}, {ID: 42}, {ID: 43}}, Releases: []Release{{BridgeVersion: "V1.3", Tag: "V1.3", Channel: "stable", Protocols: []int{40, 41}, Asset: "old.zip"}, {BridgeVersion: "V1.4", Tag: "V1.4", Channel: "stable", Protocols: []int{40, 41, 42}, Asset: "new.zip"}, {BridgeVersion: "V1.41", Tag: "V1.41", Channel: "stable", Protocols: []int{40, 41, 42, 43}, Asset: "future.zip"}}}
}
func TestCompatibilityCandidatesNewestFirst(t *testing.T) {
	idx := sampleIndex()
	if err := Validate(idx); err != nil {
		t.Fatal(err)
	}
	got := CompatibleCandidates(idx, Target{ModVersion: "V1.2", Protocol: 42})
	if len(got) != 2 || got[0].BridgeVersion != "V1.41" || got[1].BridgeVersion != "V1.4" {
		t.Fatalf("unexpected candidates: %+v", got)
	}
	got = CompatibleCandidates(idx, Target{ModVersion: "V1.3", Protocol: 43})
	if len(got) != 1 || got[0].BridgeVersion != "V1.41" {
		t.Fatalf("protocol 43 should require V1.41: %+v", got)
	}
}
func TestReleaseModRange(t *testing.T) {
	r := Release{Protocols: []int{41}, ModMin: "V1.1", ModMax: "V1.2"}
	if !r.Supports(Target{ModVersion: "V1.2", Protocol: 41}) {
		t.Fatal("V1.2 should be supported")
	}
	if r.Supports(Target{ModVersion: "V1.3", Protocol: 41}) {
		t.Fatal("V1.3 should be outside range")
	}
}
