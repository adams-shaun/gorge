package oracle

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestPlaySetServesSyntheticPool holds dplay's set lookup to the synthetic pools,
// which play unverified cards, and to the deck pools, which stay exactly as ByName
// serves them.
func TestPlaySetServesSyntheticPool(t *testing.T) {
	corpusDirOrSkip(t)
	set, err := PlaySet("std-trk")
	if err != nil {
		t.Fatal(err)
	}
	const card = "The Holodeck"
	i, ok := set.Lookup(card)
	if !ok {
		t.Fatalf("PlaySet(std-trk) does not hold %q", card)
	}
	if got := set.Status(i); got != cardset.StatusUnverified {
		t.Fatalf("%q is %s in std-trk, want %s: the test needs an unverified card", card, got, cardset.StatusUnverified)
	}
	if sup := set.Supported(); !sup[i] {
		t.Fatalf("%q is unverified but not supported by PlaySet(std-trk)", card)
	}

	fdn, err := PlaySet("fdn")
	if err != nil {
		t.Fatal(err)
	}
	if fdn.Name() != "fdn" {
		t.Fatalf("PlaySet(fdn).Name() = %q, want fdn", fdn.Name())
	}
	base, err := cardset.ByName("fdn")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := fdn.Supported(), base.Supported(); !slices.Equal(got, want) {
		t.Fatalf("PlaySet(fdn) supports %d entries, ByName(fdn) %d", len(got), len(want))
	}
}
