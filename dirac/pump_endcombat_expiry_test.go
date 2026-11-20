package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// pumpEOCScen is a scenario with a vanilla 2/2 and three override rows: a
// "can't block" grant and a +2/+0 pump, both Duration$ UntilEndOfCombat
// (kop.Override.UntilEndCombat, applied Permanent), and a plain +1/+0 pump
// that has no end-of-combat flag.
type pumpEOCScen struct {
	*scen
	bear                  CardID
	noBlock, pump2, plain int
}

func newPumpEOCScen(t *testing.T) *pumpEOCScen {
	t.Helper()
	var f pumpEOCScen
	sc, _ := newBLBScenTweak(t, []cardset.Entry{{Name: "Plains", Count: 20}}, func(tbl *kop.Table, _ map[string]CardID) {
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2})
		f.bear = CardID(len(tbl.Cards) - 1)
		f.noBlock = len(tbl.Overrides)
		tbl.Overrides = append(tbl.Overrides,
			kop.Override{AddKw: kop.KwCantBlock, UntilEndCombat: true},
			kop.Override{PTDelta: true, ValPT: true, PA: kop.Const(2), TA: kop.Const(0), UntilEndCombat: true},
			kop.Override{PTDelta: true, ValPT: true, PA: kop.Const(1), TA: kop.Const(0)})
		f.pump2, f.plain = f.noBlock+1, f.noBlock+2
	})
	f.scen = sc
	return &f
}

func (f *pumpEOCScen) apply(sl Slot, idx int) {
	f.s.applyOverrideFl(f.k, sl, idx, f.bear, f.s.ovDurFl(kop.DurPermanent))
}

func (f *pumpEOCScen) cantBlock(sl Slot) bool { return f.s.kw(f.k, sl)&kop.KwCantBlock != 0 }

func (f *pumpEOCScen) power(sl Slot) int { p, _ := f.s.pt(f.k, sl); return p }

// nrec counts the override records on the fx pool.
func (f *pumpEOCScen) nrec() int {
	n := 0
	for i := 0; i < int(f.s.g.nfx); i++ {
		if f.s.g.fx[i].kind == fxOverride {
			n++
		}
	}
	return n
}

// TestPumpEndCombatDrop: the records of a Pump Duration$ UntilEndOfCombat are
// live from creation and go away on the way out of the end-of-combat step; a
// record without the class flag stays.
func TestPumpEndCombatDrop(t *testing.T) {
	setup := func(t *testing.T) (*pumpEOCScen, Slot) {
		f := newPumpEOCScen(t)
		sl := f.slot(f.perm(f.bear, 0))
		f.s.step = kop.StepBlockers
		f.apply(sl, f.noBlock)
		f.apply(sl, f.pump2)
		f.apply(sl, f.plain)
		if !f.cantBlock(sl) || f.power(sl) != 5 || f.nrec() != 3 {
			t.Fatalf("precondition: can't block %v, power %d, %d records; want true, 5, 3", f.cantBlock(sl), f.power(sl), f.nrec())
		}
		return f, sl
	}
	t.Run("dropped at end of combat", func(t *testing.T) {
		f, sl := setup(t)
		f.s.step = kop.StepEndCombat
		f.s.advance(f.k)
		if f.cantBlock(sl) || f.power(sl) != 3 || f.nrec() != 1 {
			t.Fatalf("after end of combat: can't block %v, power %d, %d records; want false, 3 (the plain +1), 1", f.cantBlock(sl), f.power(sl), f.nrec())
		}
	})
	t.Run("mutation", func(t *testing.T) {
		DebugNoOvEndCombat = true
		defer func() { DebugNoOvEndCombat = false }()
		f, sl := setup(t)
		f.s.step = kop.StepEndCombat
		f.s.advance(f.k)
		if !f.cantBlock(sl) || f.power(sl) != 5 || f.nrec() != 3 {
			t.Fatalf("under DebugNoOvEndCombat the records were dropped: can't block %v, power %d, %d records", f.cantBlock(sl), f.power(sl), f.nrec())
		}
	})
}

// TestPumpEndCombatCleanup: a record made after combat (the postcombat main
// phase) or in a turn whose combat was skipped is dropped by cleanup (gorge's
// cleanup backstop), and one dropped at end of combat does not come back in a
// second combat.
func TestPumpEndCombatCleanup(t *testing.T) {
	t.Run("made in the postcombat main phase", func(t *testing.T) {
		f := newPumpEOCScen(t)
		sl := f.slot(f.perm(f.bear, 0))
		f.s.step = kop.StepMain2
		f.apply(sl, f.pump2)
		if f.power(sl) != 4 || f.nrec() != 1 {
			t.Fatalf("precondition: power %d, %d records; want 4, 1", f.power(sl), f.nrec())
		}
		f.s.cleanupGen(f.k)
		if f.power(sl) != 2 || f.nrec() != 0 {
			t.Fatalf("after cleanup: power %d, %d records; want 2, 0", f.power(sl), f.nrec())
		}
	})
	t.Run("combat skipped", func(t *testing.T) {
		f := newPumpEOCScen(t)
		sl := f.slot(f.perm(f.bear, 0))
		f.s.step = kop.StepMain1
		f.apply(sl, f.noBlock)
		f.apply(sl, f.plain)
		// no StepEndCombat pass: the records live through the turn
		f.s.step = kop.StepMain2
		f.s.advance(f.k)
		if !f.cantBlock(sl) || f.nrec() != 2 {
			t.Fatalf("precondition: the record was dropped before cleanup: can't block %v, %d records", f.cantBlock(sl), f.nrec())
		}
		f.s.cleanupGen(f.k)
		if f.cantBlock(sl) || f.nrec() != 1 || f.power(sl) != 3 {
			t.Fatalf("after cleanup: can't block %v, %d records, power %d; want false, 1 (the plain record), 3", f.cantBlock(sl), f.nrec(), f.power(sl))
		}
	})
	t.Run("an extra combat does not revive it", func(t *testing.T) {
		f := newPumpEOCScen(t)
		sl := f.slot(f.perm(f.bear, 0))
		f.apply(sl, f.pump2)
		if f.power(sl) != 4 {
			t.Fatalf("precondition: power %d, want 4", f.power(sl))
		}
		for pass := 0; pass < 2; pass++ {
			f.s.step = kop.StepEndCombat
			f.s.advance(f.k)
			if f.power(sl) != 2 || f.nrec() != 0 {
				t.Fatalf("end-of-combat pass %d: power %d, %d records; want 2, 0", pass+1, f.power(sl), f.nrec())
			}
		}
	})
}
