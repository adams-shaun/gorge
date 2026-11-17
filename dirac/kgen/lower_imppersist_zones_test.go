package kgen

import "testing"

// TestLeaveExileTriggerZones: the kernel's TrigLeaveExile fires from the
// battlefield listener walk (fireLeaveExile), and slotTriggers drops an
// ability whose TriggerZones is not None/Battlefield. A leave-exile trigger
// scoped elsewhere would lower and never fire, so the shape must be refused.
func TestLeaveExileTriggerZones(t *testing.T) {
	base := map[string]string{"Origin": "Exile", "Destination": "Any"}
	for _, tc := range []struct {
		name string
		z    string
		want bool
	}{
		{"no zone", "", true},
		{"battlefield", "Battlefield", true},
		{"graveyard", "Graveyard", false},
		{"command", "Command", false},
		{"hand", "Hand", false},
	} {
		p := map[string]string{"Origin": base["Origin"], "Destination": base["Destination"]}
		if tc.z != "" {
			p["TriggerZones"] = tc.z
		}
		if got := leaveExileTrigger(p); got != tc.want {
			t.Errorf("%s: leaveExileTrigger=%v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestImpForgetThirdSourceLowers: the lockstep case where an effect other
// than the carrier moves an imprinted card out of exile (TestImpForgetRemoval).
func TestImpForgetThirdSourceLowers(t *testing.T) {
	c := lowerCorpusCard(t, "Binding Negotiation")
	if r := probeRefusals(c); r != "" {
		t.Errorf("Binding Negotiation refused: %s", r)
	}
}
