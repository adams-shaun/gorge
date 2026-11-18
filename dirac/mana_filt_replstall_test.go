package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestFiltScriptStallPropagates: the filter search's only payment path runs
// Apprentice Wizard's filter ({U}, {T}: Add {C}{C}{C}, funded by the Island)
// and Thermopod's "Sacrifice a creature: Add {R}" (an otherTapX gap of the
// search, sacXDo), and the sacrificed creature dies under two exile-instead
// replacements (two Samurai of the Pale Curtain), which latches StallReplLeave
// in the fork. The search must declare the stall on the real state (capFull,
// the game draws as an unmodelled stall) instead of treating the line as "no
// payment" and silently dropping the play: gorge offers the cast the kernel's
// priority options then lack (TestTapXScriptStallPropagates's mechanism).
func TestFiltScriptStallPropagates(t *testing.T) {
	set := cardsettest.Set(t, "mbc")
	names := []string{"Apprentice Wizard", "Thermopod", "Samurai of the Pale Curtain", "Abrade", "Island", "Havoc Jester"}
	deck := []cardset.Entry{}
	for _, n := range names {
		i, ok := set.Lookup(n)
		if !ok {
			t.Fatalf("%s not in the mbc pool", n)
		}
		if !set.Supported()[i] {
			t.Fatalf("%s not supported in the mbc table", n)
		}
		c := 1
		if n == "Samurai of the Pale Curtain" {
			c = 2 // the two competing exile-instead replacements
		}
		deck = append(deck, cardset.Entry{Name: n, Count: c})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	for _, samurai := range []int{1, 2} {
		sc := newScenTableLib(t, tbl, id["Island"], Config{NoElide: true})
		sc.perm(id["Island"], 0)
		wiz := sc.perm(id["Apprentice Wizard"], 0)
		th := sc.perm(id["Thermopod"], 0)
		for i := 0; i < samurai; i++ {
			sc.perm(id["Samurai of the Pale Curtain"], 0)
		}
		sc.perm(id["Havoc Jester"], 0)
		abrade := sc.hand(id["Abrade"], 0)
		s, k := sc.s, sc.k
		a := MakeAction(AkCast, TCode(abrade), 0, 0)
		if s.Stalled() {
			t.Fatalf("precondition: the board alone latched %s", StallName(s.StallReason()))
		}
		if !s.scriptFilt(k, 0).has(sc.slot(wiz)) {
			t.Fatal("precondition: the Wizard is no census filter gap")
		}
		if !s.otherGaps(k, 0, a).has(sc.slot(th)) {
			t.Fatal("precondition: Thermopod is no other gap of the filter search")
		}
		found := s.filtScript(k, a, 0, false)
		if samurai == 1 {
			// control: one replacement competes with nothing, the line pays
			if !found || s.Stalled() {
				t.Fatalf("control: found %v stalled %v, want a path and no stall", found, s.Stalled())
			}
			continue
		}
		if found {
			t.Fatal("precondition: the search found a payment path, want the only one to stall")
		}
		if !s.Stalled() || s.StallReason() != StallReplLeave {
			t.Fatalf("the search dropped the play silently: stalled %v reason %s, want stalled %s",
				s.Stalled(), StallName(s.StallReason()), StallName(StallReplLeave))
		}
	}
}
