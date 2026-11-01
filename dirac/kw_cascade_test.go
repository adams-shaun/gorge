package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Cascade (CR 702.85a, dirac/kw_cascade.go): a printed-Cascade spell queues
// one native trigger per instance when cast; each exiles from the library
// top to the first nonland card of strictly lower mana value (spell MV +
// X), offers its free cast, and bottoms the exiled cards (non-found in
// library order, then the found card if it was not cast).

type cascScen struct {
	sc     *scen
	plains CardID
	c1, c2 CardID // vanilla creatures of mana value 1 and 2
	c4     CardID // vanilla creature of mana value 4 (not below a 4-drop's)
	cas    CardID // cascade 1, mana value 4
	cas2   CardID // cascade 2, mana value 4
	casX   CardID // cascade 1, {X}{1}
}

func cascTable(t *testing.T) *cascScen {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	cs := &cascScen{}
	for _, c := range da {
		if tbl.Cards[c].Name == "Plains" {
			cs.plains = c
		}
	}
	add := func(d kop.CardDef) CardID {
		d.Types, d.Power, d.Toughness = kop.TCreature, 1, 1
		tbl.Cards = append(tbl.Cards, d)
		return CardID(len(tbl.Cards) - 1)
	}
	cs.c1 = add(kop.CardDef{Name: "Test One", Cost: kop.ManaCost{Generic: 1}})
	cs.c2 = add(kop.CardDef{Name: "Test Two", Cost: kop.ManaCost{Generic: 2}})
	cs.c4 = add(kop.CardDef{Name: "Test Four", Cost: kop.ManaCost{Generic: 4}})
	cs.cas = add(kop.CardDef{Name: "Test Cascader", Cost: kop.ManaCost{Generic: 4}, Cascade: 1})
	cs.cas2 = add(kop.CardDef{Name: "Test Double Cascader", Cost: kop.ManaCost{Generic: 4}, Cascade: 2})
	cs.casX = add(kop.CardDef{Name: "Test X Cascader", Cost: kop.ManaCost{Generic: 1, X: 1}, Cascade: 1})
	cs.sc = newScenTable(t, tbl, Config{NoElide: true})
	return cs
}

// stock replaces seat 0's library top with cards (first = top); the rest of
// the library stays.
func (cs *cascScen) stock(cards ...CardID) []Obj {
	var os []Obj
	for _, c := range cards {
		o := cs.sc.obj(c, 0)
		cs.sc.s.objs[o].zone = kop.ZoneLibrary
		os = append(os, o)
	}
	for i := len(os) - 1; i >= 0; i-- {
		cs.sc.s.pl[0].libTop(os[i])
	}
	return os
}

// emptyLib leaves seat 0's library holding only cards (first = top).
func (cs *cascScen) emptyLib(cards ...CardID) []Obj {
	pl := &cs.sc.s.pl[0]
	pl.libLo = pl.libHi
	return cs.stock(cards...)
}

func (cs *cascScen) lib() []Obj {
	pl := &cs.sc.s.pl[0]
	return append([]Obj(nil), pl.lib[pl.libLo:pl.libHi]...)
}

// run casts spell and plays the stack out, answering each free-cast ask
// with accept; it returns the number of asks.
func (cs *cascScen) run(t *testing.T, spell Obj, x int, accept bool) (asks int) {
	t.Helper()
	sc := cs.sc
	sc.castCard(t, spell)
	for i := 0; i < 200; i++ {
		d := sc.s.Step()
		switch {
		case d.Kind == DkPriority && sc.s.sp == 0 && sc.s.npend == 0:
			return asks
		case d.Kind == DkPriority:
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
		case d.Kind == DkChoose && d.Sub == ChX:
			sc.chooseX(t, uint8(x))
		case d.Kind == DkChoose && d.Sub == ChGenCastFree:
			asks++
			if accept {
				sc.s.Apply(d.Options[1])
			} else {
				sc.s.Apply(d.Options[0])
			}
		default:
			sc.s.Apply(d.Options[0])
		}
	}
	t.Fatal("the stack never emptied")
	return
}

// onBattlefield counts seat 0's battlefield objects of card c.
func (cs *cascScen) onBattlefield(c CardID) (n int) {
	for i := 0; i < int(cs.sc.s.nobj); i++ {
		if ob := &cs.sc.s.objs[i]; ob.card == c && ob.zone == kop.ZoneBattlefield {
			n++
		}
	}
	return
}

