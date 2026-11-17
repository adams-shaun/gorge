package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestUnchosenChosenSourcePricesOpenAtAPayment: a "Add one mana of the chosen
// color" source whose colour is not chosen yet. gorge's payment bound folds it
// open in every colour (potential.go: addPotentialChosenColour declines a
// source with no chosen colour, so producedOpen("Chosen") prices it
// unbounded), where the kernel's mask read none. With a colour chosen the
// bound is that one colour, as before.
func TestUnchosenChosenSourcePricesOpenAtAPayment(t *testing.T) {
	tbl, id := tapsDeferShapeTable(t, func(tbl *kop.Table, id map[string]CardID) {
		rewriteMana(tbl, id["Firdoch Core"], func(a *kop.Ability) {
			a.ManaColors = 0
			a.AFlags |= kop.AfManaChosen
		})
	})
	measure := func(chosen uint8) (open bool, av avail, sc *scen, sl Slot) {
		sc = newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
		core := sc.perm(id["Firdoch Core"], 0)
		sl = sc.slot(core)
		sc.s.perms[sl].chosen = objWord(chosen)
		if pc := sc.s.manaPCGen(sc.k, sl); !pc.manaChosen || pc.manaAb < 0 {
			t.Fatalf("precondition: the Core's mana facts are chosen %v ability %d, want a live chosen-colour source", pc.manaChosen, pc.manaAb)
		}
		other := payDesc{class: pdOther, subBound: true}
		open = sc.s.availPotentialFor(sc.k, 0, &av, &other, false)
		return
	}
	if open, _, _, _ := measure(255); !open {
		t.Error("no colour chosen: the payment bound must price the source open, as gorge's fold does")
	}
	open, av, _, _ := measure(kop.R)
	if open {
		t.Error("red chosen: the bound is one {R}, not open")
	}
	if r := cost(0, kop.R); !av.affords(&r, pipMaskOf(&r), 0) {
		t.Error("red chosen: the bound does not reach {R}")
	}
}
