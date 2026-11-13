package draftlab

import (
	"testing"
)

// TestDraftMBCPod drafts a Mystery Booster Commander pod at the set's
// default collation (its only one, draft): 20-card packs whose printings
// come from MBC, MB2 and PLST resolve to the one base set MBC, and every
// seat drafts all 60 cards.
func TestDraftMBCPod(t *testing.T) {
	l := testLab(t)
	pod, err := l.OpenPod("MBC", "", 41, 4, 3)
	if err != nil {
		t.Fatalf("OpenPod(MBC): %v", err)
	}
	for s := range pod {
		for r, pack := range pod[s] {
			if len(pack) != 20 {
				t.Fatalf("seat %d round %d pack holds %d cards, want 20", s, r, len(pack))
			}
		}
	}
	res, err := l.Draft(pod, newDrafters(t, l, "rating-colour", 1), 41)
	if err != nil {
		t.Fatalf("Draft: %v", err)
	}
	if res.Set != "MBC" {
		t.Errorf("Draft result set = %q, want MBC", res.Set)
	}
	for s, pool := range res.Pools {
		if len(pool) != 60 {
			t.Errorf("seat %d drafted %d cards, want 60", s, len(pool))
		}
	}
}
