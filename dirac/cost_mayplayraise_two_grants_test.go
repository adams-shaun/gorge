package dirac

// One cast is made under one may-play permission (CR 601.2b/601.2f, 118.9),
// so two granting statics that both carry a RaiseCost$ charge ONE raise. This
// pins the kernel's pick (cost_mayplayraise.go mayRaise): the first granting
// static in the walk order, with no StallMayRaise for the second. The
// pre-fix kernel capped the game (StallMayRaise) rather than price one.

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

func TestTwoGrantingMayPlayRaisesPriceOne(t *testing.T) {
	tbl, err := cardsettest.Set(t, "mayraise").Pool()
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	for _, name := range []string{"Festival of Embers", "Shock"} {
		if _, ok := id[name]; !ok {
			t.Fatalf("%s not in the mayraise pool", name)
		}
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	// Two Festivals of Embers under seat 0, and a Shock in seat 0's
	// graveyard: both statics grant the instant's graveyard cast and each
	// carries RaiseCost$ PayLife<1>.
	// Shock first: a Festival already on the battlefield would exile it
	// instead of letting it reach the graveyard (its own replacement).
	shock := sc.grave(id["Shock"], 0, 1)[0]
	sc.perm(id["Festival of Embers"], 0)
	sc.perm(id["Festival of Embers"], 0)

	// Precondition: the rule's walk sees BOTH granting statics carrying a
	// raise, so a sum (2 life) and one permission (1 life) differ.
	raised := 0
	sc.s.mayPlayStatics(sc.k, 0, shock, func(st statSrc) bool {
		if st.a.Kind == kop.AbStatic && st.a.Static.Kind == kop.StatMayPlayFrom && st.a.Static.StFlags&kop.StMayRaise != 0 {
			raised++
		}
		return true
	})
	if raised != 2 {
		t.Fatalf("precondition: %d granting raises over Shock, want 2", raised)
	}

	c, _, _ := sc.s.mayRaise(sc.k, 0, shock)
	if c == nil {
		t.Fatal("mayRaise found no raise with two granting statics")
	}
	if sc.s.Stalled() || sc.s.StallReason() == StallMayRaise {
		t.Fatalf("two granting may-play raises stalled the kernel (%s), want one priced", StallName(sc.s.StallReason()))
	}
	if got := mayRaiseLife(c); got != 1 {
		t.Fatalf("the raise costs %d life, want 1 (one permission's raise; 2 is the sum of both)", got)
	}
}
