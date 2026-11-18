package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPainOnlyCensusPlainCastFloats: the plain-cast half of the pain census's
// lethal guard, at painOnlyScen's pose (six untapped Cabal Pits, {C}1 {B}1
// floated, life 1, Caligo Skin-Witch — printed {1}{B}, kicker {3}{B} — in
// hand). The offer gate's census (availForMode) routes every pain source
// through painAvailMaskAt, whose per-activation guard (painN < life) withholds
// every pit's {B} at 1 life, and painTight (the summed guard) bails there
// because the per-activation guard already emptied the census — so the census
// holds the floats alone. gorge's side is measured beside the planner
// (engine/rules/payment_plan_paincast_test.go): the floats alone pay {1}{B},
// so gorge prices and offers the same cast — the parent ticket's "plain cast
// is the remaining offer" was this agreement, not a divergence. The short
// floats and the life-2 arms below pin the guard on the plain route: with the
// {B} float missing at 1 life the cast is withheld (every {B} unit would be a
// refused activation), and at 2 life exactly one activation fits the summed
// guard (gorge prices one) and the cast is offered.
func TestPainOnlyCensusPlainCastFloats(t *testing.T) {
	sc, caligo := painOnlyScen(t, 1)
	var av avail
	sc.s.availFor(sc.k, 0, &av)
	if av.n != 2 || av.mask[0] != 1<<kop.B || av.mask[1] != 1<<kop.C || av.cnt[0] != 1 || av.cnt[1] != 1 {
		t.Fatalf("precondition: the census is not the floats alone: n=%d units=%v/%v", av.n, av.mask[:3], av.cnt[:3])
	}
	if !castModeOffered(sc, caligo, 0) {
		t.Fatal("the plain cast is not offered at the pose (the floats alone pay {1}{B})")
	}
	// the {B} float missing at 1 life: no {B} unit but a refused activation's
	sc2, caligo2 := painOnlyScen(t, 1)
	sc2.s.pl[0].pool[kop.B] = 0
	if sc2.s.pl[0].pool[kop.B] != 0 {
		t.Fatal("precondition: the {B} float did not drop")
	}
	if castModeOffered(sc2, caligo2, 0) {
		t.Fatal("the plain cast is offered at 1 life with every {B} unit a refused activation")
	}
	// 2 life: one activation fits the summed guard (pain 1 < 2) and pays the pip
	sc3, caligo3 := painOnlyScen(t, 2)
	sc3.s.pl[0].pool[kop.B] = 0
	if !castModeOffered(sc3, caligo3, 0) {
		t.Fatal("the plain cast is not offered at 2 life (one pit activation, pain 1 < 2, pays the pip)")
	}
}
