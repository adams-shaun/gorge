package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestPayBeforeETBStrayEntryStalls: Mox Diamond's replacement is answered while
// its cast spell resolves (the lockstep in dirac/oracle/moxdiamond_test.go);
// the card put onto the battlefield any other way would enter without the
// discard election gorge asks, so the game stalls (StallPayETB) instead of
// continuing with a Mox gorge would not have in play.
func TestPayBeforeETBStrayEntryStalls(t *testing.T) {
	set := cardsettest.Set(t, "moxdiamond")
	deck := []cardset.Entry{{Name: "Mox Diamond", Count: 4}, {Name: "Mountain", Count: 36}}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Mountain"], Config{NoElide: true})
	if sc.s.StallReason() != StallNone {
		t.Fatalf("precondition: the fresh scenario already stalled with %s", StallName(sc.s.StallReason()))
	}
	sc.perm(id["Mox Diamond"], 0) // an entry that is not a cast spell resolving
	if got := sc.s.StallReason(); got != StallPayETB {
		t.Fatalf("a Mox Diamond put onto the battlefield: stall reason %q, want %q", StallName(got), StallName(StallPayETB))
	}
}
