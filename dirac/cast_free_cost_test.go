package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// freeCostTable is vanilla's Test Charm as a {0} spell with one non-mana
// additional cost (flags, n) and no target: a free cast of it begun without
// the offer's cost census (Wishing Well's Play) reaches the cost stage with
// the cost's picks or life still to pay.
func freeCostTable(flags kop.CostFlags, n uint8) *kop.Table {
	t := *v.Table
	t.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	cd := &t.Cards[v.TestCharm]
	cd.Cost = kop.ManaCost{}
	a := cd.Abilities[0]
	a.NModes, a.ModesPick, a.ModesFew = 0, 0, 0
	a.Targets = [kop.MaxTargets]kop.TargetSpec{}
	a.NTargets = 0
	a.TgtModes = [kop.MaxTargets]uint8{}
	a.Cost = kop.Cost{Flags: flags, N: n}
	a.Prog = []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}
	cd.Abilities = []kop.Ability{a}
	return &t
}

// TestFreeCastUnpayableLifeReverses: a free cast with a fixed PayLife<2> and
// one life would pay into negative life (loseLife does not clamp), where
// gorge reverses the cast at its cost stage (CR 601.2h, 733.1). The cast has
// no target, so no target-menu census precedes the cost stage.
func TestFreeCastUnpayableLifeReverses(t *testing.T) {
	t.Parallel()
	sc := newScenTable(t, freeCostTable(kop.CostPayLife, 2), Config{NoElide: true})
	fc := sc.hand(v.TestCharm, 0)
	sc.s.pl[0].life = 1
	sc.s.beginCast(sc.k, fc, castFree, 0)
	sc.s.Step()
	if sc.s.over || sc.s.pl[0].life != 1 {
		t.Fatalf("life %d over %v, want the cast reversed with 1 life", sc.s.pl[0].life, sc.s.over)
	}
	if z := sc.s.objs[fc].zone; z != kop.ZoneHand {
		t.Fatalf("spell zone %d, want the hand after the cost reversal", z)
	}
	if sc.s.sp != 0 {
		t.Fatalf("stack holds %d entries, want the reversed cast gone", sc.s.sp)
	}
}

// TestFreeCastEmptyHandDiscardReverses: the same for an unpayable discard
// part: the hand holds only the card being cast, so the discard pick has no
// candidate and the cast reverses there (gorge's discardAsk aborts, CR 733.1)
// instead of posing a decision with no options.
func TestFreeCastEmptyHandDiscardReverses(t *testing.T) {
	t.Parallel()
	sc := newScenTable(t, freeCostTable(kop.CostDiscard, 1), Config{NoElide: true})
	fc := sc.hand(v.TestCharm, 0)
	sc.s.beginCast(sc.k, fc, castFree, 0)
	sc.s.Step()
	if z := sc.s.objs[fc].zone; z != kop.ZoneHand {
		t.Fatalf("spell zone %d, want the hand after the discard reversal", z)
	}
	if sc.s.sp != 0 {
		t.Fatalf("stack holds %d entries, want the reversed cast gone", sc.s.sp)
	}
}
