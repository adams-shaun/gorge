package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestSneakEntryTapFiresNoTapAll: a sneaked creature enters tapped and
// attacking; that tapped state is its entry state, not a tapping (CR 603.2e,
// gorge emitTap's entering flag), so Rewrite History's "whenever one or more
// creatures you control become tapped" does not trigger.
func TestSneakEntryTapFiresNoTapAll(t *testing.T) {
	set := cardsettest.Synth(t, "std-msh")
	names := []string{"Rewrite History", "Elektra, Daughter of the Hand"}
	var deck []cardset.Entry
	for _, n := range names {
		if i, ok := set.Lookup(n); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the std-msh table", n)
		}
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s, k := sc.s, sc.k
	sc.perm(id[names[0]], 0)
	el := sc.perm(id[names[1]], 0)
	sl := sc.slot(el)
	// precondition: tapping the creature the ordinary way does trigger the enchantment
	s.perms[sl].flags |= pfTapped
	s.fireTaps(k, sl)
	if s.npend != 1 {
		t.Fatalf("an ordinary tap queued %d triggers, want 1: the test sees nothing", s.npend)
	}
	s.npend = 0
	s.perms[sl].flags &^= pfTapped
	s.sneakEnter(k, sl, &stackEntry{ctrl: 0, trigObj: NoObj})
	if s.perms[sl].flags&pfTapped == 0 {
		t.Fatal("the sneaked creature did not enter tapped")
	}
	if s.npend != 0 {
		t.Fatalf("entering tapped queued %d triggers, want none", s.npend)
	}
}
