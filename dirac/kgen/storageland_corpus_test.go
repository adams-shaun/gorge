package kgen

import (
	"strings"
	"testing"
)

// TestStorageLandCorpusRefusals pins the real corpus shapes the storage X
// gap does not lower: the five fee-bearing storage lands stay refused for
// their announced-X counter cost beside a generic fee, and Crucible of the
// Spirit Dragon for its cost beyond {T}. No lowered storage card regresses
// into this list.
func TestStorageLandCorpusRefusals(t *testing.T) {
	cases := []struct{ name, want string }{
		{"Molten Slagheap", "an announced-X counter cost beside a generic fee"},
		{"Dreadship Reef", "an announced-X counter cost beside a generic fee"},
		{"Saltcrusted Steppe", "an announced-X counter cost beside a generic fee"},
		{"Fungal Reaches", "an announced-X counter cost beside a generic fee"},
		{"Calciform Pools", "an announced-X counter cost beside a generic fee"},
		{"Crucible of the Spirit Dragon", "mana restriction on a source whose cost goes beyond {T}"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			lc := lowerCorpusCard(t, c.name)
			var got []string
			for _, rf := range lc.Refusals {
				got = append(got, rf.Reason)
			}
			if len(got) != 1 || !strings.Contains(got[0], c.want) {
				t.Fatalf("%s: refusals = %v, want exactly one containing %q", c.name, got, c.want)
			}
			// gorge's script now reaches these sources, so the reason must not
			// repeat the old claim that its planner prices no source fee.
			if strings.Contains(got[0], "gorge's planner prices no such source") {
				t.Fatalf("%s: stale reason %q still claims gorge's planner prices no source fee", c.name, got[0])
			}
		})
	}
}
