package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// kwBandingScen is a scenario with a vanilla 2/2 and one override row: the
// Battering Ram-shaped grant, Banding beside UntilEndCombat (a Pump KW$
// Banding Duration$ UntilEndOfCombat).
type kwBandingScen struct {
	*scen
	ram  CardID
	band int
}

func newKwBandingScen(t *testing.T) *kwBandingScen {
	t.Helper()
	var f kwBandingScen
	sc, _ := newBLBScenTweak(t, []cardset.Entry{{Name: "Plains", Count: 20}}, func(tbl *kop.Table, _ map[string]CardID) {
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Ram", Types: kop.TCreature, Power: 1, Toughness: 1})
		f.ram = CardID(len(tbl.Cards) - 1)
		f.band = len(tbl.Overrides)
		tbl.Overrides = append(tbl.Overrides, kop.Override{Banding: true, UntilEndCombat: true})
	})
	f.scen = sc
	return &f
}

// TestBatteringRamBanding: a granted banding (kop.Override.Banding) is live
// from the record's application and gone with the record: dropped on the way
// out of the end-of-combat step (static_ovendcombat.go) and by cleanup. The
// kernel's banding rules model no band formation; the read is the record
// predicate the combat reads (the CR 702.22c ask) will call.
func TestBatteringRamBanding(t *testing.T) {
	setup := func(t *testing.T) (*kwBandingScen, Slot) {
		f := newKwBandingScen(t)
		sl := f.slot(f.perm(f.ram, 0))
		f.s.step = kop.StepBlockers
		f.s.applyOverrideFl(f.k, sl, f.band, f.ram, f.s.ovDurFl(kop.DurPermanent))
		if !f.s.hasBanding(f.k, sl) {
			t.Fatal("precondition: the applied record does not read as banding")
		}
		return f, sl
	}
	t.Run("dropped at end of combat", func(t *testing.T) {
		f, sl := setup(t)
		f.s.step = kop.StepEndCombat
		f.s.advance(f.k)
		if f.s.hasBanding(f.k, sl) {
			t.Fatal("the banding record survived the end-of-combat step")
		}
	})
	t.Run("dropped by cleanup", func(t *testing.T) {
		f, sl := setup(t)
		f.s.step = kop.StepMain2
		f.s.cleanupGen(f.k)
		if f.s.hasBanding(f.k, sl) {
			t.Fatal("the banding record survived cleanup")
		}
	})
}
