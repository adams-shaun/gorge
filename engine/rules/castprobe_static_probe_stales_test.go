package rules

import "testing"

// staticProbeStales decides the castprobe.go offerAsSpellOnStack probe's
// staticEffects-memo invalidation. A kept memo must be one whose every input
// a card move never touches: the recorded gates' dependency classes (the
// active player, a life total) AND a build that recorded no state read. A
// state-read build bakes zone-dependent content into the memo at scan time
// (SharedKeywordsZone's filter, a GainsAbilitiesOf$ over scoped zones, AddType$
// ImprintedCreatureType's exiled creature), so it must rebuild around the
// probe even when it recorded zero gates. Each case below names the input
// shape; the state-read rows and their no-state-read twins differ in that one
// field alone, which is exactly the discriminator the probe must honor.
func TestStaticProbeStales(t *testing.T) {
	cases := []struct {
		name       string
		epoch      int
		events     int
		gatesKnown bool
		stateRead  bool
		gates      []staticGateRec
		want       bool
	}{
		{"epoch off the log head", 3, 4, true, false, nil, true},
		{"unknown gate set", 4, 4, false, false, nil, true},
		{"already invalidated", -1, 4, true, false, nil, true},
		{"no gates, no state read", 4, 4, true, false, nil, false},
		{"active-only gates", 4, 4, true, false, []staticGateRec{{dep: staticGateDepActive}}, false},
		{"life-only gates", 4, 4, true, false, []staticGateRec{{dep: staticGateDepLife}}, false},
		{"active and life gates", 4, 4, true, false, []staticGateRec{{dep: staticGateDepActive}, {dep: staticGateDepLife}}, false},
		{"state-read build, no gates", 4, 4, true, true, nil, true},
		{"state-read build, life gates", 4, 4, true, true, []staticGateRec{{dep: staticGateDepLife}}, true},
		{"zone gate", 4, 4, true, false, []staticGateRec{{dep: staticGateDepAny}}, true},
		{"mixed gates", 4, 4, true, false, []staticGateRec{{dep: staticGateDepActive}, {dep: staticGateDepAny}}, true},
	}
	for _, tc := range cases {
		if got := staticProbeStales(tc.epoch, tc.events, tc.gatesKnown, tc.stateRead, tc.gates); got != tc.want {
			t.Errorf("%s: staticProbeStales(epoch=%d events=%d gatesKnown=%v stateRead=%v) = %v, want %v",
				tc.name, tc.epoch, tc.events, tc.gatesKnown, tc.stateRead, got, tc.want)
		}
	}
}
