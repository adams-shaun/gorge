package kgen

import "testing"

// TestSnowFilterLowers: a filter naming the Snow supertype (the Snow head,
// the Snow property, nonSnow) lowers with no refusal.
func TestSnowFilterLowers(t *testing.T) {
	for _, name := range []string{"Skred", "Chill to the Bone", "Abominable Treefolk", "Priest of the Haunted Edge"} {
		t.Run(name, func(t *testing.T) {
			if r := probeRefusals(lowerCorpusCard(t, name)); r != "" {
				t.Errorf("%s refused: %s", name, r)
			}
		})
	}
}