// TestCascadeHitCast: lands are skipped, the first cheaper nonland is cast
// free, the skipped cards go to the bottom (the cast card is on the stack,
// not the bottom).
func TestCascadeHitCast(t *testing.T) {
	cs := cascTable(t)
	cs.sc.lands(0, cs.plains, 4)
	spell := cs.sc.hand(cs.cas, 0)
	os := cs.emptyLib(cs.plains, cs.c4, cs.c2, cs.plains)
	if asks := cs.run(t, spell, 0, true); asks != 1 {
		t.Fatalf("%d free-cast asks, want 1", asks)
	}
	if cs.onBattlefield(cs.c2) != 1 {
		t.Fatalf("the found MV-2 card was not cast")
	}
	want := []Obj{os[3], os[0], os[1]} // unexiled tail, then the skipped cards in library order
	if got := cs.lib(); len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("library %v, want %v", got, want)
	}
}

// TestCascadeHitDeclined: a declined hit goes to the bottom last.
func TestCascadeHitDeclined(t *testing.T) {
	cs := cascTable(t)
	cs.sc.lands(0, cs.plains, 4)
	spell := cs.sc.hand(cs.cas, 0)
	os := cs.emptyLib(cs.plains, cs.c1, cs.c2)
	if asks := cs.run(t, spell, 0, false); asks != 1 {
		t.Fatalf("%d free-cast asks, want 1", asks)
	}
	if cs.onBattlefield(cs.c1) != 0 {
		t.Fatalf("a declined card was cast")
	}
	want := []Obj{os[2], os[0], os[1]}
	if got := cs.lib(); len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("library %v, want %v (found card last)", got, want)
	}
}

// TestCascadeNoHit: nothing cheaper in the library: everything is exiled
// and bottomed in order, with no prompt.
func TestCascadeNoHit(t *testing.T) {
	cs := cascTable(t)
	cs.sc.lands(0, cs.plains, 4)
	spell := cs.sc.hand(cs.cas, 0)
	os := cs.emptyLib(cs.plains, cs.c4, cs.plains)
	if asks := cs.run(t, spell, 0, true); asks != 0 {
		t.Fatalf("%d free-cast asks, want none", asks)
	}
	if got := cs.lib(); len(got) != 3 || got[0] != os[0] || got[1] != os[1] || got[2] != os[2] {
		t.Fatalf("library %v, want %v", got, os)
	}
}

// TestCascadeEqualMVSkipped: "less than", not "or less": an MV-4 card under
// an MV-4 spell is skipped.
func TestCascadeEqualMVSkipped(t *testing.T) {
	cs := cascTable(t)
	cs.sc.lands(0, cs.plains, 4)
	spell := cs.sc.hand(cs.cas, 0)
	cs.emptyLib(cs.c4, cs.c1)
	if asks := cs.run(t, spell, 0, true); asks != 1 || cs.onBattlefield(cs.c4) != 0 || cs.onBattlefield(cs.c1) != 1 {
		t.Fatalf("asks %d, four-drop cast %d, one-drop cast %d: want the MV-1 card cast, the MV-4 skipped", asks, cs.onBattlefield(cs.c4), cs.onBattlefield(cs.c1))
	}
}

// TestCascadeXSpellMV: an {X}{1} spell cast for X = 3 has mana value 4 on
// the stack (CR 202.3b): the MV-2 card is below it.
func TestCascadeXSpellMV(t *testing.T) {
	cs := cascTable(t)
	cs.sc.lands(0, cs.plains, 4)
	spell := cs.sc.hand(cs.casX, 0)
	cs.emptyLib(cs.c2)
	if asks := cs.run(t, spell, 3, true); asks != 1 || cs.onBattlefield(cs.c2) != 1 {
		t.Fatalf("asks %d, c2 cast %d: want the MV-2 card found under X = 3", asks, cs.onBattlefield(cs.c2))
	}
	// X = 0: mana value 1, so the MV-2 card is not below it
	cs = cascTable(t)
	cs.sc.lands(0, cs.plains, 1)
	spell = cs.sc.hand(cs.casX, 0)
	cs.emptyLib(cs.c2)
	if asks := cs.run(t, spell, 0, true); asks != 0 {
		t.Fatalf("asks %d under X = 0, want none", asks)
	}
}

// TestCascadeTwoInstances: two printed instances resolve as two triggers
// (CR 702.85c), each with its own free-cast ask.
func TestCascadeTwoInstances(t *testing.T) {
	cs := cascTable(t)
	cs.sc.lands(0, cs.plains, 4)
	spell := cs.sc.hand(cs.cas2, 0)
	cs.emptyLib(cs.c1, cs.c2)
	if asks := cs.run(t, spell, 0, true); asks != 2 || cs.onBattlefield(cs.c1) != 1 || cs.onBattlefield(cs.c2) != 1 {
		t.Fatalf("asks %d, c1 %d, c2 %d: want two asks, both cast", asks, cs.onBattlefield(cs.c1), cs.onBattlefield(cs.c2))
	}
}
