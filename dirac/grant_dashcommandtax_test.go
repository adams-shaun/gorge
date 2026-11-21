package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestGrantPlayCostDashCommandTax: the grant search prices a commander's
// dashed cast from the command zone as its dash cost plus the commander tax
// (CR 118.9d, 903.8), as offerDashCommand and beginCast do; a hand dashed
// cast of the same card (no tax) stays the bare dash cost.
func TestGrantPlayCostDashCommandTax(t *testing.T) {
	tbl, card := commanderTable(0)
	dc := kop.ManaCost{Generic: 1}
	dc.Pips[kop.R] = 1
	tbl.Cards[card].HasDash, tbl.Cards[card].DashCost = true, dc
	sc := newCommanderScen(t, tbl, card, true)
	s, k := sc.s, sc.k
	o := s.pl[0].cmd
	if o == NoObj || s.objs[o].zone != kop.ZoneCommand {
		t.Fatalf("precondition: commander %d not in the command zone (zone %v)", o, s.objs[o].zone)
	}
	a := MakeAction(AkCast, TCode(o), castDashTgt, 0)
	price := func() kop.ManaCost {
		t.Helper()
		cost, _, _, ok := s.grantPlayCost(k, 0, a, true)
		if !ok {
			t.Fatal("grantPlayCost refused the dashed cast")
		}
		return cost
	}
	if got := price(); got != dc {
		t.Fatalf("untaxed dashed command-zone cast priced %+v, want the dash cost %+v", got, dc)
	}
	s.pl[0].cmdCast = 1 // one earlier cast: the tax is {2}
	want := dc
	addCommanderTax(&want, 2)
	if s.commanderTax(0, o) != 2 || want == dc {
		t.Fatalf("precondition: tax %d, taxed cost %+v", s.commanderTax(0, o), want)
	}
	if got := price(); got != want {
		t.Fatalf("taxed dashed command-zone cast priced %+v, want %+v", got, want)
	}
	// the same card dashed from the hand pays no tax
	h := sc.hand(card, 0)
	a = MakeAction(AkCast, TCode(h), castDashTgt, 0)
	if got := price(); got != dc {
		t.Fatalf("hand dashed cast priced %+v, want the bare dash cost %+v", got, dc)
	}
}
