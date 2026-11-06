package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// The potential walk's compose plan (cost_compose_plan.go): the first
// candidate assignment's compose records the target-independent head's
// verdict per static, and the later assignments of the same call run only
// the target-dependent tail. The fixture is the two-bear compose: both
// reducers are Card.Self statics of OTHER objects, so the head denies each
// for the priced spell, and both assignments' composes answer from the plan.
func planFixture(t *testing.T) (*Engine, state.ObjID, state.ObjID, state.ObjID, Cost, costStaticViews, []state.Target) {
	t.Helper()
	spellCard := card(t, "Name:Test Bolt\nManaCost:2\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Creature | NumDmg$ 1\nOracle:x\n")
	e := handEngine(t, spellCard)
	source := func(name string) state.ObjID {
		return battlePerm(t, e, 0, "Name:"+name+"\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nS:Mode$ ReduceCost | ValidTarget$ Card.Self | Activator$ You | Type$ Spell | Amount$ 1 | Description$ Discount a spell targeting CARDNAME.\nOracle:x\n")
	}
	a, b := source("First Bear"), source("Second Bear")
	// A third bear carries its own Card.Self spell discount (the corpus's
	// self-cost-reduction shape): for the priced spell -- any other object --
	// the head's ValidCard$ branch denies it before any target is read.
	selfSource := battlePerm(t, e, 0, "Name:Third Bear\nManaCost:1 G\nTypes:Enchantment\nS:Mode$ ReduceCost | ValidCard$ Card.Self | Activator$ You | Type$ Spell | Amount$ 1 | Description$ Discount its own cast.\nOracle:x\n")
	if selfSource == 0 || e.G.Obj(selfSource).Zone != state.ZBattlefield {
		t.Fatalf("precondition: the self-discount bear never landed")
	}
	spell := e.G.Zone(state.ZHand, 0)[0]
	base := e.parseCost("2")
	statics := e.collectCostStatics()
	if len(statics.reduce) != 3 {
		t.Fatalf("precondition: expected all three reducers, got %d", len(statics.reduce))
	}
	cands := e.costPotentialTargets(0, spell, spellScope(""))
	if len(cands) != 2 || cands[0].Obj != a || cands[1].Obj != b {
		t.Fatalf("precondition: candidates=%+v", cands)
	}
	return e, spell, a, b, base, statics, cands
}

func TestCostComposePlanRecordsTheHead(t *testing.T) {
	t.Parallel()
	e, spell, a, b, base, statics, cands := planFixture(t)
	var plan *costComposePlan
	_, ok := e.potentialCostModsUsing(statics, 0, spell, spellScope(""), cands, 0, func(m costMods) bool {
		if e.costComposePlan == nil {
			t.Fatal("the potential walk took no compose plan")
		}
		plan = e.costComposePlan
		return false // enumerate every assignment
	})
	if ok || plan == nil {
		t.Fatalf("precondition: accepted=%v plan=%+v", ok, plan)
	}
	if plan.collect || !plan.serve {
		t.Fatalf("the plan never flipped to serve (collect %v serve %v)", plan.collect, plan.serve)
	}
	// The walk visits all three reducers on every assignment (raise, reduce,
	// set), in collection order: the two target-conditional reducers pass the
	// head (their ValidTarget$ gate is the tail's to read), and the third
	// bear's own Card.Self discount is denied by the head's ValidCard$
	// branch for the priced spell, target-independently.
	if len(plan.skip) != 3 {
		t.Fatalf("expected the head's verdict for all three reducers, got %v", plan.skip)
	}
	if plan.skip[0] || plan.skip[1] {
		t.Fatalf("the head must pass the target-conditional reducers, got %v", plan.skip)
	}
	if !plan.skip[2] {
		t.Fatalf("the head should deny the other-object Card.Self static, got %v", plan.skip)
	}
	// The plan records answers, not a different composition: each candidate's
	// question, re-asked plan-free, still earns exactly its own bear's
	// reduction.
	for _, c := range []state.ObjID{a, b} {
		again := e.costModifiersWithTargetsUsing(statics, 0, spell, spellScope(""), []state.Target{{Obj: c}}, true)
		if len(again.Reduces) != 1 || again.Apply(base).Generic != 1 {
			t.Fatalf("the planned walk's answers drifted for target %d: reductions=%+v", c, again.Reduces)
		}
	}
}

// A corrupted recorded verdict must be caught: verify mode recomputes every
// planned gate through the full chain and panics on a disagreement.
func TestCostComposePlanVerifyCatchesCorruption(t *testing.T) {
	t.Parallel()
	e, spell, _, _, _, statics, cands := planFixture(t)
	var plan *costComposePlan
	corrupted := false
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatal("verify mode served a corrupted plan verdict instead of panicking")
			}
			msg, ok := r.(string)
			if !ok || !strings.Contains(msg, "potential-walk compose plan is stale") {
				t.Fatalf("unexpected panic %v", r)
			}
		}()
		e.potentialCostModsUsing(statics, 0, spell, spellScope(""), cands, 0, func(m costMods) bool {
			if plan == nil && e.costComposePlan != nil && e.costComposePlan.serve {
				plan = e.costComposePlan
			}
			// Corrupt the first assignment's recorded verdict once the plan
			// has collected: claim the head DENIES the first reducer (a
			// target-conditional one it passed). The second assignment's
			// planned gate must disagree with the full chain and panic.
			if plan != nil && plan.serve && len(plan.skip) >= 1 && !corrupted {
				plan.skip[0] = true
				corrupted = true
			}
			return false
		})
	}()
	if plan == nil {
		t.Fatal("precondition: the walk never took a serving plan")
	}
	if !corrupted {
		t.Fatal("precondition: the recorded verdict was never corrupted")
	}
}

// The split is exact: prefix and tail compose to the full chain for every
// static of the collection, and a head denial is always a target-independent
// one.
func TestCostComposePlanSplitMatchesTheFullChain(t *testing.T) {
	t.Parallel()
	e, spell, a, _, _, statics, _ := planFixture(t)
	targets := []state.Target{{Obj: a}}
	for _, group := range []struct {
		mode  string
		views []staticView
	}{
		{"RaiseCost", statics.raise},
		{"ReduceCost", statics.reduce},
		{"SetCost", statics.set},
	} {
		for _, sv := range group.views {
			prefix := e.costStaticGatePrefix(sv, group.mode, 0, spell, spellScope(""), targets, false)
			full, indep := e.costStaticGate(sv, group.mode, 0, spell, spellScope(""), targets, false)
			if !prefix && (full || !indep) {
				t.Fatalf("a head denial must be target-independent: static %d/%s full (%v,%v)",
					sv.Source, group.mode, full, indep)
			}
			if !prefix {
				continue
			}
			tailOK, tailIndep := e.costStaticGateTail(sv, group.mode, 0, spell, spellScope(""), targets, false, nil, 0)
			if tailOK != full || tailIndep != indep {
				t.Fatalf("head/tail split diverged for static %d/%s: full (%v,%v), tail (%v,%v)",
					sv.Source, group.mode, full, indep, tailOK, tailIndep)
			}
		}
	}
}
