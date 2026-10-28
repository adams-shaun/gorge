package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestTargetShapeChainAnyAndUnique pins the whole-chain reading of the
// cast/activation target-shape params (cr-g287), on the real corpus carriers
// and the chain shape the engine records (root Targets + SubAbility$
// SubTargets):
//
//	(a) TargetsValid$ is ANY-match: a Heroic creature's trigger fires when a
//	    spell targets it AND another creature. The old all-match reading kept
//	    it silent.
//	(b) IsSingleTarget$ counts DISTINCT targets across the whole chain: a
//	    second, different creature on the SubAbility$ link makes the cast not
//	    single, so Leyline of Resonance stays silent. The old head-only
//	    len(Targets)==1 reading fired it.
//	(c) Two target slots naming the SAME creature are still one target (the
//	    Ivy, Gleeful Spellthief ruling, 2022-09-09): Leyline fires.
func TestTargetShapeChainAnyAndUnique(t *testing.T) {
	t.Parallel()

	// (a) Gnarlback Rhino's "Whenever you cast a spell that targets
	// CARDNAME" against a two-target spell.
	t.Run("any-match heroic fires with a second target", func(t *testing.T) {
		rhino := corpusAlternativeCard(t, "Gnarlback Rhino")
		two := card(t, "Name:ChainProbe\nManaCost:1 R\nTypes:Instant\n"+
			"A:SP$ Pump | ValidTgts$ Creature | TargetMax$ 2 | NumAtt$ 1\nOracle:x\n")
		e, rhinoID := targetsValidEngine(t, rhino, two)
		other := putToken(t, e, 1, "Name:Other Bear\nTypes:Creature\nPT:2/2", state.ZBattlefield)
		castChainProbe(t, e, "ChainProbe", rhinoID, other)
		if n := queuedTriggersFrom(e, rhinoID); n != 1 {
			t.Fatalf("Rhino queued %d triggers for a spell targeting it and another creature, want 1", n)
		}
		drainTargetsValid(t, e)
	})

	// The chain probe: the root targets a creature you control, the
	// SubAbility$ link asks for a second target of the same kind.
	subProbe := "Name:ChainSubProbe\nManaCost:1 R\nTypes:Instant\n" +
		"A:SP$ Pump | ValidTgts$ Creature.YouCtrl | NumAtt$ 1 | SubAbility$ DBSub\n" +
		"SVar:DBSub:DB$ Pump | ValidTgts$ Creature.YouCtrl | NumAtt$ 1\n" +
		"Oracle:x\n"

	// (b) A distinct second target on the sub-ability link.
	t.Run("second distinct sub target is not single", func(t *testing.T) {
		leyline := corpusAlternativeCard(t, "Leyline of Resonance")
		e, leylineID := targetsValidEngine(t, leyline, card(t, subProbe))
		first := putToken(t, e, 0, "Name:First Bear\nTypes:Creature\nPT:2/2", state.ZBattlefield)
		second := putToken(t, e, 0, "Name:Second Bear\nTypes:Creature\nPT:2/2", state.ZBattlefield)
		castChainProbe(t, e, "ChainSubProbe", first)
		answerCastSubObj(t, e, second)
		if n := queuedTriggersFrom(e, leylineID); n != 0 {
			t.Fatalf("Leyline queued %d triggers for a two-distinct-target chain, want 0", n)
		}
	})

	// (c) Both target slots name the same creature: one distinct target.
	t.Run("same creature twice is still single", func(t *testing.T) {
		leyline := corpusAlternativeCard(t, "Leyline of Resonance")
		e, leylineID := targetsValidEngine(t, leyline, card(t, subProbe))
		bear := putToken(t, e, 0, "Name:Same Bear\nTypes:Creature\nPT:2/2", state.ZBattlefield)
		castChainProbe(t, e, "ChainSubProbe", bear)
		answerCastSubObj(t, e, bear)
		if n := queuedTriggersFrom(e, leylineID); n != 1 {
			t.Fatalf("Leyline queued %d triggers for two slots on one creature, want 1", n)
		}
	})
}

// castChainProbe casts the named card from seat 0's hand and answers its
// FIRST target ask with every object in wants, in order (a multi-target
// TargetMax$ ask takes all picks in one intent).
func castChainProbe(t *testing.T, e *Engine, name string, wants ...state.ObjID) {
	t.Helper()
	id := state.ObjID(0)
	for _, hid := range e.G.Zone(state.ZHand, 0) {
		if o := e.G.Obj(hid); o != nil && o.Face() != nil && o.Face().Name == name {
			id = hid
		}
	}
	if id == 0 {
		t.Fatalf("%s not in hand", name)
	}
	e.beginCast(0, decision.Option{Kind: "cast", Obj: id})
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("the probe cast did not ask for targets: %+v", d)
	}
	choices := make([]int, 0, len(wants))
	for _, want := range wants {
		idx := -1
		for _, o := range d.Options {
			if o.Kind != "player" && o.Obj == want {
				idx = o.Index
				break
			}
		}
		if idx < 0 {
			t.Fatalf("no target option for %d: %+v", want, d.Options)
		}
		choices = append(choices, idx)
	}
	submitChoices(t, e, choices...)
}
