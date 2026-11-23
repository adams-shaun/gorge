package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// The lockstep pair (std-blb 184, decision 254) that gorge's offer accepts
// and the kernel refuses: Feed the Cycle's forage either-or part is payable
// (three graveyard cards), and the printed {1}{B} is fundable through the
// board's activations -- a {G} floats into Hidden Grotto's {1},{T} and the
// Grotto taps for the {B} (or Thornvault Forager's {T}, Forage: Add two mana
// covers it alone). The kernel's cast-offer search must see that funding too
// (CR 601.2f-h: the mana part and the either-or part price together).
func TestFeedTheCycleChainedManaOffer(t *testing.T) {
	sc, id := giftScen(t, "Feed the Cycle", "Hidden Grotto", "Thornvault Forager", "Swamp", "Forest", "Jolly Gerbils")
	for i := 0; i < 3; i++ {
		sc.perm(id["Forest"], 0)
	}
	sc.perm(id["Hidden Grotto"], 0)
	sc.perm(id["Thornvault Forager"], 0)
	sc.grave(id["Forest"], 0, 3) // the forage arm's exile fodder
	sc.perm(id["Jolly Gerbils"], 1)
	ftc := sc.hand(id["Feed the Cycle"], 0)
	d := sc.s.Step()
	if d.Kind != DkPriority {
		t.Fatalf("decision kind %d, want priority (opts %v)", d.Kind, d.Options[:d.N])
	}
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && Obj(a.Source()) == ftc {
			return
		}
	}
	t.Fatalf("Feed the Cycle not offered: forage is payable (3 grave cards) and {1}{B} floats from the board's activations")
}

// The same shape funded by a plain Swamp: the control that pins the chain, not
// the part payability, as the difference.
func TestFeedTheCycleSwampOffer(t *testing.T) {
	sc, id := giftScen(t, "Feed the Cycle", "Swamp", "Forest", "Jolly Gerbils")
	for i := 0; i < 3; i++ {
		sc.perm(id["Swamp"], 0)
	}
	sc.grave(id["Forest"], 0, 3)
	sc.perm(id["Jolly Gerbils"], 1)
	ftc := sc.hand(id["Feed the Cycle"], 0)
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && Obj(a.Source()) == ftc {
			return
		}
	}
	t.Fatalf("Feed the Cycle not offered over three untapped Swamps (forage payable, {1}{B} floats)")
}

// TestFeedTheCycleChainedGraveShort keeps the negative arm honest: one
// graveyard card short of the forage, and no black beyond the Grotto chain's
// -- the cast stays unoffered (dirac's act_forage_test.go TestForageNotOffered
// is the same negative arm over plain Swamps).
func TestFeedTheCycleChainedGraveShort(t *testing.T) {
	sc, id := giftScen(t, "Feed the Cycle", "Hidden Grotto", "Thornvault Forager", "Swamp", "Forest", "Jolly Gerbils")
	for i := 0; i < 3; i++ {
		sc.perm(id["Forest"], 0)
	}
	sc.perm(id["Hidden Grotto"], 0)
	sc.perm(id["Thornvault Forager"], 0)
	sc.grave(id["Forest"], 0, 2) // one short of the forage
	sc.perm(id["Jolly Gerbils"], 1)
	ftc := sc.hand(id["Feed the Cycle"], 0)
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && Obj(a.Source()) == ftc {
			t.Fatal("Feed the Cycle offered with neither forage nor {B} payable")
		}
	}
}

// Each funding arm in isolation: {1}{B} through the Grotto's {1},{T} chain
// alone, and through Thornvault's {T}, Forage: Add two mana alone.
func TestFeedTheCycleGrottoChainOffer(t *testing.T) {
	sc, id := giftScen(t, "Feed the Cycle", "Hidden Grotto", "Swamp", "Forest", "Jolly Gerbils")
	for i := 0; i < 3; i++ {
		sc.perm(id["Forest"], 0)
	}
	sc.perm(id["Hidden Grotto"], 0)
	sc.grave(id["Forest"], 0, 3)
	sc.perm(id["Jolly Gerbils"], 1)
	ftc := sc.hand(id["Feed the Cycle"], 0)
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && Obj(a.Source()) == ftc {
			return
		}
	}
	t.Fatalf("Feed the Cycle not offered over the Grotto {1} chain alone")
}

