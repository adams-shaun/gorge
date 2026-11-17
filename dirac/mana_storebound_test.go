package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestStoreBoundOnlyAtTheTriggerPayGate: a storage battery reads at its
// counters only for the trigger pay gate (payDesc.subBound, gorge's
// resPayNeedsMana: Descriptor.SubCounterBound). Every other pdOther payment (the
// numTimes and X asks, the window's potential gate) prices it unbounded, as
// gorge's potentialManaFor does without the flag: a 0-counter battery still
// funds the ask's count there, and only the pay gate that follows refuses it.
func TestStoreBoundOnlyAtTheTriggerPayGate(t *testing.T) {
	p := storeProbeMatchup(t)
	sc := newScenTableLib(t, p.tbl, p.storage, Config{NoElide: true})
	o := sc.perm(p.storage, 0)
	sl := sc.slot(o)
	sc.s.perms[sl].flags &^= pfTapped // Bottomless Vault enters tapped
	cost := &kop.ManaCost{}
	cost.Pips[kop.B] = 2
	affords := func(d payDesc) bool {
		var pot avail
		sc.s.availPotentialFor(sc.k, 0, &pot, &d, false)
		return pot.affords(cost, pipMaskOf(cost), 0)
	}
	plain, gate := payDesc{class: pdOther, src: NoSlot}, payDesc{class: pdOther, src: NoSlot, subBound: true}
	if !affords(plain) {
		t.Fatalf("a 0-counter battery must stay unbounded for a pdOther payment without the gate flag")
	}
	if affords(gate) {
		t.Fatalf("the trigger pay gate must read a 0-counter battery as no mana")
	}
	sc.s.perms[sl].addCtrN(sc.s, int(kop.CtrStorage), 2)
	if !affords(gate) {
		t.Fatalf("the trigger pay gate must read a 2-counter battery as {B}{B}")
	}
	cost.Pips[kop.B] = 3
	if affords(gate) {
		t.Fatalf("the trigger pay gate must read a 2-counter battery as two mana, not three")
	}
	if !affords(plain) {
		t.Fatalf("the unflagged payment keeps the battery unbounded whatever its counters")
	}
}
