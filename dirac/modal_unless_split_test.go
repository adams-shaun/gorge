package dirac

// Gorge's askTriggerModes keeps a marked GenericChoice trigger on the stack
// unasked when only the UnlessCost$ gate empties the eligible list, and removes
// it (CR 603.3c) when the target / restriction half empties it. cast.go's
// bsModes arm splits the two with modeFeasibleTargets / charmModesHaveTargets.

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

func TestMarkedTriggerKeepsOrRemovesBySplitGate(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	a := kop.Ability{Kind: kop.AbTriggered, NModes: 2, ModesPick: 1, AFlags: kop.AfCharmTrigger, NTargets: 2}
	// mode 0: a target slot with a candidate (any target: a player always
	// is), and an UnlessCost$ the empty pool cannot pay. mode 1: a mandatory
	// object target on an empty battlefield, and no unless gate.
	a.Targets[0] = kop.TargetSpec{Kind: kop.TgtAny}
	a.Targets[1] = kop.TargetSpec{Kind: kop.TgtObject}
	a.TgtModes[0], a.TgtModes[1] = 1<<0, 1<<1
	a.Prog = []kop.Instr{
		{Op: kop.OpCharm, C: 0, A: kop.Const(2)},
		{Op: kop.OpUnlessSwitched, Obj: kop.RefController, A: kop.Const(2), B: kop.Const(1), C: int16(kop.PayMana)},
		{Op: kop.OpCharm, C: 1, A: kop.Const(0)},
	}
	b := sc.s.bld()
	b.e = stackEntry{kind: seTrigger, ctrl: 0}

	// PRECONDITIONS: the mode-1 slot really has no candidate, the pool is
	// empty, and nothing is chosen yet.
	if sc.s.modeFeasibleTargets(sc.k, &a, 1) {
		t.Fatal("precondition: mode 1's object target has a candidate on an empty battlefield")
	}
	if sc.s.pl[0].pool != [kop.NumManaKinds]uint8{} || b.e.mode != 0 {
		t.Fatalf("precondition: pool %v mode %d", sc.s.pl[0].pool, b.e.mode)
	}

	// Mode 0 is dropped by the unless gate alone: the offer is empty, yet the
	// target half passes, so the trigger is KEPT (gorge: placed unasked).
	if sc.s.modeFeasible(sc.k, &a, 0) {
		t.Fatal("mode 0 offered with its UnlessCost$ unpayable")
	}
	if !sc.s.modeFeasibleTargets(sc.k, &a, 0) || !sc.s.charmModesHaveTargets(sc.k, &a) {
		t.Fatal("the unless gate alone must not read as 'no mode has targets' (the keep arm)")
	}

	// With mode 0 already chosen the only mode left has no target: the
	// eligible list is empty from targets, the REMOVAL arm (CR 603.3c).
	b.e.mode = 1 << 0
	if sc.s.charmModesHaveTargets(sc.k, &a) {
		t.Fatal("a mode with no target candidate counted as eligible (the removal arm)")
	}
}