func TestFeedTheCycleThornvaultOffer(t *testing.T) {
	sc, id := giftScen(t, "Feed the Cycle", "Thornvault Forager", "Swamp", "Forest", "Jolly Gerbils")
	for i := 0; i < 3; i++ {
		sc.perm(id["Forest"], 0)
	}
	sc.perm(id["Thornvault Forager"], 0)
	sc.grave(id["Forest"], 0, 3)
	sc.perm(id["Jolly Gerbils"], 1)
	ftc := sc.hand(id["Feed the Cycle"], 0)
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && Obj(a.Source()) == ftc {
			return
		}
	}
	t.Fatalf("Feed the Cycle not offered over Thornvault's {T}, Forage two-mana activation alone")
}

// forageFiltScen is the foragefilter fixture pool's scenario of the named cards (a Prophetic
// Prism is no BLB card) with Plains as the library card: Feed the Cycle's
// board beside a bare {1},{T} filter, which adds no mana of its own.
func forageFiltScen(t *testing.T, names ...string) (*scen, map[string]CardID) {
	t.Helper()
	set := cardsettest.Set(t, "foragefilter")
	var deck []cardset.Entry
	for _, n := range append(names, "Plains") {
		if _, ok := set.Lookup(n); !ok {
			t.Fatalf("%s is not in the foragefilter pool", n)
		}
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	return newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true}), id
}

// feedTheCycleOffered steps to the priority decision and reports whether it
// offers the cast of ftc.
func feedTheCycleOffered(t *testing.T, sc *scen, ftc Obj) bool {
	t.Helper()
	d := sc.s.Step()
	if d.Kind != DkPriority {
		t.Fatalf("decision kind %d, want priority (opts %v)", d.Kind, d.Options[:d.N])
	}
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && Obj(a.Source()) == ftc {
			return true
		}
	}
	return false
}

// The forage step of the filter search is observable only where the forage's
// exile decides the cast. Thornvault Forager's {T}, Forage: Add two mana beside
// a Prophetic Prism ({1},{T}: Add one mana of any color, a filter that nets no
// mana) funds two mana at most. Exiling three graveyard cards to make them
// leaves the cast's own forage arm unpayable (the graveyard holds none), so
// Feed the Cycle needs {1}{B}{B}: unaffordable -- the cast stays unoffered
// (CR 601.2g-h, 701.61a). Without the forage replay the kernel would add the
// two mana and still forage again for the cast, offering it.
func TestFeedTheCycleForageStepExilesTheFodder(t *testing.T) {
	sc, id := forageFiltScen(t, "Feed the Cycle", "Thornvault Forager", "Prophetic Prism", "Jolly Gerbils")
	sc.perm(id["Thornvault Forager"], 0)
	sc.perm(id["Prophetic Prism"], 0)
	sc.grave(id["Plains"], 0, 3)
	sc.perm(id["Jolly Gerbils"], 1)
	ftc := sc.hand(id["Feed the Cycle"], 0)
	if feedTheCycleOffered(t, sc, ftc) {
		t.Fatal("Feed the Cycle offered: Thornvault's forage exiles the three cards the cast's forage needs, leaving two mana for {1}{B}{B}")
	}
}

// The positive pair: six graveyard cards pay both forages (Thornvault's exile,
// then the cast's), the two mana fund {1}{B}, and the Prism's filter changes no
// count -- offered.
func TestFeedTheCycleForageStepLeavesEnoughFodder(t *testing.T) {
	sc, id := forageFiltScen(t, "Feed the Cycle", "Thornvault Forager", "Prophetic Prism", "Jolly Gerbils")
	sc.perm(id["Thornvault Forager"], 0)
	sc.perm(id["Prophetic Prism"], 0)
	sc.grave(id["Plains"], 0, 6)
	sc.perm(id["Jolly Gerbils"], 1)
	ftc := sc.hand(id["Feed the Cycle"], 0)
	if !feedTheCycleOffered(t, sc, ftc) {
		t.Fatal("Feed the Cycle not offered: six graveyard cards pay both forages and two mana fund {1}{B}")
	}
}
