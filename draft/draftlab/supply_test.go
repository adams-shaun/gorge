package draftlab

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/draft/limited/setdata"
)

// TestOpenPodMatchesBooster asserts the lab's pod opener agrees with the
// direct booster call: same shape, same CardRefs everywhere, unique IDs
// pod-wide. (The cmd/draftsim event driver and the lab must compose the
// same packs so their logs agree.)
func TestOpenPodMatchesBooster(t *testing.T) {
	l := testLab(t)
	pod, err := l.OpenPod("FDN", setdata.Arena, 5, 4, 3)
	if err != nil {
		t.Fatalf("OpenPod: %v", err)
	}
	if len(pod) != 4 || len(pod[0]) != 3 {
		t.Fatalf("pod shape = %d×%d, want 4×3", len(pod), len(pod[0]))
	}
	seen := make(map[string]bool)
	for s := range pod {
		for r := range pod[s] {
			if len(pod[s][r]) == 0 {
				t.Fatalf("seat %d round %d pack is empty", s, r)
			}
			for _, ref := range pod[s][r] {
				if ref.Set != "FDN" && ref.Set != "SPG" {
					t.Errorf("pack card %+v is neither FDN nor the SPG bonus sheet", ref)
				}
				if seen[ref.ID] {
					t.Errorf("card ID %s appears twice", ref.ID)
				}
				seen[ref.ID] = true
			}
		}
	}
	if t.Failed() {
		t.Fatal("pod disagrees with the booster contract")
	}
}

// TestSealedPoolShape: the flattened pool keeps pack order and unique IDs.
func TestSealedPoolShape(t *testing.T) {
	l := testLab(t)
	pool, err := l.SealedPool("FDN", setdata.Arena, 9, 6)
	if err != nil {
		t.Fatalf("SealedPool: %v", err)
	}
	if len(pool) == 0 {
		t.Fatal("empty sealed pool")
	}
	seen := make(map[string]bool)
	for _, ref := range pool {
		if ref.Set != "FDN" && ref.Set != "SPG" {
			t.Errorf("pool card %+v is neither FDN nor the SPG bonus sheet", ref)
		}
		if seen[ref.ID] {
			t.Errorf("card ID %s appears twice", ref.ID)
		}
		seen[ref.ID] = true
	}
}

func TestCardNames(t *testing.T) {
	l := testLab(t)
	names, err := l.CardNames("FDN", setdata.Arena)
	if err != nil {
		t.Fatalf("CardNames: %v", err)
	}
	if len(names) == 0 {
		t.Fatal("empty card-name universe")
	}
	if !slices.IsSorted(names) {
		t.Error("CardNames is not sorted")
	}
	seen := make(map[string]bool)
	for _, n := range names {
		if seen[n] {
			t.Errorf("duplicate name %q", n)
		}
		seen[n] = true
	}
	// The FDN universe covers every card a pod opened at the same
	// collation can carry.
	pod, err := l.OpenPod("FDN", setdata.Arena, 11, 4, 3)
	if err != nil {
		t.Fatal(err)
	}
	known := make(map[string]bool, len(names))
	for _, n := range names {
		known[n] = true
	}
	for s := range pod {
		for r := range pod[s] {
			for _, ref := range pod[s][r] {
				if !known[ref.Name] {
					t.Errorf("pack card %q is outside the CardNames universe", ref.Name)
				}
			}
		}
	}
}

func TestSupplyErrors(t *testing.T) {
	l := testLab(t)
	if _, err := l.OpenPacks("NO-SUCH-SET", setdata.Arena, 1, 3); err == nil {
		t.Error("OpenPacks accepted an unknown set")
	}
	if _, err := l.OpenPod("FDN", "no-such-collation", 1, 4, 3); err == nil {
		t.Error("OpenPod accepted an unknown collation")
	}
}
