package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// painOnlyTombs: seat 0 holds tombs Ancient Tombs ({T}: Add {C}{C}. Deals 2
// damage to you), swamps Swamps and, with monument, Forsaken Monument (one more
// {C} on each tap for {C}), at life, with Sire of Seven Deaths ({7}) in hand.
func painOnlyTombs(t *testing.T, tombs, swamps int, monument bool, life int16) (sc *scen, objs []Obj, tomb []Slot, sire Obj) {
	t.Helper()
	deck := []cardset.Entry{
		{Name: "Ancient Tomb", Count: 2},
		{Name: "Sire of Seven Deaths", Count: 1},
		{Name: "Swamp", Count: 4},
	}
	if monument {
		// the table carries a TapExtra only when the deck does
		deck = append(deck, cardset.Entry{Name: "Forsaken Monument", Count: 1})
	}
	tbl, da, _, err := cardsettest.Set(t, "hosted-cmdr").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc = newScenTableLib(t, tbl, id["Swamp"], Config{NoElide: true, GorgeAutopaySurface: true})
	for i := 0; i < tombs; i++ {
		objs = append(objs, sc.perm(id["Ancient Tomb"], 0))
	}
	if monument {
		sc.perm(id["Forsaken Monument"], 0)
	}
	for i := 0; i < swamps; i++ {
		sc.perm(id["Swamp"], 0)
	}
	for _, o := range objs {
		tomb = append(tomb, sc.slot(o))
	}
	sc.s.pl[0].life = life
	sire = sc.hand(id["Sire of Seven Deaths"], 0)
	pc := sc.s.manaPCGen(sc.k, tomb[0])
	if !pc.painOnly || pc.painCol != kop.ColC || pc.painN != 2 || !sc.k.painMana || sc.k.gen.anyTapExtra != monument {
		t.Fatalf("precondition: Ancient Tomb painOnly %v painCol %b painN %d, pain mana %v, tap extra %v (want %v)",
			pc.painOnly, pc.painCol, pc.painN, sc.k.painMana, sc.k.gen.anyTapExtra, monument)
	}
	return sc, objs, tomb, sire
}

// maxGeneric is the largest all-generic cost a pays.
func maxGeneric(a *avail) int {
	n := 0
	for n < 40 {
		cost := kop.ManaCost{Generic: uint8(n + 1)}
		if !a.affords(&cost, 0, 0) {
			break
		}
		n++
	}
	return n
}

func (sc *scen) census(t *testing.T) avail {
	t.Helper()
	var a avail
	sc.s.availFor(sc.k, 0, &a)
	if sc.s.Stalled() {
		t.Fatalf("the census stalled: %s", StallName(sc.s.StallReason()))
	}
	return a
}

// TestTapExtraPainOnlyRemoves: a cost that taps the pain-only source takes its
// whole tap out of the census, the TapExtra {C} included (CR 601.2g-h): no
// phantom {C} stays behind to pay with, under the life guard or clear of it.
func TestTapExtraPainOnlyRemoves(t *testing.T) {
	for _, c := range []struct {
		name          string
		tombs, swamps int
		life          int16
		guard         bool
		before, after int // largest generic payable before and after the first Tomb is tapped
	}{
		{"lone", 1, 0, 20, false, 3, 0},
		{"swamps", 1, 4, 20, false, 7, 4},
		{"two guarded", 2, 0, 3, true, 3, 3},
		{"guarded swamps", 2, 4, 3, true, 7, 7},
		{"lone guarded", 1, 4, 3, false, 7, 4},
	} {
		t.Run(c.name, func(t *testing.T) {
			sc, _, tomb, _ := painOnlyTombs(t, c.tombs, c.swamps, true, c.life)
			a := sc.census(t)
			if a.pain.on != c.guard {
				t.Fatalf("precondition: life guard binds %v, want %v", a.pain.on, c.guard)
			}
			if got := maxGeneric(&a); got != c.before {
				t.Fatalf("payable before = %d, want %d", got, c.before)
			}
			sc.s.removeSrc(sc.k, &a, tomb[0], sc.s.manaPCGen(sc.k, tomb[0]))
			if got := maxGeneric(&a); got != c.after {
				t.Fatalf("payable after the first Tomb is tapped = %d, want %d", got, c.after)
			}
			for _, sl := range tomb[1:] {
				sc.s.removeSrc(sc.k, &a, sl, sc.s.manaPCGen(sc.k, sl))
			}
			if got := maxGeneric(&a); got != c.swamps {
				t.Fatalf("payable after every Tomb is tapped = %d, want the %d Swamps alone", got, c.swamps)
			}
			if got := a.censusTotal(); got != c.swamps {
				t.Fatalf("census total after every Tomb is tapped = %d, want %d", got, c.swamps)
			}
		})
	}
}

// TestPainOnlyGuardGatesTheWholeTap: where the summed damage would be lethal, a
// pain-only source's mana, its TapExtra {C} included, counts only for the taps
// the guard allows (CR 120.3, 601.2g): at life 3 one of two Ancient Tombs may
// be tapped, so the census pays one tap's mana and no more.
func TestPainOnlyGuardGatesTheWholeTap(t *testing.T) {
	for _, c := range []struct {
		name     string
		monument bool
		life     int16
		guard    bool
		pay      int // largest generic payable: the mana of the allowed taps
		total    int // censusTotal: every counted Tomb's whole tap
	}{
		{"monument life 3", true, 3, true, 3, 6},
		{"monument life 4", true, 4, true, 3, 6},
		{"monument life 5", true, 5, false, 6, 6},
		{"monument life 20", true, 20, false, 6, 6},
		{"plain life 3", false, 3, true, 2, 4},
		{"plain life 20", false, 20, false, 4, 4},
		{"monument life 2", true, 2, false, 0, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			sc, _, _, sire := painOnlyTombs(t, 2, 0, c.monument, c.life)
			a := sc.census(t)
			if a.pain.on != c.guard {
				t.Fatalf("precondition: life guard binds %v, want %v", a.pain.on, c.guard)
			}
			if got := maxGeneric(&a); got != c.pay {
				t.Fatalf("payable = %d, want %d", got, c.pay)
			}
			// the census total counts every counted source's whole tap, the
			// guard holding the plan only (an {X} ceiling divides it)
			if got := a.censusTotal(); got != c.total {
				t.Fatalf("census total = %d, want %d", got, c.total)
			}
			if offersCast(sc.s, sire) {
				t.Fatal("the seven-mana cast is offered off two Ancient Tombs")
			}
		})
	}
}

// TestTapExtraPainOnlyKeptTarget: a pain-only source a plan keeps (a target the
// plan taps and does not consume) is put back into the census as its whole tap,
// the TapExtra {C} included (CR 601.2c, 601.2g).
func TestTapExtraPainOnlyKeptTarget(t *testing.T) {
	for _, c := range []struct {
		life int16
		pay  int
	}{{20, 6}, {3, 3}} {
		sc, objs, _, _ := painOnlyTombs(t, 2, 0, true, c.life)
		var a avail
		sc.s.availSansObj(sc.k, 0, objs[0], &a, false)
		if got := maxGeneric(&a); got != c.pay {
			t.Errorf("life %d: payable with one Tomb kept = %d, want %d", c.life, got, c.pay)
		}
	}
}
