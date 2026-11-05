package oracle

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestPlaySetServesSyntheticPool holds dplay's set lookup to the synthetic pools,
// which play unverified cards, and to the deck pools, which stay exactly as ByName
// serves them.
func TestPlaySetServesSyntheticPool(t *testing.T) {
	corpusDirOrSkip(t)
	name, cards := unverifiedSynthCards(t, 1)
	set, err := PlaySet(name)
	if err != nil {
		t.Fatal(err)
	}
	i, ok := set.Lookup(cards[0])
	if !ok {
		t.Fatalf("PlaySet(%s) does not hold %q", name, cards[0])
	}
	if sup := set.Supported(); !sup[i] {
		t.Fatalf("%q is unverified but not supported by PlaySet(%s)", cards[0], name)
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

// unverifiedSynthCards is the first synthetic pool, by name, that holds at least n
// unverified deck cards, with the first n of them. Evidence rounds verify cards, so
// no test names a card it needs to stay unverified.
func unverifiedSynthCards(t *testing.T, n int) (string, []string) {
	t.Helper()
	names, err := cardset.SynthNames()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if !strings.HasPrefix(name, "std-") {
			continue
		}
		set, err := PlaySet(name)
		if err != nil {
			t.Fatal(err)
		}
		var cards []string
		for i, card := range set.Names()[:set.NumCards()] {
			if set.Status(i) == cardset.StatusUnverified {
				cards = append(cards, card)
			}
		}
		if len(cards) >= n {
			return name, cards[:n]
		}
	}
	t.Fatalf("no synthetic pool holds %d unverified cards", n)
	return "", nil
}
