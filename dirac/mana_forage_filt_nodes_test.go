package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestForageFilterStepChargesOneNodePerAsk: the filter search takes a forage
// source as a step (comboAnyOther -> forageOther) and charges the activation's
// first ask once, as the Combo Any search does (forageWalk): the walk's nodes
// are the whole count, no plain colour-ask node before it.
func TestForageFilterStepChargesOneNodePerAsk(t *testing.T) {
	sc, id := giftScen(t, forageManaName, "Swamp")
	k := sc.k
	if !k.forageCA(id[forageManaName]) {
		t.Fatalf("%s is not a forage Combo Any source", forageManaName)
	}
	sc.grave(id["Swamp"], 0, 3) // three cards: the exile arm is payable
	o := sc.perm(id[forageManaName], 0)
	sl := sc.slot(o)
	if !sc.s.foragePayable(k, 0, sl) {
		t.Fatal("the forage is not payable: the walk has no asks to count")
	}
	want := 0
	forageWalk(k, sc.s, 0, sl, &[kop.NumManaKinds]bool{}, func() bool { want++; return true },
		func(Slot, int, int) bool { return false }, func(int) bool { return false })
	if want < 2 {
		t.Fatalf("the walk ticks %d times: too few asks to tell the orders apart", want)
	}
	fs := &filtSearch{p: 0}
	if fs.comboAnyOther(sc.s, k, sl, func(otherStep) bool { return false }) {
		t.Fatal("a refusing try found a step")
	}
	if fs.nodes != want {
		t.Errorf("the filter search charged %d nodes for the forage source, the walk's asks are %d", fs.nodes, want)
	}
}
