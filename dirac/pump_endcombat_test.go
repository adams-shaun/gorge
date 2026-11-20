package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// endCombatScen is the FDN table (a built one, so the generated-effect prep
// is live) plus one Override row (an AddKw Reach record, marked EndCombat
// when rowEC) and an Ambush Wolf of seat 0 to carry it.
func endCombatScen(t *testing.T, rowEC bool) (*scen, Obj, int) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Ambush Wolf", Count: 2}, {Name: "Plains", Count: 10}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	var wolf, plains CardID
	for _, c := range da {
		switch tbl.Cards[c].Name {
		case "Ambush Wolf":
			wolf = c
		case "Plains":
			plains = c
		}
	}
	t2 := *tbl
	t2.Overrides = append(append([]kop.Override(nil), tbl.Overrides...), kop.Override{AddKw: kop.KwReach, EndCombat: rowEC})
	idx := len(t2.Overrides) - 1
	sc := newScenTableLib(t, &t2, plains, Config{NoElide: true})
	return sc, sc.perm(wolf, 0), idx
}

func (sc *scen) ovCount(sl Slot) int {
	n := 0
	sc.s.eachOvRec(sl, func(ovRec) { n++ })
	return n
}

// TestPumpEndCombatDropsAtLeave: an EndCombat override record is live from its
// application, survives into the end-of-combat step, and is dropped on the
// transition that leaves it (gorge expireUntilEndOfCombat, CR 511.2); a
// plain record beside it stays. DebugNoPumpEndCombat keeps it.
func TestPumpEndCombatDropsAtLeave(t *testing.T) {
	run := func(t *testing.T) (*scen, Slot) {
		sc, bear, idx := endCombatScen(t, true)
		sl := sc.slot(bear)
		sc.s.applyOverrideFl(sc.k, sl, idx, 0, 0)
		if sc.k.gen == nil || !sc.k.gen.anyEndCombatOv {
			t.Fatal("precondition: the table scan did not set anyEndCombatOv")
		}
		if sc.ovCount(sl) != 1 {
			t.Fatalf("precondition: %d records after the pump, want 1", sc.ovCount(sl))
		}
		sc.s.step = kop.StepEndCombat
		sc.s.advance(sc.k)
		return sc, sl
	}
	t.Run("dropped", func(t *testing.T) {
		sc, sl := run(t)
		if n := sc.ovCount(sl); n != 0 {
			t.Fatalf("%d records after leaving end of combat, want 0", n)
		}
	})
	t.Run("mutation", func(t *testing.T) {
		DebugNoPumpEndCombat = true
		defer func() { DebugNoPumpEndCombat = false }()
		sc, sl := run(t)
		if n := sc.ovCount(sl); n != 1 {
			t.Fatalf("%d records under DebugNoPumpEndCombat, want the record kept", n)
		}
	})
}

// TestPumpEndCombatSkippedCombatLastsToCleanup: with no end-of-combat step
// the record stays live (a pump made after combat likewise), and the cleanup
// backstop drops it (gorge EndOfTurnCleanup's untilEndOfCombat).
func TestPumpEndCombatSkippedCombatLastsToCleanup(t *testing.T) {
	sc, bear, idx := endCombatScen(t, true)
	sl := sc.slot(bear)
	s := sc.s
	s.step = kop.StepMain2
	s.applyOverrideFl(sc.k, sl, idx, 0, 0)
	s.advance(sc.k) // main 2 -> end step: no end-of-combat transition
	if n := sc.ovCount(sl); n != 1 {
		t.Fatalf("%d records after main 2, want the record live", n)
	}
	s.cleanupGen(sc.k)
	if n := sc.ovCount(sl); n != 0 {
		t.Fatalf("%d records after cleanup, want 0", n)
	}
	// a second record on a row without the class is untouched by both sweeps
	sc2, bear2, idx2 := endCombatScen(t, false)
	sl2 := sc2.slot(bear2)
	sc2.s.applyOverrideFl(sc2.k, sl2, idx2, 0, 0)
	sc2.s.step = kop.StepEndCombat
	sc2.s.advance(sc2.k)
	if n := sc2.ovCount(sl2); n != 1 {
		t.Fatalf("a plain permanent record: %d after end of combat, want 1", n)
	}
}
