package oracle

import (
	"errors"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestLionsEyeDiamondMixRefused: the Diamond deck of TestLionsEyeDiamondLockstep
// against a deck holding a census gap of another kind (Bloom Tender's
// each-colour ability, Heap Gate's filter) is refused when the matchup is
// built, as diamondmix naming both cards: gorge's one prefix search activates
// the Diamond and the other gap together and the kernel models the Diamond
// alone, so the pair never reaches a game to diverge in. The Diamond-alone
// matchup still builds.
func TestLionsEyeDiamondMixRefused(t *testing.T) {
	corpusDirOrSkip(t)
	partners := []string{"Bloom Tender", "Heap Gate"}
	other := func(p string) []cardset.Entry {
		return []cardset.Entry{{Name: p, Count: 4}, {Name: "Hill Giant", Count: 6}, {Name: "Grizzly Bears", Count: 10}, {Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10}}
	}
	var names [2][]string
	for _, e := range ledDecks[0] {
		names[0] = append(names[0], e.Name)
	}
	for _, e := range ledDecks[1] {
		names[1] = append(names[1], e.Name)
	}
	for _, p := range partners {
		for _, e := range other(p) {
			names[1] = append(names[1], e.Name)
		}
	}
	set := printedDelveSet(t, names)
	if _, _, _, err := set.Matchup(ledDecks[0], ledDecks[1]); err != nil {
		t.Fatalf("the Diamond-alone matchup is refused: %v", err)
	}
	for _, p := range partners {
		if _, ok := set.Lookup(p); !ok {
			t.Fatalf("precondition: %s not in the set", p)
		}
		_, _, _, err := set.Matchup(ledDecks[0], other(p))
		var r *cardset.InteractionRefusal
		if !errors.As(err, &r) {
			t.Fatalf("Diamond vs %s: err %v, want an InteractionRefusal", p, err)
		}
		if r.Reason != "diamondmix" || !slices.Contains(r.Cards, "Lion's Eye Diamond") || !slices.Contains(r.Cards, p) {
			t.Fatalf("Diamond vs %s: refused %s naming %v, want diamondmix naming both", p, r.Reason, r.Cards)
		}
		t.Logf("Diamond vs %s: %v", p, err)
	}
}
