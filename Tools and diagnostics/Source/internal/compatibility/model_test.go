package compatibility

import "testing"

func TestCompatibleCandidatesNewestFirst(t *testing.T) {
	index := Index{Schema: 1, Protocols: []ProtocolGeneration{{ID: 40}, {ID: 41}, {ID: 42}, {ID: 43}}, Releases: []Release{
		{BridgeVersion: "V1.5.4", Tag: "V1.5.4", Channel: "stable", Protocols: []int{40, 41}},
		{BridgeVersion: "V1.6.0", Tag: "V1.6.0", Channel: "stable", Protocols: []int{41, 42}},
		{BridgeVersion: "V1.7.0", Tag: "V1.7.0", Channel: "stable", Protocols: []int{43}},
	}}
	got := CompatibleCandidates(index, Target{ModVersion: "V1.6.2", Protocol: 41})
	if len(got) != 2 || got[0].Tag != "V1.6.0" || got[1].Tag != "V1.5.4" {
		t.Fatalf("unexpected candidates: %+v", got)
	}
}

func TestReleaseModBoundsAreOptionalSafety(t *testing.T) {
	r := Release{BridgeVersion: "V2.0.0", Tag: "V2.0.0", Channel: "stable", Protocols: []int{42}, ModMin: "V1.8.0", ModMax: "V1.9.9"}
	if !r.Supports(Target{ModVersion: "V1.9.0", Protocol: 42}) {
		t.Fatal("expected target inside optional mod bounds to be supported")
	}
	if r.Supports(Target{ModVersion: "V2.0.0", Protocol: 42}) {
		t.Fatal("expected target above modMax to be rejected")
	}
	if r.Supports(Target{Protocol: 42}) {
		t.Fatal("unknown mod version must not bypass explicit mod bounds")
	}
}

func TestValidateRejectsDuplicateTags(t *testing.T) {
	index := Index{Schema: 1, Protocols: []ProtocolGeneration{{ID: 40}, {ID: 41}}, Releases: []Release{
		{BridgeVersion: "V1.0.0", Tag: "V1.0.0", Channel: "stable", Protocols: []int{40}},
		{BridgeVersion: "V1.0.1", Tag: "v1.0.0", Channel: "stable", Protocols: []int{41}},
	}}
	if err := Validate(index); err == nil {
		t.Fatal("expected duplicate tag validation failure")
	}
}

func TestCurrentSupportedProtocols(t *testing.T) {
	if !ProtocolSupported(40) || !ProtocolSupported(41) || ProtocolSupported(42) {
		t.Fatalf("unexpected protocol support set: %v", SupportedProtocols())
	}
}

func TestValidateRejectsNonMonotonicProtocolCatalog(t *testing.T) {
	index := Index{
		Schema:    1,
		Protocols: []ProtocolGeneration{{ID: 41}, {ID: 40}},
		Releases:  []Release{{BridgeVersion: "V1.0.0", Tag: "V1.0.0", Channel: "stable", Protocols: []int{41}}},
	}
	if err := Validate(index); err == nil {
		t.Fatal("expected non-monotonic protocol catalog to fail validation")
	}
}
