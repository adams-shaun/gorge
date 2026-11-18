package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestXSweepCensusPricesTheReflectedAnyColourExtraAsOnePick: a reflected
// StatTapExtra (StTapReflect, Roxanne's body) over a fixed any-colour tap
// (Birds of Paradise) is one more mana OF THE PICK, so one tap makes
// manaN+r mana of the one colour it picks (CR 605.3b; kgen's StTapReflect
// doc). The X-bound sweep's census (and the multikicker census, the unbounded
// mode) must price the grown tap as a one-colour source: recording it as
// {ColAny:2} independent units read {R}{G} payable off one tap, an X-bound
// cast or kicker gorge's payment planner prices insufficient.
func TestXSweepCensusPricesTheReflectedAnyColourExtraAsOnePick(t *testing.T) {
	sc, sl := tapReflectScen(t, nil)
	pc := sc.s.manaPCGen(sc.k, sl)
	if srcMask(pc) != uint8(kop.ColAny) || pc.manaCol != kop.ColAny || pc.manaChosen || pc.manaMulti {
		t.Fatal("precondition: the source is not one plain fixed any-colour tap")
	}
	if r := sc.s.srcReflectPick(sc.k, sl, pc); r != 1 {
		t.Fatalf("precondition: srcReflectPick = %d, want the carrier's 1", r)
	}
	// the sweep's spell-payment descriptor (filters_gen.go xBoundPayable)
	d := payDesc{class: pdSpell, card: sc.s.cardAt(sl), src: NoSlot, og: restrOrigin{from: kop.ZoneHand}}
	rg := cost(0, kop.R, kop.G)
	rr := cost(0, kop.R, kop.R)
	for _, bounded := range []bool{true, false} {
		var pot avail
		var pu [maxPotUnits]potUnit
		sc.s.availPotentialFor(sc.k, 0, &pot, &d, bounded, &pu)
		if pu[0].n == 0 && pu[0].mask != 0 {
			t.Fatalf("boundedAny=%v: the census record is incomplete (the overflow marker): the sweep keeps the over-bound, which affords the mix anyway", bounded)
		}
		tap := false
		for i := 0; i < maxPotUnits && pu[i].mask != 0; i++ {
			if pu[i].mask == uint8(kop.ColAny) && pu[i].n == 2 {
				tap = true
			}
		}
		if !tap {
			t.Fatalf("boundedAny=%v: the census does not hold the grown tap (ColAny:2) at all: %v", bounded, pu[:2])
		}
		var ex avail
		if !potUnitsAvail(&ex, &pu, int(sc.s.pl[0].life)) {
			t.Fatalf("boundedAny=%v: potUnitsAvail reports no exact view: the sweep keeps the over-bound, which affords the mix anyway", bounded)
		}
		if ex.affords(&rg, pipMaskOf(&rg), 0) {
			t.Errorf("boundedAny=%v: the census affords {R}{G} off one any-colour tap's grown unit: one tap makes mana of the pick alone (CR 605.3b)", bounded)
		}
		if !ex.affords(&rr, pipMaskOf(&rr), 0) {
			t.Errorf("boundedAny=%v: the census refuses {R}{R}: the grown tap must afford manaN+r mana of one colour", bounded)
		}
	}
}
