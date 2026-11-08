package dirac

import (
	"testing"

	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestManaStepPlanCutStaysCut: a payment plan that stops at a decision (a
// Treasure sacrificed for mana parked behind a replacement ask) hands the
// rest of the cast to the manual window, and that handover lasts for the
// whole build. Before the fix manaStep cleared bflPlanCut on its first read,
// so the next re-entry — after the window's own parked sacrifice was
// answered — planned again; a fresh plan names an any-colour source's colour
// by its own rule (G first) where gorge's window asks the lowest, so one
// Treasure floated a different colour on each engine.
func TestManaStepPlanCutStaysCut(t *testing.T) {
	sc := newScen(t, Config{})
	bears := sc.hand(v.GrizzlyBears, 0)
	forest := sc.perm(v.Forest, 0)
	sc.kcastBuild(bears, v.GrizzlyBears, 0)
	sc.s.bld().bfl |= bflPlanCut

	sc.s.manaStep(sc.k)

	if sc.s.bld().bfl&bflPlanCut == 0 {
		t.Fatal("manaStep cleared the plan cut: the build's next re-entry would plan again instead of staying in the manual window")
	}
	if sc.s.perms[sc.slot(forest)].flags&pfTapped == 0 {
		t.Fatal("precondition: the manual window did not tap the Forest, so the plan-cut arm did not run")
	}
}
