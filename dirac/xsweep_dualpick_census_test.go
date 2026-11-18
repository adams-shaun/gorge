package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestXSweepCensusPricesTheReflectedDualExtraAsOnePick: a reflected
// StatTapExtra (StTapReflect, Mana Flare's body) over a fixed several-colour
// tap (a {R} or {G} dual) is one more mana OF THE PICK, so one tap makes
// manaN+r mana of one colour, never a mix (CR 605.3b; kgen's StTapReflect doc,
// "the planner's n+1 mana of one colour"; gorge pay planalts.go addTapExtra
// adds the extra per declared production). The census's record must price the
// grown tap the same way: a potUnit record of the union mask with the grown
// amount is n independent units of any of mask's colours, so the X-bound
// sweep (xBoundPayable) read {R}{G} payable off one dual tap and offered an
// X-bound cast (or an X kicker, kw_multikicker.go) gorge's payment planner
// prices insufficient -- a divergence opposite to the one the per-tap census
// was added to fix (the recorded Titan's Revenge repro).
func TestXSweepCensusPricesTheReflectedDualExtraAsOnePick(t *testing.T) {
	sc, sl := tapReflectScen(t, func(a *kop.Ability) { a.ManaColors = kop.ColR | kop.ColG })
	pc := sc.s.manaPCGen(sc.k, sl)
	if m := srcMask(pc); m != uint8(kop.ColR|kop.ColG) {
		t.Fatalf("precondition: the source's mask %#x is not the {R} or {G} dual", m)
	}
	if pc.manaXv != [kop.NumManaKinds]uint8{} || pc.manaChosen || pc.painCol != 0 {
		t.Fatal("precondition: the source is not one plain fixed-choice tap")
	}
	if r := sc.s.srcReflectPick(sc.k, sl, pc); r != 1 {
		t.Fatalf("precondition: srcReflectPick = %d, want the carrier's 1", r)
	}
	// the sweep's spell-payment descriptor (filters_gen.go xBoundPayable)
	d := payDesc{class: pdSpell, card: sc.s.cardAt(sl), src: NoSlot, og: restrOrigin{from: kop.ZoneHand}}
	var pot avail
	var pu [maxPotUnits]potUnit
	sc.s.availPotentialFor(sc.k, 0, &pot, &d, true, &pu)
	if pu[0].n == 0 && pu[0].mask != 0 {
		t.Fatal("the census record is incomplete (the overflow marker): the sweep keeps the over-bound, which affords the mix anyway")
	}
	tap := false
	for i := 0; i < maxPotUnits && pu[i].mask != 0; i++ {
		if pu[i].mask == uint8(kop.ColR|kop.ColG) && pu[i].n == 2 {
			tap = true
		}
	}
	if !tap {
		t.Fatalf("the census does not hold the dual's grown tap ({R}{G}:2) at all: %v", pu[:2])
	}
	var ex avail
	if !potUnitsAvail(&ex, &pu, int(sc.s.pl[0].life)) {
		t.Fatal("potUnitsAvail reports no exact view: the sweep keeps the over-bound, which affords the mix anyway")
	}
	rg := cost(0, kop.R, kop.G)
	rr := cost(0, kop.R, kop.R)
	gg := cost(0, kop.G, kop.G)
	if ex.affords(&rg, pipMaskOf(&rg), 0) {
		t.Error("the census affords {R}{G} off one dual tap's grown unit: one tap makes mana of the pick alone (CR 605.3b)")
	}
	if !ex.affords(&rr, pipMaskOf(&rr), 0) {
		t.Error("the census refuses {R}{R}: the grown tap must afford manaN+r mana of one colour")
	}
	if !ex.affords(&gg, pipMaskOf(&gg), 0) {
		t.Error("the census refuses {G}{G}: the grown tap must afford manaN+r mana of one colour")
	}
}

// Two reflected duals price independently: each tap picks its own colour, so
// {R}{R}{G}{G} reads payable off two grown taps and a third pip of one colour
// does not (two taps make at most {R:2}+{R:2} or {R:2}+{G:2}).
func TestXSweepCensusPricesTwoReflectedDualsIndependently(t *testing.T) {
	sc, sl := tapReflectScen(t, func(a *kop.Ability) { a.ManaColors = kop.ColR | kop.ColG })
	if r := sc.s.srcReflectPick(sc.k, sl, sc.s.manaPCGen(sc.k, sl)); r != 1 {
		t.Fatalf("precondition: srcReflectPick = %d, want the carrier's 1", r)
	}
	second := sc.perm(sc.s.cardAt(sl), 0)
	if pc := sc.s.manaPCGen(sc.k, sc.slot(second)); sc.s.srcReflectPick(sc.k, sc.slot(second), pc) != 1 {
		t.Fatal("precondition: the second dual carries no reflected extra")
	}
	d := payDesc{class: pdSpell, card: sc.s.cardAt(sl), src: NoSlot, og: restrOrigin{from: kop.ZoneHand}}
	var pot avail
	var pu [maxPotUnits]potUnit
	sc.s.availPotentialFor(sc.k, 0, &pot, &d, true, &pu)
	if pu[0].n == 0 && pu[0].mask != 0 {
		t.Fatal("the census record is incomplete (the overflow marker)")
	}
	var ex avail
	if !potUnitsAvail(&ex, &pu, int(sc.s.pl[0].life)) {
		t.Fatal("potUnitsAvail reports no exact view over two grown taps")
	}
	rrgg := cost(0, kop.R, kop.R, kop.G, kop.G)
	rrr := cost(0, kop.R, kop.R, kop.R, kop.G)
	if !ex.affords(&rrgg, pipMaskOf(&rrgg), 0) {
		t.Error("the census refuses {R}{R}{G}{G} off two grown dual taps: one tap picks {R:2}, the other {G:2}")
	}
	if ex.affords(&rrr, pipMaskOf(&rrr), 0) {
		t.Error("the census affords {R}{R}{R}{G} off two grown dual taps: neither tap makes three mana of the pick")
	}
}
