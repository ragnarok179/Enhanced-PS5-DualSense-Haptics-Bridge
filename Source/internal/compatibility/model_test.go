package compatibility

import "testing"

func sampleIndex() Index {
	return Index{Schema: 1, Protocols: []ProtocolGeneration{{ID: 41}, {ID: 42}, {ID: 43}}, Releases: []Release{{BridgeVersion: "V1.42", Tag: "V1.42", Channel: "stable", Protocols: []int{41, 42, 43}, ModMin: "V1.3", ModMax: "V1.42", Asset: "Enhanced_PS5_DualSense_Haptics_Bridge_V1.42.zip"}}}
}

func TestCompatibilityCandidatesUseV13Floor(t *testing.T) {
	idx := sampleIndex()
	if err := Validate(idx); err != nil {
		t.Fatal(err)
	}
	for _, target := range []Target{{ModVersion: "V1.3", Protocol: 41}, {ModVersion: "V1.4", Protocol: 42}, {ModVersion: "V1.41", Protocol: 43}, {ModVersion: "V1.42", Protocol: 43}} {
		got := CompatibleCandidates(idx, target)
		if len(got) != 1 || got[0].BridgeVersion != "V1.42" {
			t.Fatalf("unexpected candidates for %+v: %+v", target, got)
		}
	}
	if got := CompatibleCandidates(idx, Target{ModVersion: "V1.2", Protocol: 42}); len(got) != 0 {
		t.Fatalf("V1.2 must be outside supported range: %+v", got)
	}
}

func TestReleaseModRange(t *testing.T) {
	r := Release{Protocols: []int{41, 42, 43}, ModMin: "V1.3", ModMax: "V1.42"}
	if !r.Supports(Target{ModVersion: "V1.3", Protocol: 41}) {
		t.Fatal("V1.3 should be supported")
	}
	if !r.Supports(Target{ModVersion: "V1.42", Protocol: 43}) {
		t.Fatal("V1.42 should be supported")
	}
	if r.Supports(Target{ModVersion: "V1.2", Protocol: 42}) {
		t.Fatal("V1.2 should be outside range")
	}
}
