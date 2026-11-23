package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A permanent taps once, so its printed face and a granted one are alternatives (CR 605.1a).
// Palladium Myr's {C}{C} beside Cryptolith Rite's one mana of any colour makes {C}{C} or one
// coloured mana, never two: with two Forests the board holds one white source, and Dusk's
// {2}{W}{W} is out of reach while {3}{G} is not (dr tail-15 pair 4, decision 323).
func TestGrantFaceLargerPrintedIsAnAlternative(t *testing.T) {
	var s State
	own := grantUnit{mask: 1 << kop.C, n: 2}
	grant := grantUnit{mask: uint8(kop.ColAny), n: 1}
	u := mergeGrantFace(own, grant)
	if u.n != 2 || u.altN != 2 || u.colN != 1 {
		t.Fatalf("precondition: merged unit %+v, want the {C}{C} face as the alternative of a one-mana colour block", u)
	}
	var a avail
	a.addUnits(1<<kop.G, 2)
	s.addGrantUnit(&a, u)
	dusk := kop.ManaCost{Generic: 2}
	dusk.Pips[kop.W] = 2
	if a.affords(&dusk, pipMaskOf(&dusk), 0) {
		t.Error("two Forests and a Myr under Rite pay {2}{W}{W}: the Myr's grant makes one white mana")
	}
	wOnly := kop.ManaCost{Generic: 3}
	wOnly.Pips[kop.G] = 1
	if !a.affords(&wOnly, pipMaskOf(&wOnly), 0) {
		t.Error("{3}{G} unpayable: the Myr's {C}{C} and two Forests make four mana")
	}
	one := kop.ManaCost{Generic: 2}
	one.Pips[kop.W] = 1
	if !a.affords(&one, pipMaskOf(&one), 0) {
		t.Error("{2}{W} unpayable: a Forest, a Forest and the Myr's one white mana")
	}
}
