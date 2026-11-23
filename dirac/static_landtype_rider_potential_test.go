package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRiderGrantIntrinsicPotential: a rider-first source under a granted
// basic land type is the intrinsic unit in the planner census, but gorge's
// potentialManaFor still folds the card's own rider beside it: the
// membership walk lists every printed mana ability (the mining-counter cost
// is payable while the counter is there) and one intrinsic per granted type
// the printed abilities do not already produce exactly (CR 305.6, 605.3b).
// Gemstone Mine under an Island grant is {U} fixed and the rider's
// any-colour production unbounded, so the potential bound is open.
func TestRiderGrantIntrinsicPotential(t *testing.T) {
	sc, _, objs := blgRiderBoardSubs(t, Config{}, []string{"Island"}, "Gemstone Mine")
	o := objs["Gemstone Mine"]
	sl := sc.slot(o)
	a, ok := sc.s.riderAbilityOf(sc.k, sl)
	if !ok {
		t.Fatal("precondition: Gemstone Mine is not the bounded-rider shape")
	}
	if a.ManaRider != kop.ManaRiderDeplete {
		t.Fatalf("precondition: Gemstone Mine is not the depletion rider (rider %v)", a.ManaRider)
	}
	blgRiderReady(sc, sl, int(a.ManaRiderCtr), 3)
	if pc := sc.s.manaPCGen(sc.k, sl); !pc.manaIntrinsic {
		t.Fatal("precondition: manaPCGen does not return the intrinsic view")
	}
	var av avail
	other := payDesc{class: pdOther}
	if open := sc.s.availPotentialFor(sc.k, 0, &av, &other, false); !open {
		t.Fatalf("potential open=false, want true: gorge's potentialManaFor folds the rider's any-colour production beside the intrinsic {U}")
	}
	c := cost(1, kop.W)
	if !av.affords(&c, pipMaskOf(&c), 0) {
		t.Error("the over-bound does not reach {1}{W}: the rider's unbounded fold is missing")
	}
	if n := sc.s.perms[sl].ctrN(sc.s, int(a.ManaRiderCtr)); n != 3 {
		t.Fatalf("mining counters %d, want 3: the potential walk changes no state", n)
	}
	if sc.tapped(o) {
		t.Fatal("Gemstone Mine tapped: the potential walk taps nothing")
	}
}

// TestRiderGrantIntrinsicPotentialMask: a multi-type grant's intrinsic view
// is one unit per granted colour (CR 305.6). The potential walk's first
// fold is the view's colours (potentialFirstMask), not the card's printed
// rider's: Gemstone Mine under Island and Swamp folds {U}{B}, not the
// rider's any-colour mask the fixpoint admits on its own.
func TestRiderGrantIntrinsicPotentialMask(t *testing.T) {
	sc, _, objs := blgRiderBoardSubs(t, Config{}, []string{"Island", "Swamp"}, "Gemstone Mine")
	sl := sc.slot(objs["Gemstone Mine"])
	a, ok := sc.s.riderAbilityOf(sc.k, sl)
	if !ok {
		t.Fatal("precondition: Gemstone Mine is not the bounded-rider shape")
	}
	blgRiderReady(sc, sl, int(a.ManaRiderCtr), 3)
	pc := sc.s.manaPCGen(sc.k, sl)
	if !pc.manaIntrinsic || !pc.manaMulti {
		t.Fatalf("precondition: no multi-colour intrinsic view (intrinsic %v multi %v)", pc.manaIntrinsic, pc.manaMulti)
	}
	want := uint8(kop.ColU | kop.ColB)
	if got := uint8(sc.s.potentialFirstMask(sc.k, 0, sl, pc, nil)) & 0x3f; got != want {
		t.Fatalf("potentialFirstMask %d, want the two granted colours %d: the fold prices the view, not the rider's any-colour", got, want)
	}
}

// TestRiderGrantIntrinsicWitness: the {X} witness's census is gorge's planner
// census (potentialModeCastPlan): it lists the intrinsic unit of a
// rider-deferred source under a granted basic land type like any printed
// source's (CR 305.6), the deferred rider beside it as the relaxed
// alternative it is. The witness reaches {1}{U} over the mine alone.
func TestRiderGrantIntrinsicWitness(t *testing.T) {
	sc, _, objs := blgRiderBoardSubs(t, Config{}, []string{"Island"}, "Gemstone Mine")
	sl := sc.slot(objs["Gemstone Mine"])
	a, ok := sc.s.riderAbilityOf(sc.k, sl)
	if !ok {
		t.Fatal("precondition: Gemstone Mine is not the bounded-rider shape")
	}
	blgRiderReady(sc, sl, int(a.ManaRiderCtr), 3)
	var av avail
	sc.s.availUngranted(sc.k, 0, &av)
	c := cost(1, kop.U)
	if !av.affords(&c, pipMaskOf(&c), 0) {
		t.Fatalf("the X witness does not count the intrinsic {U}: gorge's census lists it (CR 305.6)")
	}
}
