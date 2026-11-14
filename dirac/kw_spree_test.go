package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// spree2Table is vanilla's Test Charm as a {0} Spree spell: each of its two
// modes adds {1}, and both mode targets are "up to one", so an unpayable
// combination reaches payment instead of aborting at a required target.
func spree2Table() *kop.Table {
	t := *v.Table
	t.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	cd := &t.Cards[v.TestCharm]
	cd.Cost = kop.ManaCost{}
	cd.HasModeCost = true
	cd.ModeCost = [4]kop.ManaCost{{Generic: 1}, {Generic: 1}}
	a := cd.Abilities[0]
	a.NModes, a.ModesPick, a.ModesFew = 2, 2, 0
	anyCreature := v.Table.Cards[v.PressurePoint].Abilities[0].Targets[0].Filter
	a.Targets = [kop.MaxTargets]kop.TargetSpec{
		{Kind: kop.TgtObject, Filter: anyCreature, UpTo: true},
		{Kind: kop.TgtObject, Filter: anyCreature, UpTo: true},
	}
	a.NTargets = 2
	a.TgtModes = [kop.MaxTargets]uint8{1, 2}
	cd.Abilities = []kop.Ability{a}
	return &t
}

// An unpayable Spree combination runs gor's CR 601.2g window before it
// reverses: the land is tapped, its mana floats, the card returns to hand
// and nothing resolves (CR 733.1). The old finishBuild reversed without the
// window, leaving the land untapped and the pool empty.
func TestSpreeWindowAbort(t *testing.T) {
	t.Parallel()
	sc := newScenTable(t, spree2Table(), Config{NoElide: true})
	land := sc.perm(v.Forest, 0)
	fc := sc.hand(v.TestCharm, 0)
	sc.do(t, AkCast, TCode(fc), scAny)
	// the lowering surface's witness prices the cast with no target chosen
	// and can leave the mode-cost remainder to the payment window (bflLowered
	// makes bsMana leave the pool alone, as the OTJ repro does); the window
	// below must still tap, float and reverse.
	sc.s.bld().bfl |= bflLowered
	charmnumPickMode(t, sc, 0)
	charmnumPickMode(t, sc, 1)
	sc.s.Step()
	if z := sc.s.objs[fc].zone; z != kop.ZoneHand {
		t.Fatalf("spell zone %d, want the hand after the reversal", z)
	}
	if sc.s.perms[sc.slot(land)].flags&pfTapped == 0 {
		t.Fatalf("the window left the land untapped")
	}
	if got := sc.s.pl[0].pool[kop.G]; got != 1 {
		t.Fatalf("pool green %d, want 1 floating after the window", got)
	}
	if sc.s.sp != 0 {
		t.Fatalf("stack holds %d entries, want the reversed cast gone", sc.s.sp)
	}
}

// The combined target ask runs gor's pre-target gate: with the pool short of
// the total and no untapped source left, the cast dies before any target
// (117 of 256 local seeds without it); one untapped source or a paying pool
// keeps it alive.
func TestSpreeAskDead(t *testing.T) {
	t.Parallel()
	sc := newScenTable(t, spree2Table(), Config{NoElide: true})
	land := sc.perm(v.Forest, 0)
	fc := sc.hand(v.TestCharm, 0)
	sc.do(t, AkCast, TCode(fc), scAny)
	charmnumPickMode(t, sc, 0)
	charmnumPickMode(t, sc, 1)
	if sc.s.charmTargetAskDead(sc.k) {
		t.Fatalf("the untapped Forest keeps the cast alive")
	}
	sc.s.pl[0].pool[kop.G] = 2
	if sc.s.charmTargetAskDead(sc.k) {
		t.Fatalf("the pool pays the total: want the cast alive")
	}
	sc.s.pl[0].pool[kop.G] = 0
	sc.s.perms[sc.slot(land)].flags |= pfTapped
	if !sc.s.charmTargetAskDead(sc.k) {
		t.Fatalf("pool short and no untapped source: want the cast dead")
	}
	var d Decision
	if sc.s.charmTargetsStep(sc.k, &d) {
		t.Fatalf("the dead combined step posed a decision")
	}
	if z := sc.s.objs[fc].zone; z != kop.ZoneHand {
		t.Fatalf("spell zone %d, want the hand after the pre-target reversal", z)
	}
}
