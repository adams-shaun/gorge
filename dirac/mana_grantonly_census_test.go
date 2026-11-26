package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestGrantOnlySourceCensusUnit: a creature whose only mana ability is
// Citanul Hierophants' granted "{T}: Add {G}" (Jaheira's, to a token, has the
// same shape; the tokmana pool holds Hierophants) is a census unit of gorge's payment
// census at its battlefield position (windowManaUnitsWith lists the fixed
// grant; pay.PlanAlts then drops the granted alternative, so the unit has no
// class). The planner keeps that position in phase 0 as an inert unit, so the
// Forest entered after the token is unit 2, gorge's census position, and the
// chosen plan never taps the token. A tapped token leaves gorge's walk
// without the unit, so the census is one shorter.
func TestGrantOnlySourceCensusUnit(t *testing.T) {
	// the Hierophants is a creature itself, so it is a granted-only source too
	plan := func(tapBears bool) (*planner, [4]Slot) {
		sc, _, objs := tokScen(t, Config{NoElide: true}, "Citanul Hierophants", "Forest", "Grizzly Bears", "Forest")
		var sl [4]Slot
		for i, o := range objs {
			sl[i] = sc.slot(o)
		}
		for _, i := range []int{0, 2} {
			if !sc.s.manaGranted(sc.k, sl[i]) || !sc.s.grantsFixedMana(sc.k, sl[i]) {
				t.Fatalf("precondition: board slot %d's only mana ability is not a granted fixed one", i)
			}
		}
		if tapBears {
			sc.s.perms[sl[2]].flags |= pfTapped
		}
		var c kop.ManaCost
		c.Pips[kop.G] = 2
		var pk planner
		sc.s.tapNeed(sc.k, &pk, 0, &c, 0, false)
		sc.s.plan(sc.k, 0, &pk, NoObj)
		return &pk, sl
	}

	pk, sl := plan(false)
	if !pk.found || pk.nu != 4 {
		t.Fatalf("plan: found %v units %d, want 4 (the two granted-only creatures' inert units and two Forests)", pk.found, pk.nu)
	}
	for u := 0; u < 4; u++ {
		if pk.units[u] != sl[u] {
			t.Fatalf("census units %v, want %v in battlefield order", pk.units[:pk.nu], sl)
		}
		if inert := pk.types[pk.utyp[u]].inert; inert != (u == 0 || u == 2) {
			t.Errorf("unit %d inert %v", u, inert)
		}
	}
	if pk.bestN != 2 {
		t.Fatalf("plan taps %d sources, want the two Forests", pk.bestN)
	}
	for i := 0; i < pk.bestN; i++ {
		if u := pk.bestSeq[i]; pk.types[pk.utyp[u]].inert {
			t.Errorf("the plan selects the inert unit (sequence %v)", pk.bestSeq[:pk.bestN])
		}
	}

	pk, _ = plan(true)
	if !pk.found || pk.nu != 3 {
		t.Errorf("tapped Bears: found %v units %d, want 3 (a tapped source leaves gorge's walk without the unit)", pk.found, pk.nu)
	}
}
