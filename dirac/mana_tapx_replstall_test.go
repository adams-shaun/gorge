package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestTapXScriptStallPropagates: the tap-another search's only payment path is
// Thermopod's "Sacrifice a creature: Add {R}" (kop.SacAnyManaAb, no {T}), and
// the sacrificed creature dies under two exile-instead replacements (two
// Samurai of the Pale Curtain, themselves the only candidates), which latches
// StallReplLeave in the fork (dieZone). The search must declare the stall on
// the real state (capFull, the game draws as an unmodelled stall) instead of
// treating the line as "no payment" and silently dropping the play: gorge
// offers the cast the kernel's priority options then lack (dr-mbc-baaa pair
// 103, decision 254). The matchup holds the four cards alone, so the table's
// prep carries no initStall.
func TestTapXScriptStallPropagates(t *testing.T) {
	set := cardsettest.Set(t, "mbc")
	names := []string{"Thermopod", "Samurai of the Pale Curtain", "Abrade", "Plains"}
	deck := []cardset.Entry{}
	for _, n := range names {
		i, ok := set.Lookup(n)
		if !ok {
			t.Fatalf("%s not in the mbc pool", n)
		}
		if !set.Supported()[i] {
			t.Fatalf("%s not supported in the mbc table", n)
		}
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	for i := range deck {
		if deck[i].Name == "Samurai of the Pale Curtain" {
			deck[i].Count = 2 // the two competing exile-instead replacements
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	sc.perm(id["Thermopod"], 0)
	sc.perm(id["Samurai of the Pale Curtain"], 0)
	sc.perm(id["Samurai of the Pale Curtain"], 0)
	abrade := sc.hand(id["Abrade"], 0)
	s, k := sc.s, sc.k
	a := MakeAction(AkCast, TCode(abrade), 0, 0)
	if s.Stalled() {
		t.Fatalf("precondition: the board alone latched %s", StallName(s.StallReason()))
	}
	if s.tapXScript(k, a, 0, false) {
		t.Fatal("precondition: the search found a payment path, want the only one to stall")
	}
	if !s.Stalled() || s.StallReason() != StallReplLeave {
		t.Fatalf("the search dropped the play silently: stalled %v reason %s, want stalled %s",
			s.Stalled(), StallName(s.StallReason()), StallName(StallReplLeave))
	}
}
